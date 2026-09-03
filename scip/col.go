package scip

/*
#include "helpers.h"
*/
import "C"

// Col is a column in the LP relaxation.
type Col struct {
	raw  *C.SCIP_COL
	scip *Scip
}

// Inner returns the raw pointer to the underlying SCIP_COL.
func (c Col) Inner() *C.SCIP_COL { return c.raw }

// Index returns the index of the column.
func (c Col) Index() int { return int(C.SCIPcolGetIndex(c.raw)) }

// Obj returns the objective coefficient of the column.
func (c Col) Obj() float64 { return float64(C.SCIPcolGetObj(c.raw)) }

// Lb returns the lower bound of the column.
func (c Col) Lb() float64 { return float64(C.SCIPcolGetLb(c.raw)) }

// Ub returns the upper bound of the column.
func (c Col) Ub() float64 { return float64(C.SCIPcolGetUb(c.raw)) }

// BestBound returns the best bound of the column with respect to the
// objective function.
func (c Col) BestBound() float64 { return float64(C.SCIPcolGetBestBound(c.raw)) }

// Var returns the variable associated with the column.
func (c Col) Var() Variable { return Variable{raw: C.SCIPcolGetVar(c.raw), scip: c.scip} }

// PrimalSol returns the primal LP solution of the column.
func (c Col) PrimalSol() float64 { return float64(C.SCIPcolGetPrimsol(c.raw)) }

// MinPrimalSol returns the minimal LP solution value this column ever assumed.
func (c Col) MinPrimalSol() float64 { return float64(C.SCIPcolGetMinPrimsol(c.raw)) }

// MaxPrimalSol returns the maximal LP solution value this column ever assumed.
func (c Col) MaxPrimalSol() float64 { return float64(C.SCIPcolGetMaxPrimsol(c.raw)) }

// BasisStatus returns the basis status of the column in the LP solution.
func (c Col) BasisStatus() BasisStatus { return basisStatusFromC(C.SCIPcolGetBasisStatus(c.raw)) }

// VarProbindex returns the probindex of the corresponding variable, if valid.
func (c Col) VarProbindex() (int, bool) {
	idx := C.SCIPcolGetVarProbindex(c.raw)
	if idx < 0 {
		return 0, false
	}
	return int(idx), true
}

// IsIntegral returns whether the column is of integral type.
func (c Col) IsIntegral() bool { return C.SCIPcolIsIntegral(c.raw) != 0 }

// IsRemovable returns whether the column is removable from the LP.
func (c Col) IsRemovable() bool { return C.SCIPcolIsRemovable(c.raw) != 0 }

// LpPos returns the position of the column in the current LP, if present.
func (c Col) LpPos() (int, bool) {
	pos := C.SCIPcolGetLPPos(c.raw)
	if pos < 0 {
		return 0, false
	}
	return int(pos), true
}

// LpDepth returns the depth in the tree where the column entered the LP, if
// applicable.
func (c Col) LpDepth() (int, bool) {
	depth := C.SCIPcolGetLPDepth(c.raw)
	if depth < 0 {
		return 0, false
	}
	return int(depth), true
}

// IsInLP returns whether the column is in the current LP.
func (c Col) IsInLP() bool { return C.SCIPcolIsInLP(c.raw) != 0 }

// NNonZeros returns the number of non-zero entries.
func (c Col) NNonZeros() int { return int(C.SCIPcolGetNNonz(c.raw)) }

// NLpNonZeros returns the number of non-zero entries that correspond to rows
// currently in the LP.
func (c Col) NLpNonZeros() int { return int(C.SCIPcolGetNLPNonz(c.raw)) }

// Rows returns the rows of non-zero entries.
func (c Col) Rows() []Row {
	n := c.NNonZeros()
	rowsPtr := C.SCIPcolGetRows(c.raw)
	rows := make([]Row, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, Row{raw: cAt(rowsPtr, i), scip: c.scip})
	}
	return rows
}

// Vals returns the coefficients of non-zero entries.
func (c Col) Vals() []float64 {
	n := c.NNonZeros()
	valsPtr := C.SCIPcolGetVals(c.raw)
	vals := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		vals = append(vals, float64(cAt(valsPtr, i)))
	}
	return vals
}

// StrongBranchingNode returns the node number of the last node in the current
// branch and bound run where strong branching was used on the given column.
func (c Col) StrongBranchingNode() (int64, bool) {
	node := C.SCIPcolGetStrongbranchNode(c.raw)
	if node < 0 {
		return 0, false
	}
	return int64(node), true
}

// NStrongBranches returns the number of times strong branching was applied in
// the current run on the given column.
func (c Col) NStrongBranches() int { return int(C.SCIPcolGetNStrongbranchs(c.raw)) }

// Age returns the age of the column: the total number of successive times a
// column was in the LP and was 0.0 in the solution.
func (c Col) Age() int { return int(C.SCIPcolGetAge(c.raw)) }

// Redcost returns the reduced cost of the column.
func (c Col) Redcost() float64 { return float64(C.SCIPgetColRedcost(c.scip.raw, c.raw)) }
