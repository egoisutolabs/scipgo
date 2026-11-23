package scip

import (
	"errors"
	"sync/atomic"
	"testing"
)

type noopProp struct{}

func (noopProp) Propagate(Model, PropagatorPlugin) PropResult { return PropResultDidNotFind }

type capProp struct {
	t      *testing.T
	v      Variable
	ub     float64
	plugin PropagatorPlugin
	calls  int
}

func (c *capProp) Propagate(model Model, prop PropagatorPlugin) PropResult {
	c.calls++
	c.plugin = prop
	if c.t != nil && c.calls == 1 {
		if prop.Name() != "capx" || prop.Desc() != "x <= 2" {
			c.t.Errorf("name/desc %q %q", prop.Name(), prop.Desc())
		}
		if prop.Priority() != 100000 || prop.Freq() != 1 || prop.IsDelayed() || prop.Timing() != PropTimingBeforeLp {
			c.t.Errorf("settings priority %d freq %d delayed %v timing %v", prop.Priority(), prop.Freq(), prop.IsDelayed(), prop.Timing())
		}
		if prop.DoesPresolve() || prop.PresolPriority() != 100000 || prop.PresolTiming() != PresolTimingAlways {
			c.t.Errorf("presolve settings does %v priority %d timing %v", prop.DoesPresolve(), prop.PresolPriority(), prop.PresolTiming())
		}
	}
	infeas, tightened, err := model.TryTightenVarUb(c.v, c.ub)
	if err != nil {
		panic(err)
	}
	if infeas {
		return PropResultCutOff
	}
	if tightened {
		return PropResultReducedDom
	}
	return PropResultDidNotFind
}

func TestPropagatorTightensUpperBound(t *testing.T) {
	model := MinimalModel().Maximize()
	defer model.Free()
	// Different objectives, so symmetry does not copy x's bound onto y.
	// The inequality locks both variables upwards (dualfix would otherwise
	// fix an unconstrained maximization) and does not bind.
	x := model.AddVar(0, 10, 1, "x", VarTypeInteger)
	y := model.AddVar(0, 10, 3, "y", VarTypeInteger)
	model.AddCons([]Variable{x, y}, []float64{1, 1}, NegInfinity, 100, "c")
	cap := &capProp{t: t, v: x, ub: 2}
	model.Add(NewPropagator(cap).Name("capx").Desc("x <= 2"))

	solved := model.Solve()
	if solved.Status() != StatusOptimal || solved.ObjVal() != 32 {
		t.Fatalf("status %v obj %v, want Optimal 32", solved.Status(), solved.ObjVal())
	}
	if cap.calls == 0 || cap.plugin.NCalls() == 0 || cap.plugin.NDomredsFound() == 0 {
		t.Fatalf("calls %d plugin calls %d domreds %d", cap.calls, cap.plugin.NCalls(), cap.plugin.NDomredsFound())
	}
	cap.plugin.SetFreq(-1)
	if cap.plugin.Freq() != -1 {
		t.Fatalf("SetFreq got %d", cap.plugin.Freq())
	}
}

func TestPropagatorCutsOffInfeasibleBound(t *testing.T) {
	model := MinimalModel().Maximize()
	defer model.Free()
	x := model.AddVar(0, 10, 1, "x", VarTypeInteger)
	model.AddCons([]Variable{x}, []float64{1}, NegInfinity, 10, "c")
	model.Add(NewPropagator(&capProp{v: x, ub: -1}).Name("cutoff").Timing(PropTimingBeforeLp))
	if model.Solve().Status() != StatusInfeasible {
		t.Fatalf("got %v, want Infeasible", model.Status())
	}
}

type capPresol struct {
	v      Variable
	ub     float64
	plugin PropagatorPlugin
}

func (c *capPresol) Propagate(Model, PropagatorPlugin) PropResult { return PropResultDidNotFind }

func (c *capPresol) PresolveProp(model Model, prop PropagatorPlugin) PresolResult {
	c.plugin = prop
	// Presolve acts on the transformed variable. The original AddVar handle
	// is not the one whose bound SCIP solves with.
	v := c.v
	if tv, ok := c.v.Transformed(); ok {
		v = tv
	}
	infeas, tightened, err := model.TryTightenVarUb(v, c.ub)
	if err != nil {
		panic(err)
	}
	if infeas {
		return PresolResult{Status: PresolStatusCutoff}
	}
	if tightened {
		return PresolResult{Status: PresolStatusSuccess, NChgBds: 1}
	}
	return PresolResult{Status: PresolStatusDidNotFind}
}

