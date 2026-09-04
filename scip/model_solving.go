package scip

/*
#include "helpers.h"
*/
import "C"

// ------------------------------------------------------------- solutions

func (m Model) sol(op string, raw *C.SCIP_SOL, err error) (Solution, error) {
	if err != nil {
		return Solution{}, m.wrap(op, err, "")
	}
	return m.scip.newSol(raw), nil
}

// TryCreateSol creates a new solution initialized to zero.
func (m Model) TryCreateSol() (Solution, error) {
	if err := m.guard("CreateSol"); err != nil {
		return Solution{}, err
	}
	raw, err := m.scip.createSol(false)
	return m.sol("CreateSol", raw, err)
}

// CreateSol creates a new solution initialized to zero. It panics on failure.
func (m Model) CreateSol() Solution {
	s, err := m.TryCreateSol()
	must(err)
	return s
}

// TryCreateOrigSol creates a new solution in the original space.
func (m Model) TryCreateOrigSol() (Solution, error) {
	if err := m.guard("CreateOrigSol"); err != nil {
		return Solution{}, err
	}
	raw, err := m.scip.createSol(true)
	return m.sol("CreateOrigSol", raw, err)
}

// CreateOrigSol creates a new solution in the original space. It panics on
// failure.
func (m Model) CreateOrigSol() Solution {
	s, err := m.TryCreateOrigSol()
	must(err)
	return s
}

// TryCreatePartialSol creates a new partial solution: variables left unset
// are UNKNOWN rather than zero, and are filled in by the completesol
// heuristic when the solution is added via AddSol. Useful as a MIP-start that
// fixes only some variables and lets the solver complete the rest.
func (m Model) TryCreatePartialSol() (Solution, error) {
	if err := m.guard("CreatePartialSol"); err != nil {
		return Solution{}, err
	}
	raw, err := m.scip.createPartialSol()
	return m.sol("CreatePartialSol", raw, err)
}

// CreatePartialSol creates a new partial solution; see TryCreatePartialSol.
// It panics on failure.
func (m Model) CreatePartialSol() Solution {
	s, err := m.TryCreatePartialSol()
	must(err)
	return s
}

// AddSol adds a solution to the model, consuming it: sol is invalid
// afterwards. It returns SolErrorInfeasible if the solution was not stored,
// a *CallbackPanic if a constraint handler panicked while checking it, or an
// *Error for a SCIP failure.
func (m Model) AddSol(sol *Solution) error {
	if err := m.guard("AddSol"); err != nil {
		return err
	}
	if sol == nil || sol.raw == nil {
		return m.invalid("AddSol", RetcodeInvalidData, "nil or consumed solution")
	}
	if err := m.checkHandle("AddSol", "Solution", true, sol.scip, sol.gen, sol.orig); err != nil {
		return err
	}
	stored, err := m.scip.addSol(sol)
	if cp := callbackError(m.scip.raw); cp != nil {
		return cp
	}
	if err != nil {
		return m.wrap("AddSol", err, "")
	}
	if !stored {
		return SolErrorInfeasible
	}
	return nil
}

// TryBestSol returns the best solution, and whether one exists.
func (m Model) TryBestSol() (Solution, bool, error) {
	if err := m.query("BestSol", stagesBestSol); err != nil {
		return Solution{}, false, err
	}
	s, ok := bestSolOf(m.scip)
	return s, ok, nil
}

// BestSol returns the best solution for the optimization model, if one exists.
func (m Model) BestSol() (Solution, bool) {
	s, ok, err := m.TryBestSol()
	must(err)
	return s, ok
}

// NSols returns the number of solutions found by the optimization model.
func (m Model) NSols() int {
	must(m.query("NSols", stagesTrans))
	return m.scip.nSols()
}

// GetSols returns all solutions stored in the solution storage.
func (m Model) GetSols() []Solution {
	must(m.query("GetSols", stagesTrans))
	return scipSols(m.scip)
}

// ------------------------------------------------------------- statistics

// TryObjVal returns the objective value of the best solution found (the
// primal bound); SCIP only permits this once the problem is transformed.
func (m Model) TryObjVal() (float64, error) {
	if err := m.query("ObjVal", stagesTransformed); err != nil {
		return 0, err
	}
	return m.scip.objVal(), nil
}

