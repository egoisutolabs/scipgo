package scip

/*
#include "helpers.h"
*/
import "C"

// Row is a row in the LP relaxation.
type Row struct {
	raw  *C.SCIP_ROW
	scip *Scip
	gen  uint64 // transform generation at creation; see handleErr
}

func (s *Scip) newRow(raw *C.SCIP_ROW) Row {
	h := Row{raw: raw, scip: s}
	if raw != nil {
		h.gen = s.gen()
	}
	return h
}

// live panics with *Error unless the handle is usable; see handleErr.
func (h Row) live(op string) { mustLive(op, "Row", h.raw != nil, h.scip, h.gen, false) }

// Inner returns the raw pointer to the underlying SCIP_ROW.
func (r Row) Inner() *C.SCIP_ROW { return r.raw }

// NNonZeroes returns the number of non-zero entries in the row.
func (r Row) NNonZeroes() int {
	r.live("Row.NNonZeroes")
	return int(C.SCIProwGetNNonz(r.raw))
}

// Cols returns the columns of the row.
func (r Row) Cols() []Col {
	r.live("Row.Cols")
	n := r.NNonZeroes()
	colsPtr := C.SCIProwGetCols(r.raw)
	cols := make([]Col, 0, n)
	for i := 0; i < n; i++ {
		cols = append(cols, Col{raw: cColAt(colsPtr, i), scip: r.scip})
	}
	return cols
}

// Index returns the index of the row.
func (r Row) Index() int {
	r.live("Row.Index")
	return int(C.SCIProwGetIndex(r.raw))
}

// Lhs returns the left-hand side of the row.
func (r Row) Lhs() float64 {
	r.live("Row.Lhs")
	return float64(C.SCIProwGetLhs(r.raw))
}

// Rhs returns the right-hand side of the row.
func (r Row) Rhs() float64 {
	r.live("Row.Rhs")
	return float64(C.SCIProwGetRhs(r.raw))
}

// Dual returns the dual value of the row.
func (r Row) Dual() float64 {
	r.live("Row.Dual")
	return float64(C.SCIProwGetDualsol(r.raw))
}

// FarkasDual returns the Farkas dual value of the row.
func (r Row) FarkasDual() float64 {
	r.live("Row.FarkasDual")
	return float64(C.SCIProwGetDualfarkas(r.raw))
}

// BasisStatus returns the basis status of the row.
func (r Row) BasisStatus() BasisStatus {
	r.live("Row.BasisStatus")
	return basisStatusFromC(C.SCIProwGetBasisStatus(r.raw))
}

// Name returns the name of the row.
func (r Row) Name() string {
	r.live("Row.Name")
	return goString(C.SCIProwGetName(r.raw))
}

// Age returns the age of the row.
func (r Row) Age() int {
	r.live("Row.Age")
	return int(C.SCIProwGetAge(r.raw))
}

// Rank returns the rank of the row.
func (r Row) Rank() int {
	r.live("Row.Rank")
	return int(C.SCIProwGetRank(r.raw))
}

// IsLocal returns whether the row is local.
func (r Row) IsLocal() bool {
	r.live("Row.IsLocal")
	return C.SCIProwIsLocal(r.raw) != 0
}

// IsModifiable returns whether the row is modifiable.
func (r Row) IsModifiable() bool {
	r.live("Row.IsModifiable")
	return C.SCIProwIsModifiable(r.raw) != 0
}

// IsRemovable returns whether the row is removable.
func (r Row) IsRemovable() bool {
	r.live("Row.IsRemovable")
	return C.SCIProwIsRemovable(r.raw) != 0
}

// IsIntegral returns whether the row is integral; the activity of an integral
// row (without the constant) is always integral.
func (r Row) IsIntegral() bool {
	r.live("Row.IsIntegral")
	return C.SCIProwIsIntegral(r.raw) != 0
}

// OriginType returns the origin type of the row.
func (r Row) OriginType() RowOrigin {
	r.live("Row.OriginType")
	return rowOriginFromC(C.SCIProwGetOrigintype(r.raw))
}

// Constraint returns the constraint associated with the row, if it was
// created by a constraint.
func (r Row) Constraint() (Constraint, bool) {
	r.live("Row.Constraint")
	consPtr := C.SCIProwGetOriginCons(r.raw)
	if consPtr == nil {
		return Constraint{}, false
	}
	return r.scip.newCons(consPtr), true
}

