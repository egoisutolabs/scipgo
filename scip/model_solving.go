package scip

/*
#include "helpers.h"
*/
import "C"

// CreateSol creates a new solution initialized to zero.
func (m Model) CreateSol() Solution {
	solPtr, err := m.scip.createSol(false)
	if err != nil {
		panic("Failed to create solution")
	}
	return Solution{raw: solPtr, scip: m.scip}
}

// CreateOrigSol creates a new solution in the original space.
func (m Model) CreateOrigSol() Solution {
	solPtr, err := m.scip.createSol(true)
	if err != nil {
		panic("Failed to create solution in original space")
	}
	return Solution{raw: solPtr, scip: m.scip}
}

// CreatePartialSol creates a new partial solution: variables left unset are
// UNKNOWN rather than zero, and are filled in by the completesol heuristic
// when the solution is added via AddSol. Useful as a MIP-start that fixes
// only some variables and lets the solver complete the rest.
func (m Model) CreatePartialSol() Solution {
	solPtr, err := m.scip.createPartialSol()
	if err != nil {
		panic("Failed to create partial solution in state ProblemCreated")
	}
	return Solution{raw: solPtr, scip: m.scip}
}

// AddSol adds a solution to the model, consuming it: sol is invalid
// afterwards. Returns an error if the solution was not stored (e.g. it is
// infeasible).
func (m Model) AddSol(sol *Solution) error {
	stored, err := m.scip.addSol(sol)
	rethrowPanics(m.scip.raw)
	if err != nil {
		return err
	}
	if !stored {
		return SolErrorInfeasible
	}
	return nil
}

// BestSol returns the best solution for the optimization model, if one exists.
func (m Model) BestSol() (Solution, bool) { return bestSolOf(m.scip) }

// NSols returns the number of solutions found by the optimization model.
func (m Model) NSols() int { return m.scip.nSols() }

// GetSols returns all solutions stored in the solution storage.
func (m Model) GetSols() []Solution { return scipSols(m.scip) }

// ObjVal returns the objective value of the best solution found.
func (m Model) ObjVal() float64 { return m.scip.objVal() }

// BestBound returns the best bound (dualbound) proven so far.
func (m Model) BestBound() float64 { return m.scip.bestBound() }

// NNodes returns the number of nodes explored by the optimization model.
func (m Model) NNodes() int { return m.scip.nNodes() }

// SolvingTime returns the total solving time of the optimization model.
func (m Model) SolvingTime() float64 { return m.scip.solvingTime() }

// NLpIterations returns the number of LP iterations performed.
func (m Model) NLpIterations() int { return m.scip.nLPIterations() }

// StatsJSON returns the solving statistics in JSON format.
func (m Model) StatsJSON() string {
	j, err := m.scip.statisticsJSON()
	if err != nil {
		panic("Failed to get statistics in JSON format")
	}
	return j
}

// WriteStatsJSON writes the solving statistics in JSON format to the given path.
func (m Model) WriteStatsJSON(path string) error { return m.scip.writeStatisticsJSON(path) }

// FocusNode returns the current node of the model.
func (m Model) FocusNode() Node {
	scipNode := m.scip.focusNode()
	if scipNode == nil {
		panic("Failed to get focus node")
	}
	return Node{raw: scipNode, scip: m.scip}
}

// CreateChild creates a new child node of the current node and returns it.
func (m Model) CreateChild() Node {
	nodePtr, err := m.scip.createChild()
	if err != nil {
		panic("Failed to create child node in state Solving")
	}
	return Node{raw: nodePtr, scip: m.scip}
}

func (m Model) wrapNode(ptr *C.SCIP_NODE) *Node {
	if ptr == nil {
		return nil
	}
	return &Node{raw: ptr, scip: m.scip}
}

// BestNode returns the best open node with respect to the active node
// selector, or nil if the tree is empty.
func (m Model) BestNode() *Node { return m.wrapNode(m.scip.bestNode()) }

// BestBoundNode returns the open node with the best (smallest) lower bound,
// or nil if the tree is empty.
func (m Model) BestBoundNode() *Node { return m.wrapNode(m.scip.bestBoundNode()) }

// BestLeaf returns the best leaf from the leaf queue with respect to the
// active node selector, or nil if it is empty.
func (m Model) BestLeaf() *Node { return m.wrapNode(m.scip.bestLeaf()) }

// BestChild returns the best child of the focus node with respect to the
// active node selector, or nil if there is none.
func (m Model) BestChild() *Node { return m.wrapNode(m.scip.bestChild()) }

