package scip

import (
	"errors"
	"testing"
)

// expectErrorPanic runs f and asserts it panics with an *Error whose Retcode
// matches want; a process abort or a non-Error panic fails the test.
func expectErrorPanic(t *testing.T, name string, want Retcode, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		e, ok := r.(*Error)
		if !ok {
			t.Errorf("%s: panicked with %T %v, want *Error", name, r, r)
			return
		}
		if !errors.Is(e, want) {
			t.Errorf("%s: got %v, want %v", name, e, want)
		}
	}()
	f()
}

// queryCalls lists every Model query that must never abort the process.
func queryCalls(m Model) map[string]func() {
	return map[string]func(){
		"Status": func() { m.Status() }, "NVars": func() { m.NVars() }, "Vars": func() { m.Vars() },
		"OrigVars": func() { m.OrigVars() }, "Var": func() { m.Var(0) }, "NConss": func() { m.NConss() },
		"Conss": func() { m.Conss() }, "FindCons": func() { m.FindCons("c") }, "ObjVal": func() { m.ObjVal() },
		"BestBound": func() { m.BestBound() }, "NNodes": func() { m.NNodes() }, "SolvingTime": func() { m.SolvingTime() },
		"NLpIterations": func() { m.NLpIterations() }, "NSols": func() { m.NSols() }, "GetSols": func() { m.GetSols() },
		"BestSol": func() { m.BestSol() }, "FocusNode": func() { m.FocusNode() }, "LpObjVal": func() { m.LpObjVal() },
		"LpStatus": func() { m.LpStatus() }, "VarInProb": func() { m.VarInProb(0) }, "Eps": func() { m.Eps() },
		"Eq": func() { m.Eq(1, 1) }, "Heurs": func() { m.Heurs() }, "FindHeur": func() { m.FindHeur("x") },
		"Separators": func() { m.Separators() }, "Presolvers": func() { m.Presolvers() }, "PrintVersion": m.PrintVersion,
		"BestNode": func() { m.BestNode() }, "Leaves": func() { m.Leaves() }, "InProbing": func() { InProbing(m) },
		"InDive": func() { InDive(m) },
	}
}

func TestQueriesOnFreedModelPanicWithError(t *testing.T) {
	m := createTestModel(t)
	m.Free()
	for name, f := range queryCalls(m) {
		expectErrorPanic(t, name, RetcodeInvalidCall, f)
	}
	expectErrorPanic(t, "zero Model", RetcodeInvalidCall, func() { (Model{}).NVars() })
}

func TestQueriesInInitStageDoNotAbort(t *testing.T) {
	m := NewModel().HideOutput().IncludeDefaultPlugins() // StageInit: SCIP getters would abort
	defer m.Free()
	// legal in every stage
	for name, f := range map[string]func(){"Status": func() { m.Status() }, "Eps": func() { m.Eps() },
		"Heurs": func() { m.Heurs() }, "InProbing": func() { InProbing(m) }, "InDive": func() { InDive(m) }} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s in Init: %v", name, r)
				}
			}()
			f()
		}()
	}
	if m.BestNode() != nil || len(m.Leaves()) != 0 {
		t.Error("tree accessors should be nil outside solving")
	}
	// stage-restricted queries panic with *Error instead of aborting
	for _, name := range []string{"NVars", "Vars", "NConss", "Conss", "ObjVal", "NSols", "FocusNode", "LpObjVal", "VarInProb", "FindCons", "NNodes", "SolvingTime"} {
		expectErrorPanic(t, name, RetcodeInvalidCall, queryCalls(m)[name])
	}
}

func TestTryQueriesReportStage(t *testing.T) {
	m := createTestModel(t) // StageProblem
	defer m.Free()
	if _, err := m.TryObjVal(); asError(t, err).Stage != StageProblem || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("ObjVal in Problem: %v", err)
	}
	if m.BestNode() != nil || m.Children() != nil { // the case that killed the #4 test binary
		t.Fatal("tree accessors in the problem stage should be nil")
	}
	x := m.Vars()[0]
	expectErrorPanic(t, "CurrentVal in Problem", RetcodeInvalidCall, func() { m.CurrentVal(x) })
	sol := m.CreateOrigSol()
	if sol.ObjVal() != 0 || sol.String() == "" || len(sol.AsNameMap()) != 0 {
		t.Fatal("original solution accessors are legal in the problem stage")
	}
	if n, err := m.TryNVars(); err != nil || n != 2 {
		t.Fatalf("NVars: %d %v", n, err)
	}
	if _, _, err := m.TryBestSol(); err != nil {
		t.Fatalf("BestSol in Problem should be legal: %v", err)
	}
	if _, _, err := NewModel().TryBestSol(); err != nil {
		t.Fatalf("BestSol in Init is legal per scip_sol.c: %v", err)
	}
	if _, err := m.TryNNodes(); err != nil {
		t.Fatalf("NNodes in Problem: %v", err)
	}
	solved := m.Solve()
	for name, f := range map[string]func() error{
		"Status":      func() error { _, e := solved.TryStatus(); return e },
		"ObjVal":      func() error { _, e := solved.TryObjVal(); return e },
		"BestBound":   func() error { _, e := solved.TryBestBound(); return e },
		"Vars":        func() error { _, e := solved.TryVars(); return e },
		"Conss":       func() error { _, e := solved.TryConss(); return e },
		"NConss":      func() error { _, e := solved.TryNConss(); return e },
		"SolvingTime": func() error { _, e := solved.TrySolvingTime(); return e },
	} {
		if err := f(); err != nil {
			t.Errorf("%s after solve: %v", name, err)
		}
	}
	m.Free()
	if _, err := m.TryStatus(); asError(t, err).Stage != StageFree {
		t.Fatal("TryStatus on freed model")
	}
}

