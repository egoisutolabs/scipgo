package scip

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"weak"
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
	if _, _, err := NewModel().TryBestSol(); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("BestSol in Init reads SCIPgetNSols first, which is not permitted: %v", err)
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
	if x.safeName() != "<Variable belongs to a freed model>" {
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

func TestFreeTransformInvalidatesTransformedHandles(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	orig := m.Vars()[0] // original-problem variable
	solved := m.Solve()
	trans := solved.Vars()[0] // transformed variable
	sol, ok := solved.BestSol()
	if !ok || trans.IsOriginal() {
		t.Fatal("expected a transformed variable and an incumbent")
	}
	solved.FreeTransform()
	expectErrorPanic(t, "transformed var after FreeTransform", RetcodeInvalidCall, func() { trans.Name() })
	expectErrorPanic(t, "solution after FreeTransform", RetcodeInvalidCall, func() { sol.ObjVal() })
	if err := sol.TrySetVal(orig, 1); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("Try form should agree: %v", err)
	}
	if orig.Name() != "x1" {
		t.Fatal("original variable must survive FreeTransform")
	}
	if _, err := m.TryAddCons([]Variable{trans}, []float64{1}, 0, 1, "stale"); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("stale transformed handle as argument: %v", err)
	}
	// a fresh solve issues fresh handles
	if v := m.Solve().Vars()[0]; v.Name() == "" {
		t.Fatal("handles from the new transform must work")
	}
}

type keepingHeur struct {
	model *Model
	v     *Variable
}

func (h *keepingHeur) Execute(model Model, _ HeurTiming, _ bool) HeurResult {
	if h.model == nil {
		m := model
		v := model.Vars()[0]
		h.model, h.v = &m, &v
		_ = v.SolVal() // legal while solving; must not panic
	}
	return HeurResultDidNotRun
}

func TestCallbackHandlesDieWithModel(t *testing.T) {
	m := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp"))
	h := &keepingHeur{}
	m.Add(NewHeur(h).Name("keeper"))
	m.Solve()
	if h.model == nil {
		t.Fatal("heuristic never ran")
	}
	if err := h.model.TryFree(); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("callback model must not be freeable: %v", err)
	}
	if _, err := h.model.TryStatus(); err != nil {
		t.Fatalf("callback model is usable while the owner lives: %v", err)
	}
	m.Free()
	if _, err := h.model.TryStatus(); asError(t, err).Stage != StageFree {
		t.Fatal("callback model must die with its owner")
	}
	expectErrorPanic(t, "callback variable after Free", RetcodeInvalidCall, func() { h.v.Name() })
	// an unrelated new model must not resurrect the stale handle
	other := createTestModel(t)
	defer other.Free()
	if _, err := other.TryAddCons([]Variable{*h.v}, []float64{1}, 0, 1, "z"); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("stale callback handle into a new model: %v", err)
	}
}

func TestPluginWrappersOnFreedModel(t *testing.T) {
	m := NewModel().HideOutput().IncludeDefaultPlugins()
	hs := m.Heurs()
	sep, _ := m.FindSeparator("gomory")
	m.Free()
	expectErrorPanic(t, "HeurPlugin.Name", RetcodeInvalidCall, func() { hs[0].Name() })
	expectErrorPanic(t, "HeurPlugin.SetFreq", RetcodeInvalidCall, func() { hs[0].SetFreq(1) })
	expectErrorPanic(t, "SeparatorPlugin.Freq", RetcodeInvalidCall, func() { sep.Freq() })
	expectErrorPanic(t, "zero HeurPlugin", RetcodeInvalidData, func() { (HeurPlugin{}).Name() })
}

type childrenProbe struct {
	nonFocus atomic.Int32
	solVal   atomic.Int32
}

func (p *childrenProbe) Execute(model Model, _ HeurTiming, _ bool) HeurResult {
	focus := model.FocusNode()
	if parent, ok := focus.Parent(); ok {
		if _, err := parent.TryChildren(); errors.Is(err, RetcodeInvalidData) {
			p.nonFocus.Add(1)
		}
	}
	for _, v := range model.Vars() {
		_ = v.SolVal()
		if col, ok := v.Col(); ok {
			col.Redcost() // must never abort, LP or no LP
		}
		v.Redcost()
	}
	p.solVal.Add(1)
	return HeurResultDidNotRun
}

func TestChildrenOnlyForFocusNodeAndSolvingGetters(t *testing.T) {
	probe := &childrenProbe{}
	m := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
	m.Add(NewHeur(probe).Name("probe").Timing(HeurTimingBeforeNode).Freq(1))
	m, _ = m.SetIntParam("lp/solvefreq", 0) // no node LPs below the root: exercises the no-LP Redcost path
	m, _ = m.SetLongintParam("limits/nodes", 20)
	m.Solve()
	if probe.solVal.Load() == 0 || probe.nonFocus.Load() == 0 {
		t.Fatalf("probe ran %d times, non-focus children rejected %d times", probe.solVal.Load(), probe.nonFocus.Load())
	}
}