// BestSibling returns the best sibling of the focus node with respect to the
// active node selector, or nil if there is none.
func (m Model) BestSibling() *Node { return m.wrapNode(m.scip.bestSibling()) }

// PrioChild returns the child of the focus node with the largest node
// selection priority, or nil if there is none.
func (m Model) PrioChild() *Node { return m.wrapNode(m.scip.prioChild()) }

// PrioSibling returns the sibling of the focus node with the largest node
// selection priority, or nil if there is none.
func (m Model) PrioSibling() *Node { return m.wrapNode(m.scip.prioSibling()) }

// Leaves returns the leaves of the branch-and-bound tree (the open nodes
// that are neither children nor siblings of the focus node).
func (m Model) Leaves() []Node { return m.wrapNodes(m.scip.leaves()) }

// Children returns the children of the focus node.
func (m Model) Children() []Node { return m.wrapNodes(m.scip.children()) }

// Siblings returns the siblings of the focus node.
func (m Model) Siblings() []Node { return m.wrapNodes(m.scip.siblings()) }

func (m Model) wrapNodes(ptrs []*C.SCIP_NODE) []Node {
	out := make([]Node, 0, len(ptrs))
	for _, p := range ptrs {
		out = append(out, Node{raw: p, scip: m.scip})
	}
	return out
}

// NodeGetNAddedConss returns the number of added constraints to the given node.
func (m Model) NodeGetNAddedConss(node *Node) int { return m.scip.nodeGetNAddedConss(*node) }

// VarInProb gets the variable in the current problem given its index, if it
// exists.
func (m Model) VarInProb(varProbID int) (Variable, bool) {
	v := varFromID(m.scip.raw, varProbID)
	if v == nil {
		return Variable{}, false
	}
	return Variable{raw: v, scip: m.scip}, true
}

// AddCut adds a new cut (row) to the model. Returns whether the row is
// infeasible from the local bounds.
func (m Model) AddCut(cut Row, forceCut bool) bool {
	infeasible, err := m.scip.addRow(cut, forceCut)
	if err != nil {
		panic("Failed to add row in state Solving")
	}
	return infeasible
}

// CurrentVal returns the value of a variable in the current LP/pseudo solution.
func (m Model) CurrentVal(v Variable) float64 {
	return float64(C.SCIPgetSolVal(m.scip.raw, nil, v.raw))
}

// StartProbing starts probing at the current node. The returned Prober must
// be ended with a call to its End method.
func (m Model) StartProbing() *Prober {
	mustOK(C.SCIPstartProbing(m.scip.raw))
	return &Prober{scip: m.scip}
}

// StartDiving starts diving at the current node. The returned Diver must be
// ended with a call to its End method.
func (m Model) StartDiving() *Diver {
	// Since SCIP 10, SCIPstartDive requires the current node's LP to be
	// constructed first; construct it on demand.
	if C.SCIPisLPConstructed(m.scip.raw) == 0 {
		var cutoff C.uint
		mustOK(C.SCIPconstructLP(m.scip.raw, &cutoff))
	}
	mustOK(C.SCIPstartDive(m.scip.raw))
	return &Diver{scip: m.scip}
}

// LpObjVal returns the objective value of the current LP relaxation.
func (m Model) LpObjVal() float64 { return m.scip.lpObjVal() }

// LpStatus returns the status of the current LP solve.
func (m Model) LpStatus() LPStatus { return m.scip.lpStatus() }

// SetUbNode changes the upper bound of the variable in a given node.
func (m Model) SetUbNode(node *Node, v Variable, ub float64) {
	mustOK(C.SCIPchgVarUbNode(m.scip.raw, node.raw, v.raw, C.double(ub)))
}

// SetLbNode changes the lower bound of the variable in a given node.
func (m Model) SetLbNode(node *Node, v Variable, lb float64) {
	mustOK(C.SCIPchgVarLbNode(m.scip.raw, node.raw, v.raw, C.double(lb)))
}

func scipSols(s *Scip) []Solution {
	raw := s.getSols()
	out := make([]Solution, 0, len(raw))
	for _, sol := range raw {
		out = append(out, Solution{raw: sol, scip: s})
	}
	return out
}

func bestSolOf(s *Scip) (Solution, bool) {
	raw := s.bestSol()
	if raw == nil {
		return Solution{}, false
	}
	return Solution{raw: raw, scip: s}, true
}
