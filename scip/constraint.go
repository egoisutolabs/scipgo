package scip

/*
#include "helpers.h"
*/
import "C"

import "runtime"

// Constraint is a constraint in an optimization problem.
type Constraint struct {
	raw  *C.SCIP_CONS
	scip *Scip
	gen  uint64 // transform generation at creation; see handleErr
	orig bool   // original-problem handles survive FreeTransform
}

func (s *Scip) newCons(raw *C.SCIP_CONS) Constraint {
	defer runtime.KeepAlive(s) // the C call must outlive the last Go use of the wrapper
	h := Constraint{raw: raw, scip: s}
	if raw != nil {
		h.orig = C.SCIPconsIsOriginal(raw) != 0
		h.gen = s.gen(h.orig)
	}
	return h
}

// live panics with *Error unless the handle is usable; see handleErr.
func (h Constraint) live(op string) { mustLive(op, "Constraint", h.raw != nil, h.scip, h.gen, h.orig) }

// Inner returns the raw pointer to the underlying SCIP_CONS.
func (c Constraint) Inner() *C.SCIP_CONS { return c.raw }

// Name returns the name of the constraint.
func (c Constraint) Name() string {
	defer runtime.KeepAlive(c.scip) // the C call must outlive the last Go use of the wrapper
	c.live("Constraint.Name")
	return goString(C.SCIPconsGetName(c.raw))
}

// Row returns the row associated with the constraint, if any.
func (c Constraint) Row() (Row, bool) {
	defer runtime.KeepAlive(c.scip) // the C call must outlive the last Go use of the wrapper
	c.live("Constraint.Row")
	rowPtr := C.SCIPconsGetRow(c.scip.raw, c.raw)
	if rowPtr == nil {
		return Row{}, false
	}
	return c.scip.newRow(rowPtr), true
}

// DualSol returns the dual solution of the linear constraint in the current
// LP. Returns false if the constraint is not a linear constraint.
func (c Constraint) DualSol() (float64, bool) {
	defer runtime.KeepAlive(c.scip) // the C call must outlive the last Go use of the wrapper
	c.live("Constraint.DualSol")
	if !c.isLinearCons() {
		return 0, false
	}
	return float64(C.SCIPgetDualsolLinear(c.scip.raw, c.raw)), true
}

// FarkasDualSol returns the Farkas dual solution of the linear constraint in
// the current (infeasible) LP. Returns false if the constraint is not a
// linear constraint.
func (c Constraint) FarkasDualSol() (float64, bool) {
	defer runtime.KeepAlive(c.scip) // the C call must outlive the last Go use of the wrapper
	c.live("Constraint.FarkasDualSol")
	if !c.isLinearCons() {
		return 0, false
	}
	return float64(C.SCIPgetDualfarkasLinear(c.scip.raw, c.raw)), true
}

func (c Constraint) isLinearCons() bool {
	defer runtime.KeepAlive(c.scip) // the C call must outlive the last Go use of the wrapper
	c.live("Constraint.isLinearCons")
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
	c.live("Constraint.IsModifiable")
	return c.scip.consIsModifiable(c)
}

// IsRemovable returns the removable flag of the constraint.
func (c Constraint) IsRemovable() bool {
	c.live("Constraint.IsRemovable")
	return c.scip.consIsRemovable(c)
}

// IsSeparated returns whether the constraint should be separated during LP
// processing.
func (c Constraint) IsSeparated() bool {
	c.live("Constraint.IsSeparated")
	return c.scip.consIsSeparated(c)
}

// Transformed returns the corresponding transformed constraint, if it exists.
func (c Constraint) Transformed() (Constraint, bool) {
	c.live("Constraint.Transformed")
	ptr, err := c.scip.getTransformedCons(c)
	if err != nil {
		return Constraint{}, false
	}
	if ptr == nil {
		return Constraint{}, false
	}
	return c.scip.newCons(ptr), true
}
