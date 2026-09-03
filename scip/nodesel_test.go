package scip

import (
	"sync"
	"testing"
)

type dfsNodeSel struct {
	mu          sync.Mutex
	selectCalls int
}

func (s *dfsNodeSel) Select(model Model) *Node {
	s.mu.Lock()
	s.selectCalls++
	s.mu.Unlock()
	// Prefer diving into a child, then a sibling, then the best leaf.
	if n := model.PrioChild(); n != nil {
		return n
	}
	if n := model.PrioSibling(); n != nil {
		return n
	}
	return model.BestLeaf()
}

func (s *dfsNodeSel) Comp(node1, node2 Node) int {
	// Deeper nodes first (depth-first search).
	switch {
	case node2.Depth() < node1.Depth():
		return -1
	case node2.Depth() > node1.Depth():
		return 1
	default:
		return 0
	}
}

func TestDfsNodeSelector(t *testing.T) {
	ns := &dfsNodeSel{}

	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 200)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewNodesel(ns).
		Name("DfsNodeSel").
		Desc("A depth-first-search node selector"))

	solved := model.Solve()
	if solved.Status() != StatusOptimal && solved.Status() != StatusNodeLimit {
		t.Fatalf("got status %v", solved.Status())
	}
	ns.mu.Lock()
	defer ns.mu.Unlock()
	// SCIP consults the node selector once per focused node, plus one final
	// call when no node is left.
	if ns.selectCalls != solved.NNodes()+1 {
		t.Fatalf("select calls %d, want %d", ns.selectCalls, solved.NNodes()+1)
	}
}

func TestFindNodesel(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.mps"))

	if _, ok := model.FindNodesel("bfs"); !ok {
		t.Fatal("default bfs node selector not found")
	}
	if _, ok := model.FindNodesel("does-not-exist"); ok {
		t.Fatal("non-existing node selector found")
	}
	if _, ok := model.FindNodesel("CustomNodeSel"); ok {
		t.Fatal("custom node selector found before inclusion")
	}

	model.Add(NewNodesel(internalSCIPNodeSelTester{}).
		Name("CustomNodeSel").
		Desc("A custom node selector"))

	found, ok := model.FindNodesel("CustomNodeSel")
	if !ok {
		t.Fatal("custom node selector not found after inclusion")
	}
	if found.Name() != "CustomNodeSel" || found.Desc() != "A custom node selector" {
		t.Fatal("wrong node selector metadata")
	}
	if found.StdPriority() != 1000000 || found.MemSavePriority() != 1000000 {
		t.Fatal("wrong node selector priorities")
	}
}

type internalSCIPNodeSelTester struct{}

func (internalSCIPNodeSelTester) Select(model Model) *Node { return model.BestNode() }

func (internalSCIPNodeSelTester) Comp(node1, node2 Node) int {
	switch {
	case node1.LowerBound() < node2.LowerBound():
		return -1
	case node1.LowerBound() > node2.LowerBound():
		return 1
	default:
		return 0
	}
}
