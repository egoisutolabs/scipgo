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
	gen  uint64 // transform generation at creation; see handleErr
}

func (s *Scip) newNode(raw *C.SCIP_NODE) Node {
	h := Node{raw: raw, scip: s}
	if raw != nil {
		h.gen = s.gen()
	}
	return h
}

// live panics with *Error unless the handle is usable; see handleErr.
func (h Node) live(op string) { mustLive(op, "Node", h.raw != nil, h.scip, h.gen, false) }

// Inner returns the raw pointer to the underlying SCIP_NODE.
func (n Node) Inner() *C.SCIP_NODE { return n.raw }

// Number returns the number of the node.
func (n Node) Number() int {
	n.live("Node.Number")
	return int(C.SCIPnodeGetNumber(n.raw))
}

// Depth returns the depth of the node in the branch-and-bound tree.
func (n Node) Depth() int {
	n.live("Node.Depth")
	return int(C.SCIPnodeGetDepth(n.raw))
}

// LowerBound returns the lower bound of the node.
func (n Node) LowerBound() float64 {
	n.live("Node.LowerBound")
	return float64(C.SCIPnodeGetLowerbound(n.raw))
}

// Parent returns the parent of the node and false if the node is the root node.
func (n Node) Parent() (Node, bool) {
	n.live("Node.Parent")
	parent := C.SCIPnodeGetParent(n.raw)
	if parent == nil {
		return Node{}, false
	}
	return n.scip.newNode(parent), true
}

// NChildren returns the number of children of the node.
func (n Node) NChildren() int {
	n.live("Node.NChildren")
	// SCIPgetNChildren reports the focus node's children only.
	if !stagesSolvingSolved.has(n.scip.stage()) || C.SCIPgetFocusNode(n.scip.raw) != n.raw {
		return 0
	}
	return int(C.SCIPgetNChildren(n.scip.raw))
}

// Children returns the children of the node.
func (n Node) Children() []Node {
	n.live("Node.Children")
	c, err := n.TryChildren()
	must(err)
	return c
}

// TryChildren is Children returning an error instead of panicking. SCIP only
// exposes the focus node's children, so any other node is rejected; outside
// the solving stages there are none and it returns nil, nil.
func (n Node) TryChildren() ([]Node, error) {
	m := Model{scip: n.scip}
	if err := m.checkHandle("Node.Children", "Node", n.raw != nil, n.scip, n.gen, false); err != nil {
		return nil, err
	}
	if !stagesSolvingSolved.has(n.scip.stage()) {
		return nil, nil
	}
	// SCIPgetChildren has no node argument: it lists the focus node's children.
	if C.SCIPgetFocusNode(n.scip.raw) != n.raw {
		return nil, m.invalid("Node.Children", RetcodeInvalidData, "children are only available for the focus node")
	}
	numChildren := int(C.SCIPgetNChildren(n.scip.raw))
	if numChildren == 0 {
		return nil, nil
	}
	var childNodesPtr **C.SCIP_NODE
	if err := m.call("Node.Children", C.SCIPgetChildren(n.scip.raw, &childNodesPtr, nil)); err != nil {
		return nil, err
	}
	children := make([]Node, 0, numChildren)
	for i := 0; i < numChildren; i++ {
		children = append(children, Node{raw: cNodeAt(childNodesPtr, i), scip: n.scip})
	}
	return children, nil
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
