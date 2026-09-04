package scip

/*
#include "helpers.h"
*/
import "C"

// VarId is a variable ID (the variable's problem index).
type VarId = int

// Variable is a wrapper for a mutable reference to a SCIP variable.
type Variable struct {
	raw  *C.SCIP_VAR
	scip *Scip
}

// Inner returns the raw pointer to the underlying SCIP_VAR.
func (v Variable) Inner() *C.SCIP_VAR { return v.raw }

// Index returns the index of the variable.
func (v Variable) Index() VarId {
	id := C.SCIPvarGetIndex(v.raw)
	if id < 0 {
		panic("negative variable index")
	}
	return int(id)
}

// Name returns the name of the variable.
func (v Variable) Name() string { return goString(C.SCIPvarGetName(v.raw)) }

// safeName is Name for error messages: it never dereferences a zero or
// dangling handle.
func (v Variable) safeName() string {
	switch {
	case v.raw == nil:
		return "<zero Variable>"
	case v.scip == nil || v.scip.raw == nil:
		return "<Variable of a freed model>"
	}
	return v.Name()
}

// Obj returns the objective coefficient of the variable.
func (v Variable) Obj() float64 { return float64(C.SCIPvarGetObj(v.raw)) }

// Lb returns the lower bound of the variable (local).
func (v Variable) Lb() float64 { return float64(C.SCIPvarGetLbLocal(v.raw)) }

// Ub returns the upper bound of the variable (local).
func (v Variable) Ub() float64 { return float64(C.SCIPvarGetUbLocal(v.raw)) }

// LbLocal returns the local lower bound of the variable.
func (v Variable) LbLocal() float64 { return float64(C.SCIPvarGetLbLocal(v.raw)) }

// UbLocal returns the local upper bound of the variable.
func (v Variable) UbLocal() float64 { return float64(C.SCIPvarGetUbLocal(v.raw)) }

// LbGlobal returns the global lower bound of the variable.
func (v Variable) LbGlobal() float64 { return float64(C.SCIPvarGetLbGlobal(v.raw)) }

// UbGlobal returns the global upper bound of the variable.
func (v Variable) UbGlobal() float64 { return float64(C.SCIPvarGetUbGlobal(v.raw)) }

// VarType returns the type of the variable.
func (v Variable) VarType() VarType { return varTypeFromC(C.SCIPvarGetType(v.raw)) }

// Status returns the status of the variable.
func (v Variable) Status() VarStatus { return varStatusFromC(C.SCIPvarGetStatus(v.raw)) }

// Col returns the column associated with the variable, if it is a column
// variable in the LP.
func (v Variable) Col() (Col, bool) {
	if v.Status() == VarStatusColumn {
		return Col{raw: C.SCIPvarGetCol(v.raw), scip: v.scip}, true
	}
	return Col{}, false
}

// IsInLP returns whether the variable is a column variable in the LP relaxation.
func (v Variable) IsInLP() bool { return C.SCIPvarIsInLP(v.raw) != 0 }

// SolVal returns the solution value of the variable in the current node.
func (v Variable) SolVal() float64 { return float64(C.SCIPgetVarSol(v.scip.raw, v.raw)) }

// IsDeleted returns whether the variable is deleted.
func (v Variable) IsDeleted() bool { return C.SCIPvarIsDeleted(v.raw) != 0 }

// IsTransformed returns whether the variable is transformed.
func (v Variable) IsTransformed() bool { return C.SCIPvarIsTransformed(v.raw) != 0 }

// IsOriginal returns whether the variable is original.
func (v Variable) IsOriginal() bool { return C.SCIPvarIsOriginal(v.raw) != 0 }

// IsNegated returns whether the variable is negated.
func (v Variable) IsNegated() bool { return C.SCIPvarIsNegated(v.raw) != 0 }

// IsRemovable returns whether the variable is removable (due to aging in the LP).
func (v Variable) IsRemovable() bool { return C.SCIPvarIsRemovable(v.raw) != 0 }

