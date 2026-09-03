package scip

import "testing"

type nodeDataBranchRule struct{ t *testing.T }

func (r nodeDataBranchRule) Execute(model Model, _ SCIPBranchRule, candidates []BranchingCandidate) BranchingResult {
	node := model.FocusNode()
	if node.Number() == 1 {
		if node.Depth() != 0 {
			r.t.Error("root depth != 0")
		}
		if node.LowerBound() >= 6777.0 {
			r.t.Error("root lower bound too high")
		}
		if _, ok := node.Parent(); ok {
			r.t.Error("root has a parent")
		}
	} else {
		if node.Depth() <= 0 {
			r.t.Error("non-root depth <= 0")
		}
		if node.LowerBound() > 6777.0 {
			r.t.Error("node lower bound too high")
		}
		if _, ok := node.Parent(); !ok {
			r.t.Error("non-root has no parent")
		}
	}
	if len(node.Children()) != 0 {
		r.t.Error("node has children before branching")
	}
	return BranchOn(candidates[0])
}

func TestNodeAfterSolving(t *testing.T) {
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 3)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewBranchRule(nodeDataBranchRule{t: t}))
	model.Solve()
}

type focusNodeHandler struct{ t *testing.T }

func (focusNodeHandler) GetEventMask() EventMask { return EventMaskNodeBranched }

func (h focusNodeHandler) Execute(model Model, _ SCIPEventhdlr, _ Event) {
	currentNode := model.FocusNode()
	if len(currentNode.Children()) == 0 {
		h.t.Error("node has no children after branching")
	}
}

func TestNodeAfterBranching(t *testing.T) {
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 3)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewEventhdlr(focusNodeHandler{t: t}))
	model.Solve()
}
