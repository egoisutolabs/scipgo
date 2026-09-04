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

// active rejects use of the diver when SCIP is not diving; SCIPinDive itself
// aborts outside stagesInDive, so the stage is checked first.
func (d *Diver) active(op string) error {
	m := Model{scip: d.scip}
	if err := m.query(op, stagesInDive); err != nil {
		return err
	}
	if C.SCIPinDive(d.scip.raw) != 1 {
		return m.invalid(op, RetcodeInvalidCall, "SCIP is not in diving mode")
	}
	return nil
}

// ChgVarLb changes the lower bound of a variable in the current dive.
func (d *Diver) ChgVarLb(v Variable, newBound float64) { must(d.TryChgVarLb(v, newBound)) }

// TryChgVarLb is ChgVarLb returning an error instead of panicking.
func (d *Diver) TryChgVarLb(v Variable, newBound float64) error {
	m := Model{scip: d.scip}
	if err := d.active("Diver.ChgVarLb"); err != nil {
		return err
	}
	if err := m.checkVars("Diver.ChgVarLb", v); err != nil {
		return err
	}
	return m.call("Diver.ChgVarLb", C.SCIPchgVarLbDive(d.scip.raw, v.raw, C.double(newBound)))
}

// ChgVarUb changes the upper bound of a variable in the current dive.
func (d *Diver) ChgVarUb(v Variable, newBound float64) { must(d.TryChgVarUb(v, newBound)) }

// TryChgVarUb is ChgVarUb returning an error instead of panicking.
func (d *Diver) TryChgVarUb(v Variable, newBound float64) error {
	m := Model{scip: d.scip}
	if err := d.active("Diver.ChgVarUb"); err != nil {
		return err
	}
	if err := m.checkVars("Diver.ChgVarUb", v); err != nil {
		return err
	}
	return m.call("Diver.ChgVarUb", C.SCIPchgVarUbDive(d.scip.raw, v.raw, C.double(newBound)))
}

// ChgVarObj changes the objective value of a variable in the current dive.
func (d *Diver) ChgVarObj(v Variable, newObj float64) { must(d.TryChgVarObj(v, newObj)) }

// TryChgVarObj is ChgVarObj returning an error instead of panicking.
func (d *Diver) TryChgVarObj(v Variable, newObj float64) error {
	m := Model{scip: d.scip}
	if err := d.active("Diver.ChgVarObj"); err != nil {
		return err
	}
	if err := m.checkVars("Diver.ChgVarObj", v); err != nil {
		return err
	}
	return m.call("Diver.ChgVarObj", C.SCIPchgVarObjDive(d.scip.raw, v.raw, C.double(newObj)))
}

// SolveLp solves the diving LP. iterationLimit <= 0 means no limit. Returns
// whether the LP was solved to optimality.
func (d *Diver) SolveLp(iterationLimit int) (bool, error) {
	if err := d.active("Diver.SolveLp"); err != nil {
		return false, err
	}
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
func (d *Diver) AddRow(r Row) { must(d.TryAddRow(r)) }

// TryAddRow is AddRow returning an error instead of panicking.
func (d *Diver) TryAddRow(r Row) error {
	m := Model{scip: d.scip}
	if err := d.active("Diver.AddRow"); err != nil {
		return err
	}
	if err := m.checkHandle("Diver.AddRow", "Row", r.raw != nil, r.scip); err != nil {
		return err
	}
	return m.call("Diver.AddRow", C.SCIPaddRowDive(d.scip.raw, r.raw))
}

// ChgRowLhs changes a row's lhs in the diving LP.
func (d *Diver) ChgRowLhs(r Row, newLhs float64) { must(d.TryChgRowLhs(r, newLhs)) }

// TryChgRowLhs is ChgRowLhs returning an error instead of panicking.
func (d *Diver) TryChgRowLhs(r Row, newLhs float64) error {
	m := Model{scip: d.scip}
	if err := d.active("Diver.ChgRowLhs"); err != nil {
		return err
	}
	if err := m.checkHandle("Diver.ChgRowLhs", "Row", r.raw != nil, r.scip); err != nil {
		return err
	}
	return m.call("Diver.ChgRowLhs", C.SCIPchgRowLhsDive(d.scip.raw, r.raw, C.double(newLhs)))
}

// ChgRowRhs changes a row's rhs in the diving LP.
func (d *Diver) ChgRowRhs(r Row, newRhs float64) { must(d.TryChgRowRhs(r, newRhs)) }

// TryChgRowRhs is ChgRowRhs returning an error instead of panicking.
func (d *Diver) TryChgRowRhs(r Row, newRhs float64) error {
	m := Model{scip: d.scip}
	if err := d.active("Diver.ChgRowRhs"); err != nil {
		return err
	}
	if err := m.checkHandle("Diver.ChgRowRhs", "Row", r.raw != nil, r.scip); err != nil {
		return err
	}
	return m.call("Diver.ChgRowRhs", C.SCIPchgRowRhsDive(d.scip.raw, r.raw, C.double(newRhs)))
}

// VarObj gets the variable objective value in the diving LP.
func (d *Diver) VarObj(v Variable) float64 {
	must(d.active("Diver.VarObj"))
	mustStage("Diver.VarObj", d.scip, stagesSolving)
	must(Model{scip: d.scip}.checkVars("Diver.VarObj", v))
	return float64(C.SCIPgetVarObjDive(d.scip.raw, v.raw))
}

// VarLb gets the variable lower bound in the diving LP.
func (d *Diver) VarLb(v Variable) float64 {
	must(d.active("Diver.VarLb"))
	mustStage("Diver.VarLb", d.scip, stagesSolving)
	must(Model{scip: d.scip}.checkVars("Diver.VarLb", v))
	return float64(C.SCIPgetVarLbDive(d.scip.raw, v.raw))
}

// VarUb gets the variable upper bound in the diving LP.
func (d *Diver) VarUb(v Variable) float64 {
	must(d.active("Diver.VarUb"))
	mustStage("Diver.VarUb", d.scip, stagesSolving)
	must(Model{scip: d.scip}.checkVars("Diver.VarUb", v))
	return float64(C.SCIPgetVarUbDive(d.scip.raw, v.raw))
}

// LastDiveNode gets the last branch-and-bound node (in the current run)
// number where diving was started.
func (d *Diver) LastDiveNode() int {
	must(Model{scip: d.scip}.query("Diver.LastDiveNode", stagesInDive))
	return int(C.SCIPgetLastDivenode(d.scip.raw))
}

// ChgCutoffBound changes the cutoff bound in the diving LP.
func (d *Diver) ChgCutoffBound(cutoff float64) { must(d.TryChgCutoffBound(cutoff)) }

// TryChgCutoffBound is ChgCutoffBound returning an error instead of panicking.
func (d *Diver) TryChgCutoffBound(cutoff float64) error {
	if err := d.active("Diver.ChgCutoffBound"); err != nil {
		return err
	}
	return Model{scip: d.scip}.call("Diver.ChgCutoffBound", C.SCIPchgCutoffboundDive(d.scip.raw, C.double(cutoff)))
}

// End ends diving mode. Must be called exactly once after StartDiving.
func (d *Diver) End() { must(d.TryEnd()) }

// TryEnd ends diving mode, returning an error if SCIP is not diving.
func (d *Diver) TryEnd() error {
	if err := d.active("Diver.End"); err != nil {
		return err
	}
	return Model{scip: d.scip}.call("Diver.End", C.SCIPendDive(d.scip.raw))
}