// ObjVal returns the objective value of the best solution found.
func (m Model) ObjVal() float64 {
	v, err := m.TryObjVal()
	must(err)
	return v
}

// TryBestBound returns the best bound (dual bound) proven so far.
func (m Model) TryBestBound() (float64, error) {
	if err := m.query("BestBound", stagesTransformed); err != nil {
		return 0, err
	}
	return m.scip.bestBound(), nil
}

// BestBound returns the best bound (dualbound) proven so far.
func (m Model) BestBound() float64 {
	v, err := m.TryBestBound()
	must(err)
	return v
}

// TryNNodes returns the number of nodes explored.
func (m Model) TryNNodes() (int, error) {
	if err := m.query("NNodes", stagesOrig); err != nil {
		return 0, err
	}
	return m.scip.nNodes(), nil
}

// NNodes returns the number of nodes explored by the optimization model.
func (m Model) NNodes() int {
	v, err := m.TryNNodes()
	must(err)
	return v
}

// TrySolvingTime returns the total solving time in seconds.
func (m Model) TrySolvingTime() (float64, error) {
	if err := m.query("SolvingTime", stagesSolvingTime); err != nil {
		return 0, err
	}
	return m.scip.solvingTime(), nil
}

// SolvingTime returns the total solving time of the optimization model.
func (m Model) SolvingTime() float64 {
	v, err := m.TrySolvingTime()
	must(err)
	return v
}

// NLpIterations returns the number of LP iterations performed.
func (m Model) NLpIterations() int {
	must(m.query("NLpIterations", stagesLPIter))
	return m.scip.nLPIterations()
}

// TryStatsJSON returns the solving statistics in JSON format.
func (m Model) TryStatsJSON() (string, error) {
	if err := m.guard("StatsJSON"); err != nil {
		return "", err
	}
	j, err := m.scip.statisticsJSON()
	return j, m.wrap("StatsJSON", err, "")
}

// StatsJSON returns the solving statistics in JSON format. It panics on failure.
func (m Model) StatsJSON() string {
	j, err := m.TryStatsJSON()
	must(err)
	return j
}

// WriteStatsJSON writes the solving statistics in JSON format to the given path.
func (m Model) WriteStatsJSON(path string) error {
	if err := m.guard("WriteStatsJSON"); err != nil {
		return err
	}
	return m.wrap("WriteStatsJSON", m.scip.writeStatisticsJSON(path), path)
}

// ------------------------------------------------------------- tree

// TryFocusNode returns the node currently being processed.
func (m Model) TryFocusNode() (Node, error) {
	if err := m.query("FocusNode", stagesFocus); err != nil {
		return Node{}, err
	}
	raw := m.scip.focusNode()
	if raw == nil {
		return Node{}, m.invalid("FocusNode", RetcodeInvalidCall, "no focus node; not solving")
	}
	return m.scip.newNode(raw), nil
}

// FocusNode returns the node currently being processed. It panics if there
// is none.
func (m Model) FocusNode() Node {
	n, err := m.TryFocusNode()
	must(err)
	return n
}

// TryCreateChild creates a new child of the focus node and returns it.
func (m Model) TryCreateChild() (Node, error) {
	// createChild reads SCIPgetLocalTransEstimate, which aborts outside solving.
	if err := m.query("CreateChild", stagesSolving); err != nil {
		return Node{}, err
	}
	raw, err := m.scip.createChild()
	if err != nil {
		return Node{}, m.wrap("CreateChild", err, "")
	}
	return m.scip.newNode(raw), nil
}

// CreateChild creates a new child of the focus node. It panics on failure.
func (m Model) CreateChild() Node {
	n, err := m.TryCreateChild()
	must(err)
	return n
}

func (m Model) wrapNode(ptr *C.SCIP_NODE) *Node {
	if ptr == nil {
		return nil
	}
	n := m.scip.newNode(ptr)
	return &n
}

// treeNode is the common shape of the optional tree accessors: a freed model
// panics with *Error, and outside the solving stage there is no tree, so the
// answer is nil rather than an abort inside SCIP.
func (m Model) treeNode(op string, get func() *C.SCIP_NODE) *Node {
	must(m.guard(op))
	if !stagesSolving.has(m.scip.stage()) {
		return nil
	}
	return m.wrapNode(get())
}

