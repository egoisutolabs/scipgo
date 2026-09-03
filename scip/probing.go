package scip

/*
#include "helpers.h"
*/
import "C"

// Prober gives access to methods allowed in probing mode. Obtain one from
// Model.StartProbing and call End when done (the Rust version ends
// probing on drop; Go has no destructors).
type Prober struct {
	scip *Scip
}

// NewNode creates a new probing (sub-)node, whose changes can be undone by
// backtracking to a higher node in the probing path with a call to Backtrack.
func (p *Prober) NewNode() {
	mustOK(C.SCIPnewProbingNode(p.scip.raw))
}

// Depth returns the current probing depth.
func (p *Prober) Depth() int { return int(C.SCIPgetProbingDepth(p.scip.raw)) }

// Backtrack undoes all changes applied in probing up to (and including) the
// given probing depth.
func (p *Prober) Backtrack(depth int) {
	if depth >= p.Depth() {
		panic("probing depth must be less than the current probing depth")
	}
	mustOK(C.SCIPbacktrackProbing(p.scip.raw, C.int(depth)))
}

// ChgVarLb changes the lower bound of a variable in the current probing node.
func (p *Prober) ChgVarLb(v Variable, newBound float64) {
	mustOK(C.SCIPchgVarLbProbing(p.scip.raw, v.raw, C.double(newBound)))
}

// ChgVarUb changes the upper bound of a variable in the current probing node.
func (p *Prober) ChgVarUb(v Variable, newBound float64) {
	mustOK(C.SCIPchgVarUbProbing(p.scip.raw, v.raw, C.double(newBound)))
}

// VarObj retrieves the objective value of a variable in the current probing node.
func (p *Prober) VarObj(v Variable) float64 {
	return float64(C.SCIPgetVarObjProbing(p.scip.raw, v.raw))
}

// FixVar fixes a variable to a value in the current probing node.
func (p *Prober) FixVar(v Variable, value float64) {
	mustOK(C.SCIPfixVarProbing(p.scip.raw, v.raw, C.double(value)))
}

// ChgVarObj changes the objective value of a variable in the current probing
// node.
func (p *Prober) ChgVarObj(v Variable, newObj float64) {
	mustOK(C.SCIPchgVarObjProbing(p.scip.raw, v.raw, C.double(newObj)))
}

// IsObjChanged returns whether the probing subproblem objective function has
// been changed.
func (p *Prober) IsObjChanged() bool { return C.SCIPisObjChangedProbing(p.scip.raw) != 0 }

// Propagate applies domain propagation on the probing subproblem.
// maxRounds <= 0 means no limit. Returns (cutoff, nReductions).
func (p *Prober) Propagate(maxRounds int) (bool, int) {
	rounds := C.int(maxRounds)
	if maxRounds <= 0 {
		rounds = -1
	}
	var cutoff C.uint
	var nReductions C.longlong
	mustOK(C.SCIPpropagateProbing(p.scip.raw, rounds, &cutoff, &nReductions))
	return cutoff != 0, int(nReductions)
}

// PropagateImplications applies domain propagation of the binary variables
// fixed at the current probing node that are triggered by the implication
// graph and the clique table. Returns whether a cutoff was detected.
func (p *Prober) PropagateImplications() bool {
	var cutoff C.uint
	mustOK(C.SCIPpropagateProbingImplications(p.scip.raw, &cutoff))
	return cutoff != 0
}

// SolveLp solves the probing subproblem; the solution can be accessed with
// Model.CurrentVal. iterationLimit <= 0 means no limit. Returns
// whether a cutoff was detected.
func (p *Prober) SolveLp(iterationLimit int) (bool, error) {
	if !p.scip.isLPConstructed() {
		if _, err := p.scip.constructLP(); err != nil {
			return false, err
		}
	}
	limit := C.int(iterationLimit)
	if iterationLimit <= 0 {
		limit = -1
	}
	var lperror C.uint
	var cutoff C.uint
	if err := retcodeError(C.SCIPsolveProbingLP(p.scip.raw, limit, &lperror, &cutoff)); err != nil {
		return false, err
	}
	if lperror != 0 {
		return false, RetcodeLpError
	}
	return cutoff != 0, nil
}

// SolveLpWithPricing solves the probing subproblem with pricing.
// maxPricingRounds <= 0 means no limit. Returns whether a cutoff was
// detected.
func (p *Prober) SolveLpWithPricing(maxPricingRounds int) (bool, error) {
	if !p.scip.isLPConstructed() {
		if _, err := p.scip.constructLP(); err != nil {
			return false, err
		}
	}
	rounds := C.int(maxPricingRounds)
	if maxPricingRounds <= 0 {
		rounds = -1
	}
	var lperror C.uint
	var cutoff C.uint
	const pretendAtRoot C.uint = 0
	const displayInfo C.uint = 1
	if err := retcodeError(C.SCIPsolveProbingLPWithPricing(p.scip.raw, pretendAtRoot, displayInfo,
		rounds, &lperror, &cutoff)); err != nil {
		return false, err
	}
	if lperror != 0 {
		return false, RetcodeLpError
	}
	return cutoff != 0, nil
}

// AddRow adds a row to the probing subproblem.
func (p *Prober) AddRow(r Row) {
	mustOK(C.SCIPaddRowProbing(p.scip.raw, r.raw))
}

// End ends probing mode. Must be called exactly once after StartProbing.
func (p *Prober) End() {
	if C.SCIPinProbing(p.scip.raw) != 1 {
		panic("SCIP is expected to be in probing mode before Prober.End is called.")
	}
	C.SCIPendProbing(p.scip.raw)
}

// InProbing reports whether SCIP is currently in probing mode.
func InProbing(m Model) bool { return C.SCIPinProbing(m.scip.raw) != 0 }

// InDive reports whether SCIP is currently in diving mode.
func InDive(m Model) bool { return C.SCIPinDive(m.scip.raw) != 0 }
