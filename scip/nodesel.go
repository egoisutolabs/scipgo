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

// SCIPNodesel is a wrapper struct for the internal SCIP node selector.
type SCIPNodesel struct {
	raw *C.SCIP_NODESEL
}

// Inner returns the internal raw pointer of the node selector.
func (n SCIPNodesel) Inner() *C.SCIP_NODESEL { return n.raw }

// Name returns the name of the node selector.
func (n SCIPNodesel) Name() string { return goString(C.SCIPnodeselGetName(n.raw)) }

// Desc returns the description of the node selector.
func (n SCIPNodesel) Desc() string { return goString(C.SCIPnodeselGetDesc(n.raw)) }

// StdPriority returns the standard priority of the node selector.
func (n SCIPNodesel) StdPriority() int32 { return int32(C.SCIPnodeselGetStdPriority(n.raw)) }

// MemSavePriority returns the memory saving priority of the node selector.
func (n SCIPNodesel) MemSavePriority() int32 { return int32(C.SCIPnodeselGetMemsavePriority(n.raw)) }
