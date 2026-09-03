package scip

import (
	"sync/atomic"
	"testing"
)

type allInfeasibleConshdlr struct{}

func (allInfeasibleConshdlr) Check(Model, SCIPConshdlr, Solution) bool { return false }

func (allInfeasibleConshdlr) Enforce(Model, SCIPConshdlr) ConshdlrResult {
	return ConshdlrResultCutOff
}

func TestAllInfConshdlr(t *testing.T) {
	model := DefaultModel()
	model.IncludeConshdlr("AllInfeasibleConshdlr", "All infeasible constraint handler", -1, -1, allInfeasibleConshdlr{})
	solved := model.Solve()
	if solved.Status() != StatusInfeasible {
		t.Fatalf("got status %v, want Infeasible", solved.Status())
	}
}

type countingConshdlr struct{ enfops, sepa, prop atomic.Int32 }

func (*countingConshdlr) Check(Model, SCIPConshdlr, Solution) bool { return true }

func (*countingConshdlr) Enforce(Model, SCIPConshdlr) ConshdlrResult {
	return ConshdlrResultFeasible
}

func (c *countingConshdlr) EnforcePseudo(_ Model, _ SCIPConshdlr, _, _ bool) ConshdlrResult {
	c.enfops.Add(1)
	return ConshdlrResultFeasible
}

func (c *countingConshdlr) SeparateLP(Model, SCIPConshdlr) SeparationResult {
	c.sepa.Add(1)
	return SeparationResultDidNotFind
}

func (c *countingConshdlr) Propagate(Model, SCIPConshdlr) PropResult {
	c.prop.Add(1)
	return PropResultDidNotFind
}

func TestConshdlrSepaAndProp(t *testing.T) {
	c := &countingConshdlr{}
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
	model.IncludeConshdlr("counting", "", -1, -1, c)
	model, _ = model.SetLongintParam("limits/nodes", 30)
	model.Solve()
	if c.sepa.Load() == 0 || c.prop.Load() == 0 {
		t.Fatalf("sepa=%d prop=%d, want both > 0", c.sepa.Load(), c.prop.Load())
	}
}

func TestConshdlrEnfops(t *testing.T) {
	c := &countingConshdlr{}
	model := createTestModel(t)
	model.IncludeConshdlr("counting", "", -1, -1, c)
	model, _ = model.SetIntParam("lp/solvefreq", -1) // never solve LPs: only pseudo solutions
	model, _ = model.SetLongintParam("limits/nodes", 50)
	model.Solve()
	if c.enfops.Load() == 0 {
		t.Fatal("EnforcePseudo never called")
	}
}