func (c *capPresol) Copy() any { return &capPresol{v: c.v, ub: c.ub} }

func TestPropagatorPresolTightensBound(t *testing.T) {
	// Heuristics off: one that runs before presolving stores x = 10, and
	// that incumbent is still the best solution after the bound moves to 4.
	model := DefaultModel().HideOutput().SetHeuristics(ParamSettingOff).Maximize()
	defer model.Free()
	x := model.AddVar(0, 10, 1, "x", VarTypeInteger)
	// A constraint gives presolving a round to run in. It does not bind.
	model.AddCons([]Variable{x}, []float64{1}, NegInfinity, 100, "c")
	hook := &capPresol{v: x, ub: 4}
	// Above pseudoobj (6000000). Once a heuristic has a solution, pseudoobj
	// can prove the remainder cutoff and SCIP ends presolve before anything
	// of lower priority runs.
	model.Add(NewPropagator(hook).
		Name("presolcap").
		Desc("x <= 4 in presolve").
		Freq(-1). // propagation must not be what tightens the bound
		PresolPriority(10000000).
		PresolMaxRounds(5).
		PresolTiming(PresolTimingFast | PresolTimingMedium))

	solved := model.Solve()
	if solved.Status() != StatusOptimal || solved.ObjVal() != 4 {
		t.Fatalf("status %v obj %v, want Optimal 4", solved.Status(), solved.ObjVal())
	}
	if !hook.plugin.DoesPresolve() || hook.plugin.NPresolCalls() == 0 || hook.plugin.NChgBds() != 1 {
		t.Fatalf("does %v presol calls %d chgbds %d", hook.plugin.DoesPresolve(), hook.plugin.NPresolCalls(), hook.plugin.NChgBds())
	}
	if hook.plugin.PresolPriority() != 10000000 || hook.plugin.PresolTiming() != PresolTimingFast|PresolTimingMedium {
		t.Fatalf("presol priority %d timing %v", hook.plugin.PresolPriority(), hook.plugin.PresolTiming())
	}
	if hook.plugin.NCalls() != 0 {
		t.Fatalf("propagation ran %d times with freq -1", hook.plugin.NCalls())
	}
}

func TestPropagatorPresolCutsOffInfeasibleBound(t *testing.T) {
	model := DefaultModel().HideOutput().SetHeuristics(ParamSettingOff).Maximize()
	defer model.Free()
	x := model.AddVar(0, 10, 1, "x", VarTypeInteger)
	model.AddCons([]Variable{x}, []float64{1}, NegInfinity, 100, "c")
	model.Add(NewPropagator(&capPresol{v: x, ub: -1}).
		Name("presolcut").
		Freq(-1).
		PresolPriority(10000000))
	if model.Solve().Status() != StatusInfeasible {
		t.Fatalf("got %v, want Infeasible", model.Status())
	}
}

type countingProp struct{ calls *atomic.Int32 }

func (p countingProp) Copy() any { return p }

func (p countingProp) Propagate(Model, PropagatorPlugin) PropResult {
	p.calls.Add(1)
	return PropResultDidNotFind
}

type localProp struct{ calls *atomic.Int32 }

func (p *localProp) Propagate(Model, PropagatorPlugin) PropResult {
	p.calls.Add(1)
	return PropResultDidNotFind
}

func TestPropagatorCopy(t *testing.T) {
	copied := countingProp{calls: new(atomic.Int32)}
	local := &localProp{calls: new(atomic.Int32)}
	source := NewModel().HideOutput().IncludeDefaultPlugins()
	defer source.Free()
	source.Add(
		NewPropagator(copied).Name("copiedprop").Priority(1000000),
		NewPropagator(local).Name("localprop").Priority(1000000),
	)

	target := NewModel().HideOutput()
	defer target.Free()
	if _, err := source.scip.copyPluginsTo(target.scip); err != nil {
		t.Fatal(err)
	}
	target = target.CreateProb("copy").SetPresolving(ParamSettingOff).SetHeuristics(ParamSettingOff).Maximize()
	target.AddVar(0, 5, 1, "x", VarTypeInteger)
	target.Solve()
	if copied.calls.Load() == 0 {
		t.Fatal("Copyable propagator did not run in the copy")
	}
	if local.calls.Load() != 0 {
		t.Fatal("non-Copyable propagator ran in the copy")
	}
}

