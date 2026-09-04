package scip

/*
#include "helpers.h"
*/
import "C"

// Diver gives access to methods allowed in diving mode. Obtain one from
// Model.StartDiving and call End when done (the Rust version ends
// diving on drop; Go has no destructors).
type Diver struct {
	scip *Scip
}

// ChgVarLb changes the lower bound of a variable in the current dive.
func (d *Diver) ChgVarLb(v Variable, newBound float64) {
	mustOK(C.SCIPchgVarLbDive(d.scip.raw, v.raw, C.double(newBound)))
}

// ChgVarUb changes the upper bound of a variable in the current dive.
func (d *Diver) ChgVarUb(v Variable, newBound float64) {
	mustOK(C.SCIPchgVarUbDive(d.scip.raw, v.raw, C.double(newBound)))
}

// ChgVarObj changes the objective value of a variable in the current dive.
func (d *Diver) ChgVarObj(v Variable, newObj float64) {
	mustOK(C.SCIPchgVarObjDive(d.scip.raw, v.raw, C.double(newObj)))
}

// SolveLp solves the diving LP. iterationLimit <= 0 means no limit. Returns
// whether the LP was solved to optimality.
func (d *Diver) SolveLp(iterationLimit int) (bool, error) {
	limit := C.int(iterationLimit)
	if iterationLimit <= 0 {
		limit = -1
	}
	var lperror C.uint
	var cutoff C.uint
	// The fourth argument of SCIPsolveDiveLP is `cutoff` (the diving LP was
	// infeasible or hit the objective limit), not "LP solved".
	if err := retcodeError(C.SCIPsolveDiveLP(d.scip.raw, limit, &lperror, &cutoff)); err != nil {
		return false, err
	}
	if lperror != 0 {
		return false, RetcodeLpError
	}
	return C.SCIPgetLPSolstat(d.scip.raw) == C.SCIP_LPSOLSTAT_OPTIMAL, nil
}

// AddRow adds a row to the diving LP.
func (d *Diver) AddRow(r Row) {
	mustOK(C.SCIPaddRowDive(d.scip.raw, r.raw))
}

// ChgRowLhs changes a row's lhs in the diving LP.
func (d *Diver) ChgRowLhs(r Row, newLhs float64) {
	mustOK(C.SCIPchgRowLhsDive(d.scip.raw, r.raw, C.double(newLhs)))
}

// ChgRowRhs changes a row's rhs in the diving LP.
func (d *Diver) ChgRowRhs(r Row, newRhs float64) {
	mustOK(C.SCIPchgRowRhsDive(d.scip.raw, r.raw, C.double(newRhs)))
}

// VarObj gets the variable objective value in the diving LP.
func (d *Diver) VarObj(v Variable) float64 { return float64(C.SCIPgetVarObjDive(d.scip.raw, v.raw)) }

// VarLb gets the variable lower bound in the diving LP.
func (d *Diver) VarLb(v Variable) float64 { return float64(C.SCIPgetVarLbDive(d.scip.raw, v.raw)) }

// VarUb gets the variable upper bound in the diving LP.
func (d *Diver) VarUb(v Variable) float64 { return float64(C.SCIPgetVarUbDive(d.scip.raw, v.raw)) }

// LastDiveNode gets the last branch-and-bound node (in the current run)
// number where diving was started.
func (d *Diver) LastDiveNode() int { return int(C.SCIPgetLastDivenode(d.scip.raw)) }

// ChgCutoffBound changes the cutoff bound in the diving LP.
func (d *Diver) ChgCutoffBound(cutoff float64) {
	mustOK(C.SCIPchgCutoffboundDive(d.scip.raw, C.double(cutoff)))
}

// End ends diving mode. Must be called exactly once after StartDiving.
func (d *Diver) End() { must(d.TryEnd()) }

// TryEnd ends diving mode, returning an error if SCIP is not diving.
func (d *Diver) TryEnd() error {
	m := Model{scip: d.scip}
	if C.SCIPinDive(d.scip.raw) != 1 {
		return m.invalid("Diver.End", RetcodeInvalidCall, "SCIP is not in diving mode")
	}
	return m.call("Diver.End", C.SCIPendDive(d.scip.raw))
}