func TestModelsAreCollectable(t *testing.T) {
	collectable := func(name string, mk func() *Scip) {
		t.Helper()
		wp := weak.Make(mk())
		for i := 0; i < 5 && wp.Value() != nil; i++ {
			runtime.GC()
		}
		if wp.Value() != nil {
			t.Fatalf("%s: a dropped model must be collectable; a registry is holding it", name)
		}
	}
	collectable("plain", func() *Scip { return createTestModel(t).Solve().scip })
	// a plugin that keeps callback handles sits in the plugin registry, which
	// must not root the model through those handles
	collectable("plugin keeps callback handles", func() *Scip {
		m := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp"))
		h := &keepingHeur{}
		m.Add(NewHeur(h).Name("keeper"))
		m.Solve()
		if h.model == nil {
			t.Fatal("heuristic never ran")
		}
		return m.scip
	})
}

func TestSubSCIPHandlesRejectedByParent(t *testing.T) {
	parent := createTestModel(t)
	defer parent.Free()
	other := createTestModel(t) // stands in for a sub-SCIP copy of parent
	defer other.Free()
	setCopyParent(other.scip.raw, parent.scip.raw)
	defer forgetCopy(other.scip.raw)
	worker := weakScip(other.scip.raw) // what a Copyable plugin's callback sees
	if worker.owner.Value() != parent.scip {
		t.Fatal("a sub-SCIP wrapper must resolve to the parent instance")
	}
	v := worker.newVar(other.Vars()[0].raw)
	if v.Name() == "" {
		t.Fatal("the worker handle is alive while the copy is")
	}
	if _, err := parent.TryAddCons([]Variable{v}, []float64{1}, 0, 1, "w"); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("a sub-SCIP handle must not enter the parent: %v", err)
	}
	forgetCopy(other.scip.raw) // SCIP freed the copy
	expectErrorPanic(t, "handle after the copy is freed", RetcodeInvalidCall, func() { v.Name() })
	setCopyParent(other.scip.raw, parent.scip.raw) // a later copy reuses the address
	expectErrorPanic(t, "handle after the address is reused", RetcodeInvalidCall, func() { v.Name() })
	if fresh := weakScip(other.scip.raw).newVar(other.Vars()[0].raw); fresh.Name() == "" {
		t.Fatal("a wrapper minted in the new incarnation is alive")
	}
}

type nodeKeeper struct{ n *Node }

func (k *nodeKeeper) Execute(model Model, _ HeurTiming, _ bool) HeurResult {
	if k.n == nil {
		n := model.FocusNode()
		k.n = &n
	}
	return HeurResultDidNotRun
}

func TestNodeChildrenAfterSolve(t *testing.T) {
	k := &nodeKeeper{}
	m := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp"))
	defer m.Free()
	m.Add(NewHeur(k).Name("keeper"))
	solved := m.Solve()
	if k.n == nil || solved.Stage() != StageSolved {
		t.Fatalf("keeper=%v stage=%v", k.n, solved.Stage())
	}
	if k.n.NChildren() != 0 { // no focus node in Solved: SCIPgetFocusNode must not be called
		t.Fatal("children after solving")
	}
	if c, err := k.n.TryChildren(); c != nil || err != nil {
		t.Fatalf("TryChildren after solving: %v %v", c, err)
	}
}

func TestCreateProbInvalidatesOriginalHandles(t *testing.T) {
	m := createTestModel(t)
	defer m.Free()
	x := m.Vars()[0]
	m.CreateProb("replacement")
	expectErrorPanic(t, "original var after CreateProb", RetcodeInvalidCall, func() { x.Name() })
	if _, err := m.TryAddCons([]Variable{x}, []float64{1}, 0, 1, "stale"); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("stale original handle as argument: %v", err)
	}
	if x.safeName() != "<Variable belongs to a problem that was freed>" {
		t.Fatalf("safeName must not panic: %q", x.safeName())
	}
	if (Variable{}).safeName() != "<zero Variable>" {
		t.Fatal("safeName on zero")
	}
	y := m.AddVar(0, 1, 1, "y", VarTypeBinary)
	if y.Name() != "y" {
		t.Fatal("handles of the new problem must work")
	}
}

func TestBacktrackToCurrentDepthIsNoop(t *testing.T) {
	// exercised through the prober only while solving, so drive it from a heuristic
	var checked atomic.Bool
	probe := heurFunc(func(model Model, _ HeurTiming, _ bool) HeurResult {
		if checked.Load() {
			return HeurResultDidNotRun
		}
		p, err := model.TryStartProbing()
		if err != nil {
			return HeurResultDidNotRun
		}
		defer p.End()
		p.NewNode()
		if err := p.TryBacktrack(p.Depth()); err != nil {
			panic(err)
		}
		if err := p.TryBacktrack(p.Depth() + 1); !errors.Is(err, RetcodeInvalidData) {
			panic("depth beyond current must be rejected")
		}
		checked.Store(true)
		return HeurResultDidNotRun
	})
	m := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.lp"))
	defer m.Free()
	m.Add(NewHeur(probe).Name("bt"))
	m.Solve()
	if !checked.Load() {
		t.Fatal("probing never ran")
	}
}

// heurFunc adapts a function to the Heuristic interface.
type heurFunc func(Model, HeurTiming, bool) HeurResult

func (f heurFunc) Execute(m Model, t HeurTiming, inf bool) HeurResult { return f(m, t, inf) }