// IsTransFromOrig returns whether the variable is a directed counterpart of
// an original variable.
func (v Variable) IsTransFromOrig() bool { return C.SCIPvarIsTransformedOrigvar(v.raw) != 0 }

// IsActive returns whether the variable is active (neither fixed nor aggregated).
func (v Variable) IsActive() bool { return C.SCIPvarIsActive(v.raw) != 0 }

// Transformed returns the transformed variable if it exists.
func (v Variable) Transformed() (Variable, bool) {
	varPtr := C.SCIPvarGetTransVar(v.raw)
	if varPtr == nil {
		return Variable{}, false
	}
	return Variable{raw: varPtr, scip: v.scip}, true
}

// Redcost returns the reduced costs of the variable in the current node's LP
// relaxation. Returns (0, false) if the variable is active but not in the
// current LP, (rc, true) with the reduced cost otherwise. SCIP_INVALID is
// reported as not-present.
func (v Variable) Redcost() (float64, bool) {
	rc := float64(C.SCIPgetVarRedcost(v.scip.raw, v.raw))
	if rc == scipInvalid {
		return 0, false
	}
	return rc, true
}

// VarType describes the type of a variable in an optimization problem.
type VarType int

// Variable types.
const (
	VarTypeContinuous VarType = iota // Continuous variable
	VarTypeInteger                   // Integer variable
	VarTypeBinary                    // Binary variable
	VarTypeImplInt                   // Implicit integer variable
)

// String implements fmt.Stringer.
func (t VarType) String() string {
	switch t {
	case VarTypeContinuous:
		return "Continuous"
	case VarTypeInteger:
		return "Integer"
	case VarTypeBinary:
		return "Binary"
	case VarTypeImplInt:
		return "ImplInt"
	default:
		return "Unknown"
	}
}

func (t VarType) toC() C.SCIP_VARTYPE {
	switch t {
	case VarTypeContinuous:
		return C.SCIP_VARTYPE_CONTINUOUS
	case VarTypeInteger:
		return C.SCIP_VARTYPE_INTEGER
	case VarTypeBinary:
		return C.SCIP_VARTYPE_BINARY
	default:
		return C.SCIP_VARTYPE_IMPLINT
	}
}

func varTypeFromC(t C.SCIP_VARTYPE) VarType {
	switch t {
	case C.SCIP_VARTYPE_CONTINUOUS:
		return VarTypeContinuous
	case C.SCIP_VARTYPE_INTEGER:
		return VarTypeInteger
	case C.SCIP_VARTYPE_BINARY:
		return VarTypeBinary
	case C.SCIP_VARTYPE_IMPLINT:
		return VarTypeImplInt
	default:
		panic("unknown SCIP variable type")
	}
}

// VarStatus represents the status of a SCIP variable.
type VarStatus int

// Variable statuses.
const (
	VarStatusOriginal        VarStatus = iota // Original variable
	VarStatusLoose                            // Loose variable
	VarStatusColumn                           // Column variable
	VarStatusFixed                            // Fixed variable
	VarStatusAggregated                       // Aggregated variable
	VarStatusMultiAggregated                  // Multi-aggregated variable
	VarStatusNegatedVar                       // Negated variable
)

func varStatusFromC(s C.SCIP_VARSTATUS) VarStatus {
	switch s {
	case C.SCIP_VARSTATUS_ORIGINAL:
		return VarStatusOriginal
	case C.SCIP_VARSTATUS_LOOSE:
		return VarStatusLoose
	case C.SCIP_VARSTATUS_COLUMN:
		return VarStatusColumn
	case C.SCIP_VARSTATUS_FIXED:
		return VarStatusFixed
	case C.SCIP_VARSTATUS_AGGREGATED:
		return VarStatusAggregated
	case C.SCIP_VARSTATUS_MULTAGGR:
		return VarStatusMultiAggregated
	case C.SCIP_VARSTATUS_NEGATED:
		return VarStatusNegatedVar
	default:
		panic("unknown SCIP variable status")
	}
}
