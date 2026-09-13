package scip

import (
	"math/big"
	"testing"
)

func TestExactSolveSimple(t *testing.T) {
	model := NewModel().
		EnableExactSolving().
		IncludeDefaultPlugins()
	defer model.Free()
	if _, err := model.ReadProb(testFile("simple.mps")); err != nil {
		t.Fatalf("read: %v", err)
	}
	model.HideOutput().Solve()
	if model.Status() != StatusOptimal {
		t.Fatalf("status = %v, want Optimal", model.Status())
	}
	sol, ok := model.BestSol()
	if !ok {
		t.Fatal("no solution")
	}
	obj := sol.ObjValExact()
	if obj.Cmp(big.NewRat(27, 1)) != 0 {
		t.Fatalf("ObjValExact = %s, want 27/1", obj.RatString())
	}
	if got, _ := obj.Float64(); got != model.ObjVal() {
		t.Fatalf("float projection %v != ObjVal %v", got, model.ObjVal())
	}
	for _, v := range model.Vars() {
		if _, err := sol.TryValExact(v); err != nil {
			t.Fatalf("ValExact(%s): %v", v.Name(), err)
		}
	}
}

// TestExactSolveP0201 checks a real MIP end to end in exact mode: the
// rational optimum is MIPLIB's documented value for p0201.
func TestExactSolveP0201(t *testing.T) {
	model := NewModel().
		EnableExactSolving().
		IncludeDefaultPlugins()
	defer model.Free()
	if _, err := model.ReadProb(testFile("p0201.mps")); err != nil {
		t.Fatalf("read: %v", err)
	}
	model.HideOutput().Solve()
	if model.Status() != StatusOptimal {
		t.Fatalf("status = %v", model.Status())
	}
	sol, ok := model.BestSol()
	if !ok {
		t.Fatal("no solution")
	}
	obj := sol.ObjValExact()
	if obj.Cmp(big.NewRat(7615, 1)) != 0 {
		t.Fatalf("ObjValExact = %s, want 7615/1", obj.RatString())
	}
}

// TestEnableExactStageError checks the Init-stage rule is enforced before
// reaching C.
func TestEnableExactStageError(t *testing.T) {
	model := mustRead(t, NewModel().IncludeDefaultPlugins(), testFile("simple.mps"))
	defer model.Free()
	err := model.TryEnableExactSolving(true)
	if err == nil || asError(t, err).Retcode != RetcodeInvalidCall {
		t.Fatalf("want staging error, got %v", err)
	}
}

// TestValExactRequiresExactMode checks the non-exact case is rejected before
// reaching C: SCIP has no rational value to report and would crash.
func TestValExactRequiresExactMode(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.mps"))
	defer model.Free()
	model.Solve()
	sol, ok := model.BestSol()
	if !ok {
		t.Fatal("no solution")
	}
	if _, err := sol.TryObjValExact(); err == nil || asError(t, err).Retcode != RetcodeInvalidCall {
		t.Fatalf("want InvalidCall, got %v", err)
	}
	if _, err := sol.TryValExact(model.Vars()[0]); err == nil || asError(t, err).Retcode != RetcodeInvalidCall {
		t.Fatalf("want InvalidCall, got %v", err)
	}
}