func (m Model) treeNodes(op string, allowed stageSet, get func() []*C.SCIP_NODE) []Node {
	must(m.guard(op))
	if !allowed.has(m.scip.stage()) {
		return nil
	}
	return m.wrapNodes(get())
}

// BestNode returns the best open node with respect to the active node
// selector, or nil if the tree is empty.
func (m Model) BestNode() *Node { return m.treeNode("BestNode", m.scip.bestNode) }

// BestBoundNode returns the open node with the best (smallest) lower bound,
// or nil if the tree is empty.
func (m Model) BestBoundNode() *Node { return m.treeNode("BestBoundNode", m.scip.bestBoundNode) }

// BestLeaf returns the best leaf from the leaf queue with respect to the
// active node selector, or nil if it is empty.
func (m Model) BestLeaf() *Node { return m.treeNode("BestLeaf", m.scip.bestLeaf) }

// BestChild returns the best child of the focus node with respect to the
// active node selector, or nil if there is none.
func (m Model) BestChild() *Node { return m.treeNode("BestChild", m.scip.bestChild) }

// BestSibling returns the best sibling of the focus node with respect to the
// active node selector, or nil if there is none.
func (m Model) BestSibling() *Node { return m.treeNode("BestSibling", m.scip.bestSibling) }

// PrioChild returns the child of the focus node with the largest node
// selection priority, or nil if there is none.
func (m Model) PrioChild() *Node { return m.treeNode("PrioChild", m.scip.prioChild) }

// PrioSibling returns the sibling of the focus node with the largest node
// selection priority, or nil if there is none.
func (m Model) PrioSibling() *Node { return m.treeNode("PrioSibling", m.scip.prioSibling) }

// Leaves returns the leaves of the branch-and-bound tree (the open nodes
// that are neither children nor siblings of the focus node).
func (m Model) Leaves() []Node { return m.treeNodes("Leaves", stagesSolvingSolved, m.scip.leaves) }

// Children returns the children of the focus node.
func (m Model) Children() []Node {
	return m.treeNodes("Children", stagesSolvingSolved, m.scip.children)
}

// Siblings returns the siblings of the focus node.
func (m Model) Siblings() []Node {
	return m.treeNodes("Siblings", stagesSolvingSolved, m.scip.siblings)
}

func (m Model) wrapNodes(ptrs []*C.SCIP_NODE) []Node {
	out := make([]Node, 0, len(ptrs))
	for _, p := range ptrs {
		out = append(out, m.scip.newNode(p))
	}
	return out
}

// NodeGetNAddedConss returns the number of added constraints to the given node.
func (m Model) NodeGetNAddedConss(node *Node) int {
	must(m.guard("NodeGetNAddedConss"))
	must(m.checkNode("NodeGetNAddedConss", node))
	return m.scip.nodeGetNAddedConss(*node)
}

// VarInProb gets the variable in the current problem given its index, if it
// exists.
func (m Model) VarInProb(varProbID int) (Variable, bool) {
	must(m.query("VarInProb", stagesTrans))
	v := varFromID(m.scip.raw, varProbID)
	if v == nil {
		return Variable{}, false
	}
	return m.scip.newVar(v), true
}

// TryAddCut adds a new cut (row) to the LP. It reports whether the row is
// infeasible from the local bounds.
func (m Model) TryAddCut(cut Row, forceCut bool) (bool, error) {
	if err := m.guard("AddCut"); err != nil {
		return false, err
	}
	if err := m.checkHandle("AddCut", "Row", cut.raw != nil, cut.scip, cut.gen, false); err != nil {
		return false, err
	}
	infeasible, err := m.scip.addRow(cut, forceCut)
	return infeasible, m.wrap("AddCut", err, cut.Name())
}

// AddCut adds a new cut to the LP; see TryAddCut. It panics on failure.
func (m Model) AddCut(cut Row, forceCut bool) bool {
	infeasible, err := m.TryAddCut(cut, forceCut)
	must(err)
	return infeasible
}

