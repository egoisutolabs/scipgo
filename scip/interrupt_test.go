package scip

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// hardTestModel returns a model whose solve runs long enough to interrupt.
func hardTestModel(t *testing.T) Model {
	t.Helper()
	return mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
}

// A 50 ms deadline on a long solve must stop it promptly with
// StatusUserInterrupt, report the context error through errors.Is, and leave
// the model usable.
func TestSolveContextDeadlineInterrupts(t *testing.T) {
	model := hardTestModel(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	solved, err := model.SolveContext(ctx)
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("solve ran %v past the deadline", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SolveContext error = %v, want context.DeadlineExceeded", err)
	}
	var e *Error
	if !errors.As(err, &e) || e.Op != "SolveContext" {
		t.Fatalf("error is not an *Error of SolveContext: %v (%T)", err, err)
	}
	if s := solved.Status(); s != StatusUserInterrupt {
		t.Fatalf("status = %v, want UserInterrupt", s)
	}
	if sol, ok := solved.BestSol(); ok {
		sol.ObjVal() // the incumbent, when present, must be readable
	}

	// The model stays usable: free the transformed problem and solve again.
	// gen-ip054 cannot finish in 2 s, so the second deadline stops that solve
	// in turn; what is asserted is that it runs and reports cleanly.
	solved = solved.FreeTransform()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	resolved, err := solved.SolveContext(ctx2)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("re-solve error = %v, want context.DeadlineExceeded", err)
	}
	if s := resolved.Status(); s != StatusUserInterrupt {
		t.Fatalf("re-solve status = %v, want UserInterrupt", s)
	}
}

// An already-cancelled context must prevent SCIPsolve from being called.
func TestSolveContextCancelledBeforeStart(t *testing.T) {
	model := createTestModel(t)
	before := model.Stage()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	solved, err := model.SolveContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SolveContext error = %v, want context.Canceled", err)
	}
	var e *Error
	if !errors.As(err, &e) || e.Op != "SolveContext" {
		t.Fatalf("error is not an *Error of SolveContext: %v (%T)", err, err)
	}
	if solved.NNodes() != 0 {
		t.Fatalf("NNodes = %d, want 0: SCIPsolve must not run", solved.NNodes())
	}
	if got := solved.Stage(); got != before {
		t.Fatalf("stage = %v, want unchanged %v", got, before)
	}

	// The model still solves afterwards.
	if s := model.Solve().Status(); s != StatusOptimal {
		t.Fatalf("status = %v, want Optimal", s)
	}
}

// Interrupt from another goroutine stops a plain Solve; the leftover request
// must not stop the next solve.
func TestInterruptStopsSolve(t *testing.T) {
	model := hardTestModel(t)
	time.AfterFunc(100*time.Millisecond, model.Interrupt)

	start := time.Now()
	solved := model.Solve()
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("solve ran %v past the interrupt", elapsed)
	}
	if s := solved.Status(); s != StatusUserInterrupt {
		t.Fatalf("status = %v, want UserInterrupt", s)
	}

	solved = solved.FreeTransform()
	solved, err := solved.SetLongintParam("limits/nodes", 1)
	if err != nil {
		t.Fatal(err)
	}
	if s := solved.Solve().Status(); s == StatusUserInterrupt {
		t.Fatal("stale interrupt stopped the re-solve")
	}
}

// Interrupt on a freed or zero model is a no-op.
func TestInterruptOnFreedModel(t *testing.T) {
	var zero Model
	zero.Interrupt()

	model := createTestModel(t)
	model.Free()
	model.Interrupt() // must not panic or crash
}

type interruptingHeur struct{ done atomic.Bool }

func (h *interruptingHeur) Execute(model Model, timing HeurTiming, nodeInf bool) HeurResult {
	if !h.done.Swap(true) {
		model.Interrupt()
	}
	return HeurResultDidNotRun
}

// Interrupt from inside a plugin callback stops the solve once the callback
// returns.
func TestInterruptFromCallback(t *testing.T) {
	model := hardTestModel(t)
	h := &interruptingHeur{}
	model.Add(NewHeuristic(h).Name("stopper"))

	solved := model.Solve()
	if !h.done.Load() {
		t.Fatal("heuristic never ran")
	}
	if s := solved.Status(); s != StatusUserInterrupt {
		t.Fatalf("status = %v, want UserInterrupt", s)
	}
}

