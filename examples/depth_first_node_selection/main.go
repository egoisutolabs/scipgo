// A node selector implementing a depth-first-search (DFS) strategy: it dives
// as deep as possible into the tree before backtracking by always preferring
// a child of the current node, then a sibling, and only then the best
// remaining leaf.
package main

import (
	"fmt"
	"os"

	"github.com/egoisutolabs/scipgo/scip"
)

type depthFirstNodeSel struct{}

func (depthFirstNodeSel) Select(model scip.Model) *scip.Node {
	if n := model.PrioChild(); n != nil {
		return n
	}
	if n := model.PrioSibling(); n != nil {
		return n
	}
	return model.BestLeaf()
}

func (depthFirstNodeSel) Comp(node1, node2 scip.Node) int {
	// Order deeper nodes first; break ties by the smaller lower bound.
	switch {
	case node2.Depth() != node1.Depth():
		if node2.Depth() < node1.Depth() {
			return -1
		}
		return 1
	case node1.LowerBound() < node2.LowerBound():
		return -1
	case node1.LowerBound() > node2.LowerBound():
		return 1
	default:
		return 0
	}
}

func main() {
	model, err := scip.NewModel().
		IncludeDefaultPlugins().
		ReadProb("../../data/test/p0201.mps")
	if err != nil {
		fmt.Println("Failed to read problem file:", err)
		os.Exit(1)
	}

	// The custom selector has the highest priority by default, so SCIP uses
	// it over the built-in node selectors.
	model.Add(scip.NewNodesel(depthFirstNodeSel{}).
		Name("DepthFirst").
		Desc("Depth-first-search node selector"))

	solved := model.Solve()
	if solved.Status() != scip.StatusOptimal {
		fmt.Println("unexpected status:", solved.Status())
		os.Exit(1)
	}
	fmt.Println("solved to optimality:", solved.ObjVal())
}
