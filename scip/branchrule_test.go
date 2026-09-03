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

func (r *firstChoosingBranchingRule) Execute(_ Model, branchrule SCIPBranchRule, candidates []BranchingCandidate) BranchingResult {
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

func (cuttingOffBranchingRule) Execute(Model, SCIPBranchRule, []BranchingCandidate) BranchingResult {
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

func (r firstBranchingRule) Execute(model Model, _ SCIPBranchRule, candidates []BranchingCandidate) BranchingResult {
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

func (r customBranchingRule) Execute(model Model, _ SCIPBranchRule, _ []BranchingCandidate) BranchingResult {
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

func (r highestBoundBranchRule) Execute(model Model, _ SCIPBranchRule, candidates []BranchingCandidate) BranchingResult {
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
