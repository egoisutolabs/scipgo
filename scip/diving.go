package scip

/*
#include "helpers.h"
*/
import "C"

import "runtime"

// Diver drives SCIP's diving mode, started with Model.StartDiving and ended
// with End. Every method reports "not diving", a freed model or a wrong
// stage as *Error: the plain forms panic with it, the Try forms return it.
type Diver struct {
	scip *Scip
}

// active rejects use of the diver outside allowed or when SCIP is not
// diving. allowed must be a subset of stagesInDive, the stages where
// SCIPinDive itself is defined, so the stage is read once.
func (d *Diver) active(op string, allowed stageSet) error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	m := Model{scip: d.scip}
	if err := m.query(op, allowed); err != nil {
		return err
	}
	if C.SCIPinDive(d.scip.raw) != 1 {
		return m.invalid(op, RetcodeInvalidCall, "SCIP is not in diving mode")
	}
	return nil
}

func (d *Diver) varOp(op string, v Variable, call func() C.SCIP_RETCODE) error {
	m := Model{scip: d.scip}
	if err := d.active(op, stagesInDive); err != nil {
		return err
	}
	if err := m.checkVars(op, v); err != nil {
		return err
	}
	return m.call(op, call())
}

func (d *Diver) rowOp(op string, r Row, call func() C.SCIP_RETCODE) error {
	m := Model{scip: d.scip}
	if err := d.active(op, stagesInDive); err != nil {
		return err
	}
	if err := m.checkHandle(op, "Row", r.raw != nil, r.scip, r.gen, false); err != nil {
		return err
	}
	return m.call(op, call())
}

// varGet is the shared shape of the dive getters, which SCIP defines only
// while solving.
func (d *Diver) varGet(op string, v Variable) {
	must(d.active(op, stagesSolving))
	must(Model{scip: d.scip}.checkVars(op, v))
}

// ChgVarLb changes a variable's lower bound in the dive. It panics on failure.
func (d *Diver) ChgVarLb(v Variable, newBound float64) { must(d.TryChgVarLb(v, newBound)) }

// TryChgVarLb is ChgVarLb returning an error instead of panicking.
func (d *Diver) TryChgVarLb(v Variable, newBound float64) error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	return d.varOp("Diver.ChgVarLb", v, func() C.SCIP_RETCODE { return C.SCIPchgVarLbDive(d.scip.raw, v.raw, C.double(newBound)) })
}

// ChgVarUb changes a variable's upper bound in the dive. It panics on failure.
func (d *Diver) ChgVarUb(v Variable, newBound float64) { must(d.TryChgVarUb(v, newBound)) }

// TryChgVarUb is ChgVarUb returning an error instead of panicking.
func (d *Diver) TryChgVarUb(v Variable, newBound float64) error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	return d.varOp("Diver.ChgVarUb", v, func() C.SCIP_RETCODE { return C.SCIPchgVarUbDive(d.scip.raw, v.raw, C.double(newBound)) })
}

// ChgVarObj changes a variable's objective coefficient in the dive. It
// panics on failure.
func (d *Diver) ChgVarObj(v Variable, newObj float64) { must(d.TryChgVarObj(v, newObj)) }

// TryChgVarObj is ChgVarObj returning an error instead of panicking.
func (d *Diver) TryChgVarObj(v Variable, newObj float64) error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	return d.varOp("Diver.ChgVarObj", v, func() C.SCIP_RETCODE { return C.SCIPchgVarObjDive(d.scip.raw, v.raw, C.double(newObj)) })
}

// SolveLp solves the dive LP with an optional iteration limit (<= 0 means
// unlimited) and reports whether it was solved to optimality.
func (d *Diver) SolveLp(iterationLimit int) (bool, error) {
	defer runtime.KeepAlive(d.scip)                                  // the C call must outlive the last Go use of the wrapper
	if err := d.active("Diver.SolveLp", stagesSolving); err != nil { // SCIPsolveDiveLP, SCIPgetLPSolstat
		return false, err
	}
	limit := C.int(iterationLimit)
	if iterationLimit <= 0 {
		limit = -1
	}
	var lperror, cutoff C.uint
	if err := (Model{scip: d.scip}).call("Diver.SolveLp", C.SCIPsolveDiveLP(d.scip.raw, limit, &lperror, &cutoff)); err != nil {
		return false, err
	}
	if lperror != 0 {
		return false, RetcodeLpError
	}
	return C.SCIPgetLPSolstat(d.scip.raw) == C.SCIP_LPSOLSTAT_OPTIMAL, nil
}

