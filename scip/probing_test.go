package scip

import (
	"sync/atomic"
	"testing"
)

type probingTester struct{ t *testing.T }

func (probingTester) GetEventMask() EventMask { return EventMaskNodeSolved }

func (h probingTester) Execute(model Model, _ SCIPEventhdlr, _ Event) {
	prober := model.StartProbing()
	if prober.IsObjChanged() {
		h.t.Error("obj changed before any modification")
	}

	for _, v := range model.Vars() {
		prober.ChgVarObj(v, 0)
	}
	if !prober.IsObjChanged() {
		h.t.Error("obj not changed")
	}

	if _, err := prober.SolveLp(-1); err != nil {
		h.t.Errorf("solve_lp: %v", err)
	}
	if model.LpObjVal() > 1e-6 || model.LpObjVal() < -1e-6 {
		h.t.Errorf("LP obj %v != 0", model.LpObjVal())
	}

	prober.End()

	if InProbing(model) {
		h.t.Error("still probing after End")
	}
}

func TestProber(t *testing.T) {
	model := mustRead(t, NewModel().
		IncludeDefaultPlugins(), testFile("simple.mps"))
	model = model.HideOutput().
		SetPresolving(ParamSettingOff).
		SetSeparating(ParamSettingOff).
		SetHeuristics(ParamSettingOff)
	model, err := SetParam(model, "branching/pscost/priority", int32(100000))
	if err != nil {
		t.Fatal(err)
	}
	model.Add(NewEventhdlr(probingTester{t: t}))
	model.Solve()
}

type probingAddRowTester struct {
	checked *atomic.Bool
	t       *testing.T
}

func (h probingAddRowTester) GetEventMask() EventMask { return EventMaskNodeSolved }

func (h probingAddRowTester) Execute(model Model, _ SCIPEventhdlr, _ Event) {
	if h.checked.Load() {
		return
	}
	prober := model.StartProbing()
	// Since SCIP 10 the node LP is freed before NODE_SOLVED, so solve the
	// probing LP first; with no probing changes this is the node LP.
	if _, err := prober.SolveLp(-1); err != nil {
		h.t.Errorf("solve_lp: %v", err)
	}
	if obj := model.LpObjVal(); obj > -25 {
		h.t.Errorf("LP obj %v > -25", obj)
	}
	row := NewRow().Eq(-1).AddTo(model) // unsatisfiable row
	prober.AddRow(row)
	if _, err := prober.SolveLp(-1); err != nil {
		h.t.Errorf("solve_lp: %v", err)
	}
	if obj := model.LpObjVal(); obj < 1e15 && obj > -1e15 {
		h.t.Errorf("LP obj %v, expected infeasibility", obj)
	}
	prober.End()
	h.checked.Store(true)
}

func TestProbingAddRow(t *testing.T) {
	checked := &atomic.Bool{}
	model := mustRead(t, NewModel().
		IncludeDefaultPlugins(), testFile("simple.mps"))
	model = model.HideOutput().
		SetPresolving(ParamSettingOff).
		SetSeparating(ParamSettingOff).
		SetHeuristics(ParamSettingOff)
	model, err := SetParam(model, "branching/pscost/priority", int32(100000))
	if err != nil {
		t.Fatal(err)
	}
	model.Add(NewEventhdlr(probingAddRowTester{checked: checked, t: t}))
	model.Solve()
	if !checked.Load() {
		t.Fatal("probing assertions never ran")
	}
}
