package scip

import (
	"errors"
	"testing"
)

func asError(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("got %T %v, want *Error", err, err)
	}
	return e
}

func TestTryAddVarAfterSolve(t *testing.T) {
	m := createTestModel(t).Solve()
	defer m.Free()
	_, err := m.TryAddVar(0, 1, 1, "late", VarTypeBinary)
	e := asError(t, err)
	if e.Op != "AddVar" || e.Stage != StageSolved || !errors.Is(err, RetcodeInvalidCall) || e.Detail != "late" {
		t.Fatalf("unexpected error %+v", e)
	}
	if e.Unwrap() != RetcodeInvalidCall {
		t.Fatal("Unwrap should yield the Retcode")
	}
}

func TestPanicCarriesError(t *testing.T) {
	m := createTestModel(t).Solve()
	defer m.Free()
	defer func() {
		e, ok := recover().(*Error)
		if !ok || e.Op != "AddVar" || e.Stage != StageSolved {
			t.Fatalf("panic value %v, want *Error for AddVar in Solved", e)
		}
	}()
	m.AddVar(0, 1, 1, "late", VarTypeBinary)
}

func TestTryIncludeDuplicateBranchRule(t *testing.T) {
	m := NewModel().HideOutput().IncludeDefaultPlugins()
	defer m.Free()
	rule := cuttingOffBranchingRule{}
	if err := m.TryIncludeBranchRule("dup", "", 1, -1, 1, rule); err != nil {
		t.Fatal(err)
	}
	err := m.TryIncludeBranchRule("dup", "", 1, -1, 1, rule)
	e := asError(t, err)
	if e.Op != "IncludeBranchRule" || e.Detail != "dup" || !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("unexpected error %+v", e)
	}
}

func TestTryParamErrors(t *testing.T) {
	m := NewModel()
	defer m.Free()
	_, err := m.TryIntParam("no/such/param")
	e := asError(t, err)
	if e.Op != "IntParam" || e.Detail != "no/such/param" || !errors.Is(err, RetcodeParameterUnknown) {
		t.Fatalf("unexpected error %+v", e)
	}
	if _, err := m.SetIntParam("display/verblevel", -1); !errors.Is(err, RetcodeParameterWrongVal) {
		t.Fatalf("got %v", err)
	}
}

func TestFreedModelReturnsError(t *testing.T) {
	m := createTestModel(t)
	m.Free()
	_, err := m.TryAddVar(0, 1, 1, "x", VarTypeBinary)
	e := asError(t, err)
	if e.Stage != StageFree || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("unexpected error %+v", e)
	}
	if _, err := m.TrySolve(); asError(t, err).Stage != StageFree {
		t.Fatal("TrySolve on a freed model should fail in StageFree")
	}
	if err := m.TryFree(); err != nil {
		t.Fatalf("second Free: %v", err)
	}
	if (Model{}).Stage() != StageFree {
		t.Fatal("zero Model should report StageFree")
	}
}

func TestTryAddValidation(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	err := m.TryAdd(42)
	if e := asError(t, err); e.Op != "Add" || !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("unexpected error %+v", e)
	}
	x := m.AddVar(0, 10, 1, "cont", VarTypeContinuous)
	_, err = m.TryAddConsSetPart([]Variable{x}, "sp")
	if e := asError(t, err); e.Op != "AddConsSetPart" || e.Detail == "" {
		t.Fatalf("unexpected error %+v", e)
	}
	_, err = m.TryAddConsLocal(NewCons().Expression(x.Expr().Pow(2)).Le(1))
	if !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("got %v", err)
	}
}

type panickingHeur struct{}

func (panickingHeur) Execute(Model, HeurTiming, bool) HeurResult { panic("boom") }

func TestCallbackPanicReturned(t *testing.T) {
	m := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp"))
	defer m.Free()
	m.Add(NewHeur(panickingHeur{}).Name("boom"))
	_, err := m.TrySolve()
	var cp *CallbackPanic
	if !errors.As(err, &cp) {
		t.Fatalf("got %T %v, want *CallbackPanic", err, err)
	}
	if cp.Value != "boom" || cp.Plugin != "heuristic scip.panickingHeur" {
		t.Fatalf("unexpected %+v", cp)
	}
	// the model is still usable
	if m.Stage() != StageSolved && m.Stage() != StageSolving {
		t.Fatalf("stage %v", m.Stage())
	}
}

func TestSolveStillPanicsOnCallbackPanic(t *testing.T) {
	m := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp"))
	defer m.Free()
	m.Add(NewHeur(panickingHeur{}).Name("boom"))
	defer func() {
		if _, ok := recover().(*CallbackPanic); !ok {
			t.Fatal("Solve should panic with *CallbackPanic")
		}
	}()
	m.Solve()
}

func TestErrorString(t *testing.T) {
	e := &Error{Op: "AddVar", Stage: StageSolved, Retcode: RetcodeInvalidCall, Detail: "x"}
	if got, want := e.Error(), "scip: AddVar in stage Solved: SCIP_INVALIDCALL (x)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if Stage(99).String() != "Stage(99)" {
		t.Fatal("out-of-range stage string")
	}
}
