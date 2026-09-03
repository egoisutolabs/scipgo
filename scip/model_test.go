package scip

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadProbFailureBalancesRefcounts(t *testing.T) {
	model := NewModel().HideOutput().IncludeDefaultPlugins()

	_, err := model.ReadProb(testFile("bad.opb"))
	if err == nil {
		t.Fatal("expected read failure")
	}

	// SCIP created variable x1 before the constraint failed. Dropping the
	// model must not crash from an unbalanced release; the next solve proves
	// the library still works.
	solved := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp")).Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
}

func TestSolveFromLpFile(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp")).Solve()

	if model.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", model.Status())
	}
	if model.ObjVal() != 200 {
		t.Fatalf("got obj val %v, want 200", model.ObjVal())
	}
	if got := len(model.Conss()); got != 2 {
		t.Fatalf("got %d constraints, want 2", got)
	}

	sol, ok := model.BestSol()
	if !ok {
		t.Fatal("expected a best solution")
	}
	vars := model.Vars()
	if len(vars) != 2 {
		t.Fatalf("got %d vars, want 2", len(vars))
	}
	if sol.Val(vars[0]) != 40 || sol.Val(vars[1]) != 20 {
		t.Fatalf("got (%v, %v), want (40, 20)", sol.Val(vars[0]), sol.Val(vars[1]))
	}
	if sol.ObjVal() != model.ObjVal() {
		t.Fatalf("solution obj %v != model obj %v", sol.ObjVal(), model.ObjVal())
	}
}

func TestSetObjIntegral(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp")).
		SetObjIntegral().
		Solve()
	if model.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", model.Status())
	}
	if model.ObjVal() != 200 {
		t.Fatalf("got obj value %v, want 200", model.ObjVal())
	}
}

func TestSetTimeLimit(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		SetTimeLimit(0).
		IncludeDefaultPlugins(), testFile("simple.lp")).Solve()
	if model.Status() != StatusTimeLimit {
		t.Fatalf("got status %v, want TimeLimit", model.Status())
	}
	if model.SolvingTime() > 0.5 {
		t.Fatalf("solving time %v too high", model.SolvingTime())
	}
	if model.NNodes() != 0 || model.NLpIterations() != 0 {
		t.Fatal("expected 0 nodes and LP iterations")
	}
}

func TestSetMemoryLimit(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		SetMemoryLimit(0).
		IncludeDefaultPlugins(), testFile("simple.lp")).Solve()
	if model.Status() != StatusMemoryLimit {
		t.Fatalf("got status %v, want MemoryLimit", model.Status())
	}
	if model.NNodes() != 0 || model.NLpIterations() != 0 {
		t.Fatal("expected 0 nodes and LP iterations")
	}
}

func TestAddVariable(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, Infinity, 3, "x1", VarTypeInteger)
	x2 := model.AddVar(0, Infinity, 4, "x2", VarTypeContinuous)

	if model.NVars() != 2 || len(model.Vars()) != 2 {
		t.Fatalf("expected 2 vars")
	}
	if x1.raw == x2.raw {
		t.Fatal("variables share the same pointer")
	}
	if x1.VarType() != VarTypeInteger || x2.VarType() != VarTypeContinuous {
		t.Fatal("wrong variable types")
	}
	if x1.Name() != "x1" || x2.Name() != "x2" {
		t.Fatal("wrong names")
	}
	if x1.Obj() != 3 || x2.Obj() != 4 {
		t.Fatal("wrong objective coefficients")
	}
}

func TestTrySolveOnValidModel(t *testing.T) {
	solved, err := createTestModel(t).TrySolve()
	if err != nil {
		t.Fatalf("try_solve: %v", err)
	}
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
}

func TestSolveConcurrentOnValidModel(t *testing.T) {
	model := createTestModel(t)
	model, err := model.SetIntParam("parallel/maxnthreads", 2)
	if err != nil {
		t.Fatalf("set param: %v", err)
	}
	solved := model.SolveConcurrent()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
	if solved.ObjVal() != 200 {
		t.Fatalf("got obj %v, want 200", solved.ObjVal())
	}
}

func TestStatsJSONAfterSolve(t *testing.T) {
	solved := createTestModel(t).Solve()
	j := solved.StatsJSON()
	if len(j) == 0 || j[0] != '{' {
		t.Fatalf("stats json doesn't look like JSON: %q", j[:min(30, len(j))])
	}
	if !contains(j, "optimal solution found") {
		t.Fatal("stats json missing optimal status")
	}
}