// IsInGlobalCutPool returns whether the row is in the global cut pool.
func (r Row) IsInGlobalCutPool() bool {
	r.live("Row.IsInGlobalCutPool")
	return C.SCIProwIsInGlobalCutpool(r.raw) != 0
}

// IsInLP returns whether the row is in the current LP.
func (r Row) IsInLP() bool {
	r.live("Row.IsInLP")
	return C.SCIProwIsInLP(r.raw) != 0
}

// LpPosition returns the position of the row in the current LP, if present.
func (r Row) LpPosition() (int, bool) {
	r.live("Row.LpPosition")
	if r.IsInLP() {
		return int(C.SCIProwGetLPPos(r.raw)), true
	}
	return 0, false
}

// Depth returns the depth of the row; the depth in the tree when the row was
// introduced.
func (r Row) Depth() int {
	r.live("Row.Depth")
	return int(C.SCIProwGetLPDepth(r.raw))
}

// ActiveLPCount returns the number of times that this row has been sharp in
// an optimal LP solution.
func (r Row) ActiveLPCount() int {
	r.live("Row.ActiveLPCount")
	return int(C.SCIProwGetActiveLPCount(r.raw))
}

// NLpSinceCreate returns the number of LPs since this row has been created.
func (r Row) NLpSinceCreate() int {
	r.live("Row.NLpSinceCreate")
	return int(C.SCIProwGetNLPsAfterCreation(r.raw))
}

// SetRank sets the rank of the row.
func (r *Row) SetRank(rank int) {
	r.live("Row.SetRank")
	C.SCIProwChgRank(r.raw, C.int(rank))
}

// SetCoeff sets the coefficient of a variable in the row.
func (r *Row) SetCoeff(v Variable, coeff float64) {
	r.live("Row.SetCoeff")
	must(r.TrySetCoeff(v, coeff))
}

// TrySetCoeff is SetCoeff returning an error instead of panicking.
func (r *Row) TrySetCoeff(v Variable, coeff float64) error {
	m := Model{scip: r.scip}
	if err := m.checkHandle("Row.SetCoeff", "Row", r.raw != nil, r.scip, r.gen, false); err != nil {
		return err
	}
	if err := m.checkVars("Row.SetCoeff", v); err != nil {
		return err
	}
	return m.call("Row.SetCoeff", C.SCIPaddVarToRow(r.scip.raw, r.raw, v.raw, C.double(coeff)))
}

// BasisStatus is the basis status of a row.
type BasisStatus int

// Basis statuses.
const (
	BasisStatusLower BasisStatus = iota // The row is at its lower bound
	BasisStatusBasic                    // The row is basic
	BasisStatusUpper                    // The row is at its upper bound
	BasisStatusZero                     // The row is at zero
)

func basisStatusFromC(s C.SCIP_BASESTAT) BasisStatus {
	switch s {
	case C.SCIP_BASESTAT_LOWER:
		return BasisStatusLower
	case C.SCIP_BASESTAT_BASIC:
		return BasisStatusBasic
	case C.SCIP_BASESTAT_UPPER:
		return BasisStatusUpper
	case C.SCIP_BASESTAT_ZERO:
		return BasisStatusZero
	default:
		panic("unknown basis status")
	}
}

// RowOrigin is the origin type of a row.
type RowOrigin int

// Row origin types.
const (
	RowOriginConsHandler    RowOrigin = iota // Created by a constraint handler
	RowOriginConstraint                      // Created by a constraint
	RowOriginReoptimization                  // Created by reoptimization
	RowOriginSeparator                       // Created by a separator
	RowOriginUnspecified                     // Origin is unspecified
)

func rowOriginFromC(o C.SCIP_ROWORIGINTYPE) RowOrigin {
	switch o {
	case C.SCIP_ROWORIGINTYPE_CONSHDLR:
		return RowOriginConsHandler
	case C.SCIP_ROWORIGINTYPE_CONS:
		return RowOriginConstraint
	case C.SCIP_ROWORIGINTYPE_REOPT:
		return RowOriginReoptimization
	case C.SCIP_ROWORIGINTYPE_SEPA:
		return RowOriginSeparator
	case C.SCIP_ROWORIGINTYPE_UNSPEC:
		return RowOriginUnspecified
	default:
		panic("unknown row origin type")
	}
}