type sleepingEventHdlr struct{}

func (sleepingEventHdlr) GetEventMask() EventMask { return EventMaskPresolveRound }
func (sleepingEventHdlr) Execute(model Model, h EventhdlrPlugin, event Event) {
	time.Sleep(300 * time.Millisecond)
}

// Cancellation must also stop a solve that is still presolving.
func TestSolveContextCancelledDuringPresolve(t *testing.T) {
	model := createTestModel(t)
	model.Add(NewEventhdlr(sleepingEventHdlr{}).Name("sleeper"))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	solved, err := model.SolveContext(ctx)
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("solve ran %v past the deadline", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SolveContext error = %v, want context.DeadlineExceeded", err)
	}
	if s := solved.Status(); s != StatusUserInterrupt {
		t.Fatalf("status = %v, want UserInterrupt", s)
	}
}

// A stop request delivered while no solve is running yet — for instance a
// context that fires between the initial ctx.Err check and SCIPsolve's entry,
// where SCIP discards its own interrupt flag — must still stop the solve:
// the watcher keeps re-issuing the request, and the forwarding flag survives
// because the context path resets stale state before its watcher exists.
func TestInterruptWatcherReissuesAcrossSolveStart(t *testing.T) {
	model := hardTestModel(t)
	solveDone := make(chan struct{})
	watching := make(chan struct{})
	go func() {
		close(watching)
		model.scip.interruptWhile(solveDone)
	}()
	<-watching // the stop flag is set; no solve is running yet

	// The solve arrangement of SolveContext: no stale-state reset.
	solved, err := model.solveCore("Solve")
	close(solveDone)
	if err != nil {
		t.Fatalf("solveCore: %v", err)
	}
	if s := solved.Status(); s != StatusUserInterrupt {
		t.Fatalf("status = %v, want UserInterrupt: the stop request was lost", s)
	}
}

// A deadline that fires while another concurrent solve holds SCIP's
// process-wide thread pool must stop the wait for it: the solve never starts
// and the pool is left working for the next solve.
func TestSolveConcurrentContextWaitingForPool(t *testing.T) {
	model := hardTestModel(t)
	model, err := model.SetIntParam("parallel/maxnthreads", 2)
	if err != nil {
		t.Fatal(err)
	}
	tpiPoolAcquire(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	solved, err := model.SolveConcurrentContext(ctx)
	tpiPoolRelease()
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("wait ran %v past the deadline", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SolveConcurrentContext error = %v, want context.DeadlineExceeded", err)
	}
	if solved.NNodes() != 0 {
		t.Fatalf("NNodes = %d, want 0: the solve must not have started", solved.NNodes())
	}

	// The abandoned wait must not have wedged the pool.
	other, err := createTestModel(t).SetIntParam("parallel/maxnthreads", 2)
	if err != nil {
		t.Fatal(err)
	}
	if s := other.SolveConcurrent().Status(); s != StatusOptimal {
		t.Fatalf("follow-up concurrent solve status = %v, want Optimal", s)
	}
}

// A context deadline stops the workers of a concurrent solve, and the model
// is usable for a sequential solve afterwards.
func TestSolveConcurrentContextInterrupts(t *testing.T) {
	model := hardTestModel(t)
	model, err := model.SetIntParam("parallel/maxnthreads", 2)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	solved, err := model.SolveConcurrentContext(ctx)
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("concurrent solve ran %v past the deadline", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SolveConcurrentContext error = %v, want context.DeadlineExceeded", err)
	}
	var e *Error
	if !errors.As(err, &e) || e.Op != "SolveConcurrentContext" {
		t.Fatalf("error is not an *Error of SolveConcurrentContext: %v (%T)", err, err)
	}
	if s := solved.Status(); s != StatusUserInterrupt {
		t.Fatalf("status = %v, want UserInterrupt", s)
	}

	// No stop flag may linger into the next solve.
	solved = solved.FreeTransform()
	if _, err := solved.SetRealParam("limits/time", 2); err != nil {
		t.Fatal(err)
	}
	next, err := solved.SolveContext(context.Background())
	if err != nil {
		t.Fatalf("sequential solve after interrupted concurrent solve: %v", err)
	}
	if s := next.Status(); s == StatusUserInterrupt {
		t.Fatal("stale interrupt stopped the follow-up solve")
	}
}