// CurrentVal returns the value of a variable in the current LP/pseudo solution.
func (m Model) CurrentVal(v Variable) float64 {
	must(m.query("CurrentVal", stagesVarSol))
	must(m.checkVars("CurrentVal", v))
	return float64(C.SCIPgetSolVal(m.scip.raw, nil, v.raw))
}

// TryStartProbing starts probing at the current node. The returned Prober
// must be ended with End.
func (m Model) TryStartProbing() (*Prober, error) {
	if err := m.guard("StartProbing"); err != nil {
		return nil, err
	}
	if err := m.call("StartProbing", C.SCIPstartProbing(m.scip.raw)); err != nil {
		return nil, err
	}
	return &Prober{scip: m.scip}, nil
}

// StartProbing starts probing at the current node. It panics on failure.
func (m Model) StartProbing() *Prober {
	p, err := m.TryStartProbing()
	must(err)
	return p
}

// TryStartDiving starts diving at the current node. The returned Diver must
// be ended with End.
func (m Model) TryStartDiving() (*Diver, error) {
	// SCIPisLPConstructed and SCIPstartDive abort outside solving.
	if err := m.query("StartDiving", stagesSolving); err != nil {
		return nil, err
	}
	// Since SCIP 10, SCIPstartDive requires the current node's LP to be
	// constructed first; construct it on demand.
	if C.SCIPisLPConstructed(m.scip.raw) == 0 {
		var cutoff C.uint
		if err := m.call("StartDiving", C.SCIPconstructLP(m.scip.raw, &cutoff)); err != nil {
			return nil, err
		}
	}
	if err := m.call("StartDiving", C.SCIPstartDive(m.scip.raw)); err != nil {
		return nil, err
	}
	return &Diver{scip: m.scip}, nil
}

// StartDiving starts diving at the current node. It panics on failure.
func (m Model) StartDiving() *Diver {
	d, err := m.TryStartDiving()
	must(err)
	return d
}

// LpObjVal returns the objective value of the current LP relaxation.
func (m Model) LpObjVal() float64 {
	must(m.query("LpObjVal", stagesSolving))
	return m.scip.lpObjVal()
}

// LpStatus returns the status of the current LP solve.
func (m Model) LpStatus() LPStatus {
	must(m.query("LpStatus", stagesSolving))
	return m.scip.lpStatus()
}

// TrySetUbNode changes the upper bound of the variable in a given node.
func (m Model) TrySetUbNode(node *Node, v Variable, ub float64) error {
	if err := m.guard("SetUbNode"); err != nil {
		return err
	}
	if err := m.checkNode("SetUbNode", node); err != nil {
		return err
	}
	if err := m.checkVars("SetUbNode", v); err != nil {
		return err
	}
	return m.call("SetUbNode", C.SCIPchgVarUbNode(m.scip.raw, node.raw, v.raw, C.double(ub)))
}

// SetUbNode changes the upper bound of the variable in a given node. It
// panics on failure.
func (m Model) SetUbNode(node *Node, v Variable, ub float64) { must(m.TrySetUbNode(node, v, ub)) }

// TrySetLbNode changes the lower bound of the variable in a given node.
func (m Model) TrySetLbNode(node *Node, v Variable, lb float64) error {
	if err := m.guard("SetLbNode"); err != nil {
		return err
	}
	if err := m.checkNode("SetLbNode", node); err != nil {
		return err
	}
	if err := m.checkVars("SetLbNode", v); err != nil {
		return err
	}
	return m.call("SetLbNode", C.SCIPchgVarLbNode(m.scip.raw, node.raw, v.raw, C.double(lb)))
}

// SetLbNode changes the lower bound of the variable in a given node. It
// panics on failure.
func (m Model) SetLbNode(node *Node, v Variable, lb float64) { must(m.TrySetLbNode(node, v, lb)) }

func scipSols(s *Scip) []Solution {
	raw := s.getSols()
	out := make([]Solution, 0, len(raw))
	for _, sol := range raw {
		out = append(out, s.newSol(sol))
	}
	return out
}

func bestSolOf(s *Scip) (Solution, bool) {
	raw := s.bestSol()
	if raw == nil {
		return Solution{}, false
	}
	return s.newSol(raw), true
}
