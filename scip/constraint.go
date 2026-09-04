package scip

/*
#include "helpers.h"
*/
import "C"

// Constraint is a constraint in an optimization problem.
type Constraint struct {
	raw  *C.SCIP_CONS
	scip *Scip
}

// Inner returns the raw pointer to the underlying SCIP_CONS.
func (c Constraint) Inner() *C.SCIP_CONS { return c.raw }

// Name returns the name of the constraint.
func (c Constraint) Name() string {
	mustLive("Constraint.Name", "Constraint", c.raw != nil, c.scip)
	return goString(C.SCIPconsGetName(c.raw))
}

// Row returns the row associated with the constraint, if any.
func (c Constraint) Row() (Row, bool) {
	mustLive("Constraint.Row", "Constraint", c.raw != nil, c.scip)
	rowPtr := C.SCIPconsGetRow(c.scip.raw, c.raw)
	if rowPtr == nil {
		return Row{}, false
	}
	return Row{raw: rowPtr, scip: c.scip}, true
}

// DualSol returns the dual solution of the linear constraint in the current
// LP. Returns false if the constraint is not a linear constraint.
func (c Constraint) DualSol() (float64, bool) {
	mustLive("Constraint.DualSol", "Constraint", c.raw != nil, c.scip)
	if !c.isLinearCons() {
		return 0, false
	}
	return float64(C.SCIPgetDualsolLinear(c.scip.raw, c.raw)), true
}

// FarkasDualSol returns the Farkas dual solution of the linear constraint in
// the current (infeasible) LP. Returns false if the constraint is not a
// linear constraint.
func (c Constraint) FarkasDualSol() (float64, bool) {
	mustLive("Constraint.FarkasDualSol", "Constraint", c.raw != nil, c.scip)
	if !c.isLinearCons() {
		return 0, false
	}
	return float64(C.SCIPgetDualfarkasLinear(c.scip.raw, c.raw)), true
}

func (c Constraint) isLinearCons() bool {
	mustLive("Constraint.isLinearCons", "Constraint", c.raw != nil, c.scip)
	hdlr := C.SCIPconsGetHdlr(c.raw)
	if hdlr == nil {
		return false
	}
	name := C.SCIPconshdlrGetName(hdlr)
	if name == nil {
		return false
	}
	return goString(name) == "linear"
}

// IsModifiable returns the modifiable flag of the constraint.
func (c Constraint) IsModifiable() bool {
	mustLive("Constraint.IsModifiable", "Constraint", c.raw != nil, c.scip)
	return c.scip.consIsModifiable(c)
}

// IsRemovable returns the removable flag of the constraint.
func (c Constraint) IsRemovable() bool {
	mustLive("Constraint.IsRemovable", "Constraint", c.raw != nil, c.scip)
	return c.scip.consIsRemovable(c)
}

// IsSeparated returns whether the constraint should be separated during LP
// processing.
func (c Constraint) IsSeparated() bool {
	mustLive("Constraint.IsSeparated", "Constraint", c.raw != nil, c.scip)
	return c.scip.consIsSeparated(c)
}

// Transformed returns the corresponding transformed constraint, if it exists.
func (c Constraint) Transformed() (Constraint, bool) {
	mustLive("Constraint.Transformed", "Constraint", c.raw != nil, c.scip)
	ptr, err := c.scip.getTransformedCons(c)
	if err != nil {
		return Constraint{}, false
	}
	if ptr == nil {
		return Constraint{}, false
	}
	return Constraint{raw: ptr, scip: c.scip}, true
}