// AddRow adds a row to the dive LP. It panics on failure.
func (d *Diver) AddRow(r Row) { must(d.TryAddRow(r)) }

// TryAddRow is AddRow returning an error instead of panicking.
func (d *Diver) TryAddRow(r Row) error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	return d.rowOp("Diver.AddRow", r, func() C.SCIP_RETCODE { return C.SCIPaddRowDive(d.scip.raw, r.raw) })
}

// ChgRowLhs changes a row's left-hand side in the dive. It panics on failure.
func (d *Diver) ChgRowLhs(r Row, newLhs float64) { must(d.TryChgRowLhs(r, newLhs)) }

// TryChgRowLhs is ChgRowLhs returning an error instead of panicking.
func (d *Diver) TryChgRowLhs(r Row, newLhs float64) error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	return d.rowOp("Diver.ChgRowLhs", r, func() C.SCIP_RETCODE { return C.SCIPchgRowLhsDive(d.scip.raw, r.raw, C.double(newLhs)) })
}

// ChgRowRhs changes a row's right-hand side in the dive. It panics on failure.
func (d *Diver) ChgRowRhs(r Row, newRhs float64) { must(d.TryChgRowRhs(r, newRhs)) }

// TryChgRowRhs is ChgRowRhs returning an error instead of panicking.
func (d *Diver) TryChgRowRhs(r Row, newRhs float64) error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	return d.rowOp("Diver.ChgRowRhs", r, func() C.SCIP_RETCODE { return C.SCIPchgRowRhsDive(d.scip.raw, r.raw, C.double(newRhs)) })
}

// VarObj returns a variable's objective coefficient in the dive.
func (d *Diver) VarObj(v Variable) float64 {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	d.varGet("Diver.VarObj", v)
	return float64(C.SCIPgetVarObjDive(d.scip.raw, v.raw))
}

// VarLb returns a variable's lower bound in the dive.
func (d *Diver) VarLb(v Variable) float64 {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	d.varGet("Diver.VarLb", v)
	return float64(C.SCIPgetVarLbDive(d.scip.raw, v.raw))
}

// VarUb returns a variable's upper bound in the dive.
func (d *Diver) VarUb(v Variable) float64 {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	d.varGet("Diver.VarUb", v)
	return float64(C.SCIPgetVarUbDive(d.scip.raw, v.raw))
}

// LastDiveNode returns the number of the last node a dive was started at.
func (d *Diver) LastDiveNode() int {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	must(Model{scip: d.scip}.query("Diver.LastDiveNode", stagesInDive))
	return int(C.SCIPgetLastDivenode(d.scip.raw))
}

// ChgCutoffBound changes the cutoff bound in the dive. It panics on failure.
func (d *Diver) ChgCutoffBound(cutoff float64) { must(d.TryChgCutoffBound(cutoff)) }

// TryChgCutoffBound is ChgCutoffBound returning an error instead of panicking.
func (d *Diver) TryChgCutoffBound(cutoff float64) error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	if err := d.active("Diver.ChgCutoffBound", stagesInDive); err != nil {
		return err
	}
	return Model{scip: d.scip}.call("Diver.ChgCutoffBound", C.SCIPchgCutoffboundDive(d.scip.raw, C.double(cutoff)))
}

// End ends diving mode. It panics if SCIP is not diving.
func (d *Diver) End() { must(d.TryEnd()) }

// TryEnd ends diving mode, returning an error if SCIP is not diving.
func (d *Diver) TryEnd() error {
	defer runtime.KeepAlive(d.scip) // the C call must outlive the last Go use of the wrapper
	if err := d.active("Diver.End", stagesInDive); err != nil {
		return err
	}
	return Model{scip: d.scip}.call("Diver.End", C.SCIPendDive(d.scip.raw))
}