type panickingProp struct{}

func (panickingProp) Propagate(Model, PropagatorPlugin) PropResult { panic("prop boom") }

func TestPropagatorPanic(t *testing.T) {
	model := MinimalModel().Maximize()
	defer model.Free()
	model.AddVar(0, 1, 1, "x", VarTypeBinary)
	model.Add(NewPropagator(panickingProp{}).Name("boom"))
	_, err := model.TrySolve()
	var cp *CallbackPanic
	if !errors.As(err, &cp) || cp.Plugin != "propagator scip.panickingProp" || cp.Value != "prop boom" {
		t.Fatalf("got %T %v", err, err)
	}
}

type delayedPresol struct{}

func (delayedPresol) Propagate(Model, PropagatorPlugin) PropResult { return PropResultDidNotRun }

func (delayedPresol) PresolveProp(Model, PropagatorPlugin) PresolResult {
	return PresolResult{Status: PresolStatusDelayed}
}

func TestPropagatorPresolRejectsDelayed(t *testing.T) {
	model := DefaultModel().HideOutput().Maximize()
	defer model.Free()
	x := model.AddVar(0, 1, 1, "x", VarTypeBinary)
	model.AddCons([]Variable{x}, []float64{1}, NegInfinity, 1, "c")
	model.Add(NewPropagator(delayedPresol{}).Name("delayed").Freq(-1).PresolPriority(10000000))
	_, err := model.TrySolve()
	var cp *CallbackPanic
	if !errors.As(err, &cp) || cp.Value != "PresolProp returned Delayed, which SCIP 10 rejects from a propagator's presolve method" {
		t.Fatalf("got %T %v", err, err)
	}
}

type negativeCountPresol struct{}

func (negativeCountPresol) Propagate(Model, PropagatorPlugin) PropResult {
	return PropResultDidNotRun
}

func (negativeCountPresol) PresolveProp(Model, PropagatorPlugin) PresolResult {
	return PresolResult{Status: PresolStatusSuccess, NChgBds: -1}
}

func TestPropagatorPresolRejectsNegativeCount(t *testing.T) {
	model := DefaultModel().HideOutput().Maximize()
	defer model.Free()
	x := model.AddVar(0, 1, 1, "x", VarTypeBinary)
	model.AddCons([]Variable{x}, []float64{1}, NegInfinity, 1, "c")
	model.Add(NewPropagator(negativeCountPresol{}).Name("neg").Freq(-1).PresolPriority(10000000))
	_, err := model.TrySolve()
	var cp *CallbackPanic
	if !errors.As(err, &cp) {
		t.Fatalf("got %T %v", err, err)
	}
}

func TestTightenVarLbUb(t *testing.T) {
	model := DefaultModel().HideOutput().Maximize()
	defer model.Free()
	x := model.AddVar(0, 10, 1, "x", VarTypeInteger)

	infeas, tightened, err := model.TryTightenVarUb(x, 4)
	if err != nil || infeas || !tightened || x.Ub() != 4 {
		t.Fatalf("tighten ub: infeas %v tightened %v ub %v err %v", infeas, tightened, x.Ub(), err)
	}
	infeas, tightened = model.TightenVarUb(x, 9) // looser than 4
	if infeas || tightened || x.Ub() != 4 {
		t.Fatalf("looser ub: infeas %v tightened %v ub %v", infeas, tightened, x.Ub())
	}
	infeas, tightened, err = model.TryTightenVarLb(x, 12)
	if err != nil || !infeas || tightened {
		t.Fatalf("infeasible lb: infeas %v tightened %v err %v", infeas, tightened, err)
	}
	if x.Lb() != 0 {
		t.Fatalf("lb changed on an infeasible tightening: %v", x.Lb())
	}

	infeas, tightened, err = model.TryTightenVarLb(x, 3)
	if err != nil || infeas || !tightened || x.Lb() != 3 {
		t.Fatalf("tighten lb: infeas %v tightened %v lb %v err %v", infeas, tightened, x.Lb(), err)
	}
	solved := model.Solve()
	if solved.Status() != StatusOptimal || solved.ObjVal() != 4 {
		t.Fatalf("status %v obj %v, want Optimal 4", solved.Status(), solved.ObjVal())
	}
}

