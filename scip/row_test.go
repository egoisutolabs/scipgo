package scip

import (
	"sync/atomic"
	"testing"
)

func rowTestModel(t *testing.T) Model {
	t.Helper()
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize().
		SetPresolving(ParamSettingOff).
		SetSeparating(ParamSettingOff).
		SetHeuristics(ParamSettingOff)

	x1 := model.AddVar(0, Infinity, 3, "x1", VarTypeContinuous)
	x2 := model.AddVar(0, Infinity, 4, "x2", VarTypeContinuous)
	model.AddCons([]Variable{x1, x2}, []float64{2, 1}, NegInfinity, 100, "c1")
	model.AddCons([]Variable{x1, x2}, []float64{1, 2}, NegInfinity, 80, "c2")
	return model
}

type rowTesterEventHandler struct {
	checked *atomic.Bool
	t       *testing.T
}

func (h rowTesterEventHandler) GetEventMask() EventMask { return EventMaskFirstLpSolved }

func (h rowTesterEventHandler) Execute(model Model, _ EventhdlrPlugin, _ Event) {
	// Since SCIP 10 this event also fires for the initial (empty) LP, where
	// the constraint has no LP row yet; only inspect once it does.
	firstCons := model.Conss()[0]
	row, ok := firstCons.Row()
	if !ok {
		return
	}

	if row.NNonZeroes() != 2 {
		h.t.Error("n_non_zeroes != 2")
	}
	if row.Lhs() > -Infinity/2 {
		h.t.Error("lhs should be -inf")
	}
	if row.Rhs() != 100 {
		h.t.Error("rhs != 100")
	}
	if row.Index() != 0 {
		h.t.Error("index != 0")
	}
	if row.IsModifiable() || row.IsRemovable() || row.IsLocal() || row.IsIntegral() {
		h.t.Error("unexpected row flags")
	}
	if _, ok := row.Constraint(); !ok {
		h.t.Error("row has no constraint")
	}
	if row.BasisStatus() != BasisStatusUpper {
		h.t.Error("basis status != Upper")
	}
	if row.OriginType() != RowOriginConstraint {
		h.t.Error("origin != Constraint")
	}
	if row.IsInGlobalCutPool() || !row.IsInLP() {
		h.t.Error("unexpected row LP state")
	}
	if pos, ok := row.LpPosition(); !ok || pos != 0 {
		h.t.Error("lp position != 0")
	}
	if row.Depth() != 0 {
		h.t.Error("depth != 0")
	}
	if row.ActiveLPCount() != 1 {
		h.t.Error("active LP count != 1")
	}
	if row.NLpSinceCreate() != 1 {
		h.t.Error("n LP since create != 1")
	}
	if row.Rank() != 0 {
		h.t.Error("rank != 0")
	}
	row.SetRank(1)
	if row.Rank() != 1 {
		h.t.Error("rank not updated")
	}
	if row.Name() != "c1" {
		h.t.Error("row name != c1")
	}
	if row.Age() != 0 {
		h.t.Error("age != 0")
	}
	// Dual value of c1 is 2/3, negated for the internally-minimized sense.
	if d := row.Dual() - (-2.0 / 3.0); d > 1e-9 || d < -1e-9 {
		h.t.Errorf("dual %v", row.Dual())
	}
	// The LP is feasible, so there is no Farkas certificate.
	if row.FarkasDual() < Infinity/2 {
		h.t.Error("expected +inf farkas dual")
	}
	cols := row.Cols()
	if len(cols) != 2 || cols[0].Index() != 0 {
		h.t.Error("unexpected columns")
	}
	h.checked.Store(true)
}

func TestRow(t *testing.T) {
	checked := &atomic.Bool{}
	model := rowTestModel(t)
	model.Add(NewEventhdlr(rowTesterEventHandler{checked: checked, t: t}).Name("RowTesterEventHandler"))
	model.Solve()
	if !checked.Load() {
		t.Fatal("row assertions never ran")
	}
}