func TestWriteStatsJSONAfterSolve(t *testing.T) {
	solved := createTestModel(t).Solve()
	path := filepath.Join(t.TempDir(), "stats.json")
	if err := solved.WriteStatsJSON(path); err != nil {
		t.Fatalf("write stats: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[0] != '{' || !contains(string(data), "optimal solution found") {
		t.Fatalf("bad stats file: %q", data[:min(40, len(data))])
	}
}

func TestBuildModelWithFunctions(t *testing.T) {
	model := createTestModel(t)
	if model.Vars()[0].Name() != "x1" {
		t.Fatal("unexpected var order")
	}
	if model.NConss() != 2 || len(model.Conss()) != 2 {
		t.Fatalf("expected 2 constraints")
	}
	if model.Conss()[0].Name() != "c1" || model.Conss()[1].Name() != "c2" {
		t.Fatal("wrong constraint names")
	}

	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
	if solved.ObjVal() != 200 {
		t.Fatalf("got obj %v, want 200", solved.ObjVal())
	}
	sol, _ := solved.BestSol()
	vars := solved.Vars()
	if sol.Val(vars[0]) != 40 || sol.Val(vars[1]) != 20 {
		t.Fatalf("got (%v,%v), want (40,20)", sol.Val(vars[0]), sol.Val(vars[1]))
	}
}

func TestUnboundedModel(t *testing.T) {
	model := DefaultModel().Maximize().HideOutput()
	model.AddVar(0, Infinity, 1, "x1", VarTypeInteger)
	model.AddVar(0, Infinity, 1, "x2", VarTypeInteger)

	solved := model.Solve()
	if solved.Status() != StatusUnbounded {
		t.Fatalf("got status %v, want Unbounded", solved.Status())
	}
	if _, ok := solved.BestSol(); !ok {
		t.Fatal("expected a solution")
	}
}

func TestInfeasibleModel(t *testing.T) {
	model := DefaultModel().Maximize().HideOutput()
	v := model.AddVar(0, 1, 1, "x1", VarTypeInteger)
	model.AddCons([]Variable{v}, []float64{1}, NegInfinity, -1, "c1")

	solved := model.Solve()
	if solved.Status() != StatusInfeasible {
		t.Fatalf("got status %v, want Infeasible", solved.Status())
	}
	if solved.NSols() != 0 {
		t.Fatal("expected 0 solutions")
	}
	if _, ok := solved.BestSol(); ok {
		t.Fatal("expected no best solution")
	}
}

func TestScipPtr(t *testing.T) {
	model := createTestModel(t)
	if model.ScipPtr() == nil {
		t.Fatal("nil scip pointer")
	}
}

func TestWriteAndReadLp(t *testing.T) {
	dir := t.TempDir()
	lpFile := filepath.Join(dir, "test.lp")

	model := createTestModel(t)
	if err := model.Write(lpFile, "lp", false); err != nil {
		t.Fatal(err)
	}
	readModel := mustRead(t, NewModel().IncludeDefaultPlugins(), lpFile)

	solved := model.Solve()
	readSolved := readModel.Solve()

	if solved.Status() != readSolved.Status() {
		t.Fatal("statuses differ")
	}
	if solved.ObjVal() != readSolved.ObjVal() {
		t.Fatalf("obj values differ: %v vs %v", solved.ObjVal(), readSolved.ObjVal())
	}
}

func TestWriteSymbolicNames(t *testing.T) {
	model := NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("t").Minimize()
	model.AddVar(0, 1, 1, "myvar", VarTypeBinary)
	dir := t.TempDir()
	for _, tc := range []struct {
		symb bool
		want bool // expect "myvar" in the file
	}{{true, true}, {false, false}} {
		path := filepath.Join(dir, fmt.Sprintf("p_%v.lp", tc.symb))
		if err := model.Write(path, "lp", tc.symb); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(b), "myvar"); got != tc.want {
			t.Fatalf("symb=%v: contains myvar=%v, want %v", tc.symb, got, tc.want)
		}
	}
}

func TestPrintVersion(t *testing.T) {
	NewModel().PrintVersion()
}

func TestFreeTransform(t *testing.T) {
	model := createTestModel(t)
	solved := model.Solve()
	objVal := solved.ObjVal()

	second := solved.FreeTransform()
	x3 := second.AddVar(0, Infinity, 1, "x3", VarTypeInteger)
	bound := 2.0
	second.AddCons([]Variable{x3}, []float64{1}, 0, bound, "x3-cons")

	secondSolved := second.Solve()
	if secondSolved.Status() != StatusOptimal {
		t.Fatalf("got status %v", secondSolved.Status())
	}
	if d := secondSolved.ObjVal() - (objVal + bound); d > 1e-6 || d < -1e-6 {
		t.Fatalf("obj %v, want %v", secondSolved.ObjVal(), objVal+bound)
	}
}

