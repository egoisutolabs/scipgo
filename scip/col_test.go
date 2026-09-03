package scip

import (
	"sync/atomic"
	"testing"
)

func colTestModel(t *testing.T) Model {
	t.Helper()
	return rowTestModel(t)
}

type colTesterEventHandler struct {
	checked *atomic.Bool
	t       *testing.T
}

func (h colTesterEventHandler) GetEventMask() EventMask { return EventMaskFirstLpSolved }

func (h colTesterEventHandler) Execute(model Model, _ SCIPEventhdlr, event Event) {
	if event.EventType() != EventMaskFirstLpSolved {
		h.t.Error("unexpected event type")
	}
	vars := model.Vars()
	col, ok := vars[0].Col()
	if !ok {
		return // initial empty LP: variables still loose
	}

	if col.Index() != 0 {
		h.t.Error("index != 0")
	}
	// SCIP stores maximizing objectives as negated minimization.
	if col.Obj() != -3 {
		h.t.Error("obj != -3")
	}
	if col.Lb() != 0 {
		h.t.Error("lb != 0")
	}
	// Infinite upper bound tightened to 50 by root propagation of c1.
	if col.Ub() != 50 || col.BestBound() != 50 {
		h.t.Error("ub/best bound != 50")
	}
	if col.PrimalSol() != 40 || col.MinPrimalSol() != 40 || col.MaxPrimalSol() != 40 {
		h.t.Error("wrong primal solution values")
	}
	if col.BasisStatus() != BasisStatusBasic {
		h.t.Error("basis status != Basic")
	}
	if idx, ok := col.VarProbindex(); !ok || idx != 0 {
		h.t.Error("wrong probindex")
	}
	if col.IsIntegral() || col.IsRemovable() {
		h.t.Error("unexpected col flags")
	}
	if pos, ok := col.LpPos(); !ok || pos != 0 {
		h.t.Error("lp pos != 0")
	}
	if d, ok := col.LpDepth(); !ok || d != 0 {
		h.t.Error("lp depth != 0")
	}
	if !col.IsInLP() {
		h.t.Error("col not in LP")
	}
	if col.NNonZeros() != 2 || col.NLpNonZeros() != 2 {
		h.t.Error("wrong non-zero counts")
	}
	vals := col.Vals()
	if len(vals) != 2 || vals[0] != 2 || vals[1] != 1 {
		h.t.Errorf("vals %v", vals)
	}
	if _, ok := col.StrongBranchingNode(); ok {
		h.t.Error("unexpected strong branching node")
	}
	if col.NStrongBranches() != 0 {
		h.t.Error("strong branches != 0")
	}
	if col.Age() != 0 {
		h.t.Error("age != 0")
	}
	if len(col.Rows()) != 2 {
		h.t.Error("rows != 2")
	}
	if v := col.Var(); !v.IsTransformed() || v.Name() != "t_x1" {
		h.t.Errorf("col var %q not transformed", v.Name())
	}
	h.checked.Store(true)
}

func TestCol(t *testing.T) {
	checked := &atomic.Bool{}
	model := colTestModel(t)
	model.Add(NewEventhdlr(colTesterEventHandler{checked: checked, t: t}).Name("ColTesterEventHandler"))
	model.Solve()
	if !checked.Load() {
		t.Fatal("column assertions never ran")
	}
}
