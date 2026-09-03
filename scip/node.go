package scip

/*
#include "helpers.h"
*/
import "C"

import "unsafe"

// Node is a node in the branch-and-bound tree.
type Node struct {
	raw  *C.SCIP_NODE
	scip *Scip
}

// Inner returns the raw pointer to the underlying SCIP_NODE.
func (n Node) Inner() *C.SCIP_NODE { return n.raw }

// Number returns the number of the node.
func (n Node) Number() int { return int(C.SCIPnodeGetNumber(n.raw)) }

// Depth returns the depth of the node in the branch-and-bound tree.
func (n Node) Depth() int { return int(C.SCIPnodeGetDepth(n.raw)) }

// LowerBound returns the lower bound of the node.
func (n Node) LowerBound() float64 { return float64(C.SCIPnodeGetLowerbound(n.raw)) }

// Parent returns the parent of the node and false if the node is the root node.
func (n Node) Parent() (Node, bool) {
	parent := C.SCIPnodeGetParent(n.raw)
	if parent == nil {
		return Node{}, false
	}
	return Node{raw: parent, scip: n.scip}, true
}

// NChildren returns the number of children of the node.
func (n Node) NChildren() int { return int(C.SCIPgetNChildren(n.scip.raw)) }

// Children returns the children of the node.
func (n Node) Children() []Node {
	numChildren := n.NChildren()
	if numChildren == 0 {
		return nil
	}
	var childNodesPtr **C.SCIP_NODE
	mustOK(C.SCIPgetChildren(n.scip.raw, &childNodesPtr, nil))
	children := make([]Node, 0, numChildren)
	for i := 0; i < numChildren; i++ {
		children = append(children, Node{raw: cNodeAt(childNodesPtr, i), scip: n.scip})
	}
	return children
}

// cAt returns the i-th element of a C array, given a pointer to its first
// element. T is the element type (e.g. *C.SCIP_VAR for a SCIP_VAR** array).
func cAt[T any](p *T, i int) T {
	return *(*T)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i)*unsafe.Sizeof(*p)))
}

// cVarAt returns the i-th SCIP_VAR* in a C array of variable pointers.
func cVarAt(arr **C.SCIP_VAR, i int) *C.SCIP_VAR { return cAt(arr, i) }

// cConsAt returns the i-th SCIP_CONS* in a C array of constraint pointers.
func cConsAt(arr **C.SCIP_CONS, i int) *C.SCIP_CONS { return cAt(arr, i) }

// cNodeAt returns the i-th SCIP_NODE* in a C array of node pointers.
func cNodeAt(arr **C.SCIP_NODE, i int) *C.SCIP_NODE { return cAt(arr, i) }

// cSolAt returns the i-th SCIP_SOL* in a C array of solution pointers.
func cSolAt(arr **C.SCIP_SOL, i int) *C.SCIP_SOL { return cAt(arr, i) }

// cColAt returns the i-th SCIP_COL* in a C array of column pointers.
func cColAt(arr **C.SCIP_COL, i int) *C.SCIP_COL { return cAt(arr, i) }

// cDoubleAt returns the i-th double in a C array of doubles.
func cDoubleAt(arr *C.double, i int) float64 {
	return float64(cAt(arr, i))
}