func TestBestBound(t *testing.T) {
	solved := createTestModel(t).Solve()
	if d := solved.BestBound() - solved.ObjVal(); d > 1e-6 || d < -1e-6 {
		t.Fatalf("bound %v != obj %v", solved.BestBound(), solved.ObjVal())
	}
}

func TestComparison(t *testing.T) {
	model := NewModel()
	eps := model.Eps()
	if !model.Eq(1.0, 1-eps) {
		t.Fatal("1.0 should equal 1-eps")
	}
	if !model.Lt(1-2*eps, 1.0) {
		t.Fatal("1-2eps should be less than 1")
	}
	if !model.Gt(1.0, 1-2*eps) {
		t.Fatal("1 should be greater than 1-2eps")
	}
	if !model.Le(1-eps, 1.0) {
		t.Fatal("1-eps should be <= 1")
	}
	if !model.Ge(1.0, 1-eps) {
		t.Fatal("1 should be >= 1-eps")
	}
}

func TestThreadSafety(t *testing.T) {
	const n = 100
	done := make(chan Status, n)
	for i := 0; i < n; i++ {
		go func() {
			model := NewModel().
				HideOutput().
				IncludeDefaultPlugins().
				CreateProb("test").
				Maximize()
			x1 := model.AddVar(0, Infinity, 3, "x1", VarTypeInteger)
			x2 := model.AddVar(0, Infinity, 4, "x2", VarTypeInteger)
			model.AddCons([]Variable{x1, x2}, []float64{2, 1}, NegInfinity, 100, "c1")
			model.AddCons([]Variable{x1, x2}, []float64{1, 2}, NegInfinity, 80, "c2")
			done <- model.Solve().Status()
		}()
	}
	for i := 0; i < n; i++ {
		if s := <-done; s != StatusOptimal {
			t.Fatalf("got status %v, want Optimal", s)
		}
	}
}

func TestVarByIndexAfterSolve(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp")).Solve()
	for _, v := range model.Vars() {
		got, ok := model.Var(v.Index())
		if !ok || got.Name() != v.Name() {
			t.Fatalf("Var(%d) = %v ok=%v, want %v", v.Index(), got.Name(), ok, v.Name())
		}
	}
}

func TestGCFinalizerCleanup(t *testing.T) {
	// Create and solve many models, dropping them immediately, and force GC
	// in between to exercise the Scip finalizer (release + SCIPfree).
	for i := 0; i < 25; i++ {
		model := createTestModel(t)
		solved := model.Solve()
		if solved.Status() != StatusOptimal {
			t.Fatalf("iteration %d: got status %v", i, solved.Status())
		}
		runtime.GC()
	}
	runtime.GC()
	runtime.GC()
	// library still usable afterwards
	if solved := createTestModel(t).Solve(); solved.Status() != StatusOptimal {
		t.Fatal("library broken after GC")
	}
}

func TestModelFree(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp")).Solve()
	model.Free()
	model.Free() // idempotent
	if model.scip.raw != nil {
		t.Fatal("instance not freed")
	}
}

func TestTwoConcurrentSolvesThenFree(t *testing.T) {
	// SCIP's thread pool is global: freeing two instances that both ran a
	// concurrent solve used to crash in SCIPtpiExit.
	a := createTestModel(t)
	a, _ = a.SetIntParam("parallel/maxnthreads", 2)
	a.SolveConcurrent()
	b := createTestModel(t)
	b, _ = b.SetIntParam("parallel/maxnthreads", 2)
	if b.SolveConcurrent().ObjVal() != 200 {
		t.Fatal("wrong objective")
	}
	a.Free()
	b.Free()
}

func TestConcurrentResolveAfterOtherInstance(t *testing.T) {
	a := createTestModel(t)
	a, _ = a.SetIntParam("parallel/maxnthreads", 2)
	a.SolveConcurrent()
	b := createTestModel(t)
	b, _ = b.SetIntParam("parallel/maxnthreads", 2)
	b.SolveConcurrent()
	b.Free() // destroys the pool a still expects
	a = a.FreeTransform()
	if a.SolveConcurrent().ObjVal() != 200 {
		t.Fatal("wrong objective on re-solve")
	}
	a.Free()
}