func TestTightenVarWrongStageAndLiveness(t *testing.T) {
	fresh := NewModel()
	defer fresh.Free()
	if _, _, err := fresh.TryTightenVarLb(Variable{}, 0); asError(t, err).Stage != StageInit || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("init: %v", err)
	}

	model := DefaultModel().HideOutput().Maximize()
	x := model.AddVar(0, 1, 1, "x", VarTypeBinary)
	other := DefaultModel().HideOutput()
	defer other.Free()
	if _, _, err := other.TryTightenVarUb(x, 0); asError(t, err).Detail != "Variable belongs to another model" {
		t.Fatalf("foreign: %v", err)
	}
	if _, _, err := model.TryTightenVarUb(Variable{}, 0); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("zero var: %v", err)
	}

	model.Solve()
	if _, _, err := model.TryTightenVarUb(x, 0); asError(t, err).Stage != StageSolved || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("solved: %v", err)
	}
	expectErrorPanic(t, "TightenVarUb solved", RetcodeInvalidCall, func() { model.TightenVarUb(x, 0) })

	model.Free()
	if _, _, err := model.TryTightenVarLb(x, 0); asError(t, err).Stage != StageFree || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("freed: %v", err)
	}
}

func TestIncludePropagatorGuards(t *testing.T) {
	model := DefaultModel().HideOutput()
	defer model.Free()
	if err := model.TryIncludePropagator("p", "", 1, 1, false, PropTimingBeforeLp, 0, -1, PresolTimingAlways, nil); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("nil: %v", err)
	}
	var typed *noopProp
	if err := model.TryIncludePropagator("p", "", 1, 1, false, PropTimingBeforeLp, 0, -1, PresolTimingAlways, typed); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("typed nil: %v", err)
	}
	model.IncludePropagator("same", "d", 1, 1, false, PropTimingBeforeLp, 0, -1, PresolTimingAlways, noopProp{})
	if err := model.TryIncludePropagator("same", "", 1, 1, false, PropTimingBeforeLp, 0, -1, PresolTimingAlways, noopProp{}); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("duplicate: %v", err)
	}
	model.Free()
	if err := NewPropagator(noopProp{}).Name("dead").TryAddTo(model); asError(t, err).Stage != StageFree {
		t.Fatalf("include on freed: %v", err)
	}
}

func TestPropagatorPluginDiesWithModel(t *testing.T) {
	model := MinimalModel().Maximize()
	cap := &capProp{v: model.AddVar(0, 1, 1, "x", VarTypeBinary), ub: 0}
	model.Add(NewPropagator(cap).Name("capx").Desc("x <= 2"))
	model.Solve()
	if cap.plugin.Name() != "capx" {
		t.Fatal("plugin not recorded")
	}
	model.Free()
	expectErrorPanic(t, "PropagatorPlugin.Name", RetcodeInvalidCall, func() { cap.plugin.Name() })
	expectErrorPanic(t, "zero PropagatorPlugin", RetcodeInvalidData, func() { (PropagatorPlugin{}).Priority() })
}

type diveTighten struct {
	err error
	ran bool
}

func (d *diveTighten) GetEventMask() EventMask { return EventMaskNodeSolved }

func (d *diveTighten) Execute(model Model, _ EventhdlrPlugin, _ Event) {
	if d.ran {
		return
	}
	d.ran = true
	div := model.StartDiving()
	defer div.End()
	_, _, d.err = model.TryTightenVarUb(model.Vars()[0], 0)
}

func TestTightenVarRejectedWhileDiving(t *testing.T) {
	probe := &diveTighten{}
	model := mustRead(t, NewModel().IncludeDefaultPlugins(), testFile("simple.mps"))
	defer model.Free()
	model = model.HideOutput().
		SetPresolving(ParamSettingOff).
		SetSeparating(ParamSettingOff).
		SetHeuristics(ParamSettingOff)
	model.Add(NewEventhdlr(probe))
	model.Solve()
	if !probe.ran {
		t.Fatal("node event never ran")
	}
	if asError(t, probe.err).Detail != "SCIP is in diving mode" || !errors.Is(probe.err, RetcodeInvalidCall) {
		t.Fatalf("tighten during dive: %v", probe.err)
	}
}
