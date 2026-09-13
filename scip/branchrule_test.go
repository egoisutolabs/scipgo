package scip

import (
	"sync"
	"testing"
)

type firstChoosingBranchingRule struct {
	mu      sync.Mutex
	chosen  *BranchingCandidate
	failNow func()
}

func (r *firstChoosingBranchingRule) Execute(_ Model, branchrule BranchRulePlugin, candidates []BranchingCandidate) BranchingResult {
	r.mu.Lock()
	c := candidates[0]
	r.chosen = &c
	r.mu.Unlock()
	if branchrule.Name() != "FirstChoosingBranchingRule" {
		r.failNow()
	}
	return BranchingResult{Kind: BranchingResultDidNotRun}
}

func TestChoosingFirstBranchingRule(t *testing.T) {
	r := &firstChoosingBranchingRule{failNow: func() { t.Error("wrong branchrule name") }}
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 2)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewBranchRule(r).Name("FirstChoosingBranchingRule"))
	solved := model.Solve()
	if solved.Status() != StatusNodeLimit {
		t.Fatalf("got status %v, want NodeLimit", solved.Status())
	}
}

type cuttingOffBranchingRule struct{}

func (cuttingOffBranchingRule) Execute(Model, BranchRulePlugin, []BranchingCandidate) BranchingResult {
	return BranchingResult{Kind: BranchingResultCutOff}
}

func TestCuttingOffBranchingRule(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
	model.Add(NewBranchRule(cuttingOffBranchingRule{}).MaxDepth(10))
	solved := model.Solve()
	if solved.NNodes() != 1 {
		t.Fatalf("nodes %d, want 1", solved.NNodes())
	}
}

type firstBranchingRule struct{ t *testing.T }

func (r firstBranchingRule) Execute(model Model, _ BranchRulePlugin, candidates []BranchingCandidate) BranchingResult {
	if model.NVars() < len(candidates) {
		r.t.Error("more branching candidates than variables")
	}
	return BranchOn(candidates[0])
}

func TestFirstBranchingRule(t *testing.T) {
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 2)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewBranchRule(firstBranchingRule{t: t}).Name("FirstBranchingRule").MaxDepth(1000))
	solved := model.Solve()
	if solved.NNodes() <= 1 {
		t.Fatalf("nodes %d, want > 1", solved.NNodes())
	}
}

type customBranchingRule struct{ t *testing.T }

func (r customBranchingRule) Execute(model Model, _ BranchRulePlugin, _ []BranchingCandidate) BranchingResult {
	child1 := model.CreateChild()
	child2 := model.CreateChild()

	vars := model.Vars()
	model.AddConsNode(&child1, NewCons().Eq(0).Coef(vars[0], 1).Coef(vars[1], -1))
	model.AddConsNode(&child2, NewCons().Eq(1).Coef(vars[0], 1).Coef(vars[1], 1))

	if model.NodeGetNAddedConss(&child1) != 1 {
		r.t.Error("child1 cons count != 1")
	}
	if model.NodeGetNAddedConss(&child2) != 1 {
		r.t.Error("child2 cons count != 1")
	}
	return BranchingResult{Kind: BranchingResultCustomBranching}
}

func TestCustomBranchingRule(t *testing.T) {
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 2)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewBranchRule(customBranchingRule{t: t}))
	model.AddVar(0, 1, 1, "x", VarTypeBinary)
	model.AddVar(0, 1, 1, "y", VarTypeBinary)
	solved := model.Solve()
	if solved.NNodes() <= 1 {
		t.Fatalf("nodes %d, want > 1", solved.NNodes())
	}
}

type highestBoundBranchRule struct{ t *testing.T }

func (r highestBoundBranchRule) Execute(model Model, _ BranchRulePlugin, candidates []BranchingCandidate) BranchingResult {
	maxBound := NegInfinity
	var maxCandidate *BranchingCandidate
	for _, cand := range candidates {
		v, ok := model.VarInProb(cand.VarProbID)
		if !ok {
			r.t.Error("candidate variable not in problem")
			continue
		}
		if bound := v.Ub(); bound > maxBound {
			maxBound = bound
			c := cand
			maxCandidate = &c
		}
	}
	if maxCandidate != nil {
		return BranchOn(*maxCandidate)
	}
	return BranchingResult{Kind: BranchingResultDidNotRun}
}

func TestHighestBoundBranchRule(t *testing.T) {
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 2)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewBranchRule(highestBoundBranchRule{t: t}))
	solved := model.Solve()
	if solved.NNodes() <= 1 {
		t.Fatalf("nodes %d, want > 1", solved.NNodes())
	}
}

func TestInternalScipBranchRule(t *testing.T) {
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 2)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewBranchRule(firstBranchingRule{t: t}).MaxDepth(1))
	model.Solve()
}

// negFracRule records the candidates it is offered.
type negFracRule struct{ seen []BranchingCandidate }

func (r *negFracRule) Execute(_ Model, _ BranchRulePlugin, cands []BranchingCandidate) BranchingResult {
	r.seen = append(r.seen, cands...)
	return BranchOn(cands[0])
}

// TestBranchingCandidateFracNegative checks Frac is SCIP's fractionality in
// [0, 1) rather than a signed remainder: x = -1.5 has Frac 0.5, not -0.5.
func TestBranchingCandidateFracNegative(t *testing.T) {
	model := MinimalModel().HideOutput().Minimize()
	defer model.Free()
	// Bound propagation on the linear constraint would round y's bounds to
	// integers and make the LP integral, so it is switched off: the LP then
	// sits at x = -10, y = -3.5 and the rule sees y with fractionality 0.5.
	if _, err := SetParam(model, "constraints/linear/propfreq", int32(-1)); err != nil {
		t.Fatal(err)
	}
	x := model.AddVar(-10, 10, 1, "x", VarTypeInteger)
	y := model.AddVar(-10, 10, 1, "y", VarTypeInteger)
	model.AddCons([]Variable{x, y}, []float64{1, -2}, -3, -3, "link") // x - 2y = -3
	rule := &negFracRule{}
	model.Add(NewBranchRule(rule).Name("negfrac"))
	model.Solve()
	if len(rule.seen) == 0 {
		t.Fatal("rule was not called")
	}
	c := rule.seen[0]
	if c.LpSolVal > -3.4 || c.LpSolVal < -3.6 {
		t.Fatalf("LpSolVal = %v, want -3.5", c.LpSolVal)
	}
	if c.Frac < 0.49 || c.Frac > 0.51 {
		t.Fatalf("Frac = %v, want 0.5", c.Frac)
	}
	if model.Status() != StatusOptimal || model.ObjVal() != -12 { // x = -9, y = -3
		t.Fatalf("status %v obj %v", model.Status(), model.ObjVal())
	}
}
