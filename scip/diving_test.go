package scip

import (
	"sync/atomic"
	"testing"
)

type divingTester struct {
	checked *atomic.Bool
	t       *testing.T
}

func (h divingTester) GetEventMask() EventMask { return EventMaskNodeSolved }

func (h divingTester) Execute(model Model, _ EventhdlrPlugin, _ Event) {
	if h.checked.Load() {
		return
	}

	diver := model.StartDiving()

	for _, v := range model.Vars() {
		diver.ChgVarObj(v, 0)
		if diver.VarObj(v) != 0 {
			h.t.Error("dive obj not changed")
		}
	}

	ok, err := diver.SolveLp(-1)
	if err != nil || !ok {
		h.t.Errorf("solve_lp: ok=%v err=%v", ok, err)
	}
	if model.LpStatus() != LPStatusOptimal {
		h.t.Error("LP status != Optimal")
	}
	if model.LpObjVal() > 1e-6 || model.LpObjVal() < -1e-6 {
		h.t.Errorf("LP obj %v != 0", model.LpObjVal())
	}

	currentNode := model.FocusNode()
	if diver.LastDiveNode() != currentNode.Number() {
		h.t.Error("last dive node mismatch")
	}

	diver.AddRow(NewRow().Eq(-1).AddTo(model)) // unsatisfiable row
	if _, err := diver.SolveLp(-1); err != nil {
		h.t.Errorf("solve_lp: %v", err)
	}
	if model.LpStatus() != LPStatusInfeasible {
		h.t.Error("LP status != Infeasible")
	}

	diver.End()

	if InDive(model) {
		h.t.Error("still in dive after End")
	}
	h.checked.Store(true)
}

func TestDiver(t *testing.T) {
	checked := &atomic.Bool{}
	model := mustRead(t, NewModel().
		IncludeDefaultPlugins(), testFile("simple.mps"))
	model = model.HideOutput().
		SetPresolving(ParamSettingOff).
		SetSeparating(ParamSettingOff).
		SetHeuristics(ParamSettingOff)
	model.Add(NewEventhdlr(divingTester{checked: checked, t: t}))
	model.Solve()
	if !checked.Load() {
		t.Fatal("diving assertions never ran")
	}
}