func TestHandleMethodsOnFreedOrZeroHandles(t *testing.T) {
	m := createTestModel(t)
	x := m.Vars()[0]
	c := m.Conss()[0]
	sol := m.CreateOrigSol()
	m.Free()
	expectErrorPanic(t, "Variable.Name freed", RetcodeInvalidCall, func() { x.Name() })
	expectErrorPanic(t, "Variable.Lb freed", RetcodeInvalidCall, func() { x.Lb() })
	expectErrorPanic(t, "Constraint.Name freed", RetcodeInvalidCall, func() { c.Name() })
	expectErrorPanic(t, "Solution.ObjVal freed", RetcodeInvalidCall, func() { sol.ObjVal() })
	expectErrorPanic(t, "Variable{} ", RetcodeInvalidData, func() { (Variable{}).Name() })
	expectErrorPanic(t, "Constraint{}", RetcodeInvalidData, func() { (Constraint{}).Name() })
	expectErrorPanic(t, "Row{}", RetcodeInvalidData, func() { (Row{}).Name() })
	expectErrorPanic(t, "Col{}", RetcodeInvalidData, func() { (Col{}).Index() })
	expectErrorPanic(t, "Node{}", RetcodeInvalidData, func() { (Node{}).Number() })
	expectErrorPanic(t, "Solution{}", RetcodeInvalidData, func() { (Solution{}).ObjVal() })
	if x.safeName() != "<Variable of a freed model>" {
		t.Fatal("safeName on freed")
	}
}

func TestHandleGettersRespectStage(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	x := m.Vars()[0]
	expectErrorPanic(t, "Variable.SolVal in Problem", RetcodeInvalidCall, func() { x.SolVal() })
	if _, ok := x.Redcost(); ok {
		t.Fatal("Redcost outside solving should report unavailable")
	}
	solved := m.Solve()
	x = solved.Vars()[0]
	// SCIPgetVarSol is only legal while presolved or solving, so even after
	// Solve it must produce *Error rather than an abort.
	expectErrorPanic(t, "Variable.SolVal in Solved", RetcodeInvalidCall, func() { x.SolVal() })
	if sol, ok := solved.BestSol(); !ok || sol.Val(x) < 0 {
		t.Fatal("Solution.Val after solving")
	}
}

func TestMutatorTryFormsValidate(t *testing.T) {
	m := createTestModel(t) // StageProblem: neither probing nor diving
	defer m.Free()
	x := m.Vars()[0]
	p := &Prober{scip: m.scip}
	d := &Diver{scip: m.scip}
	for name, err := range map[string]error{
		"Prober.NewNode":    p.TryNewNode(),
		"Prober.ChgVarLb":   p.TryChgVarLb(x, 0),
		"Prober.FixVar":     p.TryFixVar(x, 0),
		"Prober.AddRow":     p.TryAddRow(Row{}),
		"Diver.ChgVarUb":    d.TryChgVarUb(x, 1),
		"Diver.ChgRowLhs":   d.TryChgRowLhs(Row{}, 0),
		"Diver.AddRow":      d.TryAddRow(Row{}),
		"Diver.ChgCutoff":   d.TryChgCutoffBound(1),
		"Diver.End":         d.TryEnd(),
		"Row{}.SetCoeff":    (&Row{}).TrySetCoeff(x, 1),
		"Solution{}.SetVal": (Solution{}).TrySetVal(x, 1),
		"Node{}.Children":   func() error { _, e := (Node{}).TryChildren(); return e }(),
	} {
		var e *Error
		if !errors.As(err, &e) {
			t.Errorf("%s: got %v, want *Error", name, err)
		}
	}
	if _, _, err := p.TryPropagate(1); !errors.Is(err, RetcodeInvalidCall) {
		t.Errorf("Propagate outside probing: %v", err)
	}
	sol := m.CreateOrigSol()
	if err := sol.TrySetVal(Variable{}, 1); !errors.Is(err, RetcodeInvalidData) {
		t.Errorf("SetVal zero var: %v", err)
	}
	if err := sol.TrySetVal(x, 1); err != nil {
		t.Errorf("SetVal valid: %v", err)
	}
}
