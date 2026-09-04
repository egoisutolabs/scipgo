package scip

/*
#include "helpers.h"
*/
import "C"

// NodeSel is the interface for defining custom node selectors.
//
// A node selector decides which of the currently open (leaf) nodes of the
// branch-and-bound tree should be processed next:
//   - Select picks the next node to be processed, and
//   - Comp defines a total order on the open nodes (-1, 0, or +1), which SCIP
//     uses to keep its internal node queue sorted.
type NodeSel interface {
	// Select selects the next node to be processed. Return nil to let SCIP
	// fall back to the node with the best Comp ranking.
	Select(model Model) *Node
	// Comp compares two nodes: return -1 if node1 should be processed before
	// node2, +1 if node2 should be processed before node1, and 0 if both
	// nodes are considered equally good.
	Comp(node1, node2 Node) int
}

// NodeselPlugin is a wrapper struct for the internal SCIP node selector.
type NodeselPlugin struct {
	raw  *C.SCIP_NODESEL
	scip *Scip // keeps the owning instance alive and identifies it
}

// live panics with *Error unless the wrapper is usable; see handleErr.
func (h NodeselPlugin) live(op string) { mustLive(op, "NodeselPlugin", h.raw != nil, h.scip, 0, true) }

// Inner returns the internal raw pointer of the node selector.
func (n NodeselPlugin) Inner() *C.SCIP_NODESEL { return n.raw }

// Name returns the name of the node selector.
func (n NodeselPlugin) Name() string {
	n.live("NodeselPlugin.Name")
	return goString(C.SCIPnodeselGetName(n.raw))
}

// Desc returns the description of the node selector.
func (n NodeselPlugin) Desc() string {
	n.live("NodeselPlugin.Desc")
	return goString(C.SCIPnodeselGetDesc(n.raw))
}

// StdPriority returns the standard priority of the node selector.
func (n NodeselPlugin) StdPriority() int32 {
	n.live("NodeselPlugin.StdPriority")
	return int32(C.SCIPnodeselGetStdPriority(n.raw))
}

// MemSavePriority returns the memory saving priority of the node selector.
func (n NodeselPlugin) MemSavePriority() int32 {
	n.live("NodeselPlugin.MemSavePriority")
	return int32(C.SCIPnodeselGetMemsavePriority(n.raw))
}
