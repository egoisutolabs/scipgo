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

type panickingPricer struct{}

func (panickingPricer) GenerateColumns(Model, PricerPlugin, bool) PricerResult { panic("no columns") }

func TestCallbackPanicNamesPricer(t *testing.T) {
	m := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp"))
	defer m.Free()
	m.Add(NewPricer(panickingPricer{}).Name("boom"))
	_, err := m.TrySolve()
	var cp *CallbackPanic
	if !errors.As(err, &cp) || cp.Plugin != "pricer scip.panickingPricer" || cp.Value != "no columns" {
		t.Fatalf("got %T %v", err, err)
	}
}

type panickingConshdlr struct{}

func (panickingConshdlr) Check(Model, ConshdlrPlugin, Solution) bool { panic("check boom") }
func (panickingConshdlr) Enforce(Model, ConshdlrPlugin) ConshdlrResult {
	return ConshdlrResultFeasible
}

func TestAddSolCallbackPanicConsumesSolution(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	m.IncludeConshdlr("boom", "", -1, -1, panickingConshdlr{})
	sol := m.CreateOrigSol()
	err := m.AddSol(&sol)
	var cp *CallbackPanic
	if !errors.As(err, &cp) || cp.Value != "check boom" {
		t.Fatalf("got %T %v", err, err)
	}
	if sol.raw != nil {
		t.Fatal("solution should be consumed on the panic path")
	}
}

func TestBuilderAddToWithoutProblem(t *testing.T) {
	m := NewModel().HideOutput().IncludeDefaultPlugins() // StageInit: count getters would abort
	defer m.Free()
	_, err := NewVar().TryAddTo(m)
	if e := asError(t, err); e.Op != "AddVar" || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("unexpected %+v", e)
	}
	if _, err := NewCons().Le(1).TryAddTo(m); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("got %v", err)
	}
	// an explicit name skips the count getter and reaches SCIP, which rejects it
	if _, err := NewVar().Name("x").TryAddTo(m); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("got %v", err)
	}
}

func TestTrySetBoundNilNode(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	x := m.Vars()[0]
	if err := m.TrySetUbNode(nil, x, 0); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("got %v", err)
	}
	var none *Node // e.g. BestNode() on an empty tree
	if err := m.TrySetLbNode(none, x, 0); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("got %v", err)
	}
}

func TestTreeOpsOutsideSolvingReturnErrors(t *testing.T) {
	m := createTestModel(t) // StageProblem: the underlying SCIP getters would abort
	defer m.Free()
	if _, err := m.TryFocusNode(); asError(t, err).Stage != StageProblem || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("FocusNode: %v", err)
	}
	if _, err := m.TryCreateChild(); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("CreateChild: %v", err)
	}
	if _, err := m.TryStartDiving(); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("StartDiving: %v", err)
	}
}

func TestConvenienceMethodsReportOwnOp(t *testing.T) {
	m := NewModel()
	_, err := m.TrySetDisplayVerbosity(-1)
	if asError(t, err).Op != "SetDisplayVerbosity" {
		t.Fatalf("got %v", err)
	}
	m.Free()
	for name, f := range map[string]func() error{
		"SetTimeLimit":   func() error { _, e := m.TrySetTimeLimit(1); return e },
		"SetMemoryLimit": func() error { _, e := m.TrySetMemoryLimit(1); return e },
		"Maximize":       func() error { _, e := m.TryMaximize(); return e },
		"Minimize":       func() error { _, e := m.TryMinimize(); return e },
	} {
		if got := asError(t, f()).Op; got != name {
			t.Fatalf("Op %q, want %q", got, name)
		}
	}
}

func TestWrapKeepsWrappedRetcode(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	_, err := m.TryAddConsNonlinear(ParseExpr("<<< not an expression"), 0, 1, "bad")
	e := asError(t, err)
	if e.Retcode == RetcodeInvalidData || e.Detail == "" {
		t.Fatalf("wrapped SCIP retcode lost: %+v", e)
	}
}

func TestZeroHandlesRejected(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	x := m.Vars()[0]
	c := m.Conss()[0]
	cases := map[string]error{
		"SetHeurPriority":   m.TrySetHeurPriority(HeurPlugin{}, 1),
		"SetSepaPriority":   m.TrySetSepaPriority(SeparatorPlugin{}, 1),
		"AddConsCoef cons":  m.TryAddConsCoef(Constraint{}, x, 1),
		"AddConsCoef var":   m.TryAddConsCoef(c, Variable{}, 1),
		"AddConsCoefSetppc": m.TryAddConsCoefSetppc(c, Variable{}),
		"SetConsModifiable": m.TrySetConsModifiable(Constraint{}, true),
		"AddCut":            func() error { _, e := m.TryAddCut(Row{}, false); return e }(),
		"SetUbNode var":     m.TrySetUbNode(&Node{}, Variable{}, 0),
	}
	for name, err := range cases {
		if !errors.Is(err, RetcodeInvalidData) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if _, err := m.TryAddCons([]Variable{{}}, []float64{1}, 0, 1, "z"); !errors.Is(err, RetcodeInvalidData) {
		t.Errorf("AddCons: %v", err)
	}
	if _, err := m.TryAddConsSetPart([]Variable{{}}, "z"); !errors.Is(err, RetcodeInvalidData) {
		t.Errorf("AddConsSetPart: %v", err)
	}
	if _, err := m.TryAddConsNonlinear(Variable{}.Expr().Pow(2), 0, 1, "z"); !errors.Is(err, RetcodeInvalidData) {
		t.Errorf("AddConsNonlinear: %v", err)
	}
	if (Variable{}).Expr().String() != "<zero Variable>" {
		t.Error("zero variable expression string")
	}
}

func TestSelfReviewZeroNodesAndNilSolution(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	x := m.Vars()[0]
	if err := m.TrySetUbNode(&Node{}, x, 0); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("zero node: %v", err)
	}
	if _, err := m.TryAddConsNode(&Node{}, NewCons().Coef(x, 1).Le(1)); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("zero node cons: %v", err)
	}
	if err := m.AddSol(nil); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("nil solution: %v", err)
	}
	sol := m.CreateOrigSol()
	_ = m.AddSol(&sol) // consumes
	if err := m.AddSol(&sol); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("consumed solution: %v", err)
	}
	p := &Prober{scip: m.scip} // not probing: the depth getter would abort
	if err := p.TryBacktrack(0); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("backtrack outside probing: %v", err)
	}
	if err := p.TryEnd(); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("end outside probing: %v", err)
	}
}

func TestDiverEndOutsideSolving(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	d := &Diver{scip: m.scip}
	if err := d.TryEnd(); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("got %v", err)
	}
}
