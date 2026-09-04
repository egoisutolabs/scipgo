package scip

/*
#include "helpers.h"
*/
import "C"

import "runtime"

// Prober drives SCIP's probing mode, started with Model.StartProbing and
// ended with End. Every method reports "not probing", a freed model or a
// wrong stage as *Error: the plain forms panic with it, the Try forms
// return it.
type Prober struct {
	scip *Scip
}

// active rejects use of the prober outside allowed or when SCIP is not
// probing. allowed must be a subset of stagesTransformed, the stages where
// SCIPinProbing itself is defined, so the stage is read once.
func (p *Prober) active(op string, allowed stageSet) error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	m := Model{scip: p.scip}
	if err := m.query(op, allowed); err != nil {
		return err
	}
	if C.SCIPinProbing(p.scip.raw) != 1 {
		return m.invalid(op, RetcodeInvalidCall, "SCIP is not in probing mode")
	}
	return nil
}

// varOp runs a probing call on a variable after the shared checks.
func (p *Prober) varOp(op string, v Variable, call func() C.SCIP_RETCODE) error {
	m := Model{scip: p.scip}
	if err := p.active(op, stagesTransformed); err != nil {
		return err
	}
	if err := m.checkVars(op, v); err != nil {
		return err
	}
	return m.call(op, call())
}

// NewNode creates a new probing sub node. It panics on failure.
func (p *Prober) NewNode() { must(p.TryNewNode()) }

// TryNewNode creates a new probing sub node, returning an error on failure.
func (p *Prober) TryNewNode() error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	if err := p.active("Prober.NewNode", stagesTransformed); err != nil {
		return err
	}
	return Model{scip: p.scip}.call("Prober.NewNode", C.SCIPnewProbingNode(p.scip.raw))
}

// Depth returns the current probing depth.
func (p *Prober) Depth() int {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	must(p.active("Prober.Depth", stagesProbingDepth))
	return int(C.SCIPgetProbingDepth(p.scip.raw))
}

// Backtrack undoes all probing changes above depth. It panics on failure.
func (p *Prober) Backtrack(depth int) { must(p.TryBacktrack(depth)) }

// TryBacktrack undoes all probing changes above the given depth, which must
// be non-negative and at most the current probing depth (equal is a no-op).
func (p *Prober) TryBacktrack(depth int) error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	m := Model{scip: p.scip}
	if depth < 0 {
		return m.invalid("Prober.Backtrack", RetcodeInvalidData, "negative depth")
	}
	if err := p.active("Prober.Backtrack", stagesProbingDepth); err != nil { // SCIPgetProbingDepth
		return err
	}
	if depth > int(C.SCIPgetProbingDepth(p.scip.raw)) {
		return m.invalid("Prober.Backtrack", RetcodeInvalidData, "depth exceeds the current probing depth")
	}
	return m.call("Prober.Backtrack", C.SCIPbacktrackProbing(p.scip.raw, C.int(depth)))
}

// ChgVarLb changes a variable's lower bound in probing. It panics on failure.
func (p *Prober) ChgVarLb(v Variable, newBound float64) { must(p.TryChgVarLb(v, newBound)) }

// TryChgVarLb is ChgVarLb returning an error instead of panicking.
func (p *Prober) TryChgVarLb(v Variable, newBound float64) error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	return p.varOp("Prober.ChgVarLb", v, func() C.SCIP_RETCODE { return C.SCIPchgVarLbProbing(p.scip.raw, v.raw, C.double(newBound)) })
}

// ChgVarUb changes a variable's upper bound in probing. It panics on failure.
func (p *Prober) ChgVarUb(v Variable, newBound float64) { must(p.TryChgVarUb(v, newBound)) }

// TryChgVarUb is ChgVarUb returning an error instead of panicking.
func (p *Prober) TryChgVarUb(v Variable, newBound float64) error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	return p.varOp("Prober.ChgVarUb", v, func() C.SCIP_RETCODE { return C.SCIPchgVarUbProbing(p.scip.raw, v.raw, C.double(newBound)) })
}

// VarObj returns a variable's objective coefficient in probing.
func (p *Prober) VarObj(v Variable) float64 {
	defer runtime.KeepAlive(p.scip)                // the C call must outlive the last Go use of the wrapper
	must(p.active("Prober.VarObj", stagesSolving)) // SCIPgetVarObjProbing
	must(Model{scip: p.scip}.checkVars("Prober.VarObj", v))
	return float64(C.SCIPgetVarObjProbing(p.scip.raw, v.raw))
}

// FixVar fixes a variable in probing. It panics on failure.
func (p *Prober) FixVar(v Variable, value float64) { must(p.TryFixVar(v, value)) }

// TryFixVar is FixVar returning an error instead of panicking.
func (p *Prober) TryFixVar(v Variable, value float64) error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	return p.varOp("Prober.FixVar", v, func() C.SCIP_RETCODE { return C.SCIPfixVarProbing(p.scip.raw, v.raw, C.double(value)) })
}

// ChgVarObj changes a variable's objective coefficient in probing. It panics
// on failure.
func (p *Prober) ChgVarObj(v Variable, newObj float64) { must(p.TryChgVarObj(v, newObj)) }

// TryChgVarObj is ChgVarObj returning an error instead of panicking.
func (p *Prober) TryChgVarObj(v Variable, newObj float64) error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	return p.varOp("Prober.ChgVarObj", v, func() C.SCIP_RETCODE { return C.SCIPchgVarObjProbing(p.scip.raw, v.raw, C.double(newObj)) })
}

// IsObjChanged reports whether the objective was changed in probing.
func (p *Prober) IsObjChanged() bool {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	must(p.active("Prober.IsObjChanged", stagesTransformed))
	return C.SCIPisObjChangedProbing(p.scip.raw) != 0
}

// Propagate applies domain propagation in probing, for at most maxRounds
// rounds (any value <= 0 means unlimited). It reports whether the node was
// cut off and how many reductions were found. It panics on failure.
func (p *Prober) Propagate(maxRounds int) (bool, int) {
	cutoff, n, err := p.TryPropagate(maxRounds)
	must(err)
	return cutoff, n
}

// TryPropagate is Propagate returning an error instead of panicking.
func (p *Prober) TryPropagate(maxRounds int) (cutoff bool, nReductions int, err error) {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	if err := p.active("Prober.Propagate", stagesTransformed); err != nil {
		return false, 0, err
	}
	rounds := C.int(maxRounds)
	if maxRounds <= 0 {
		rounds = -1
	}
	var ccutoff C.uint
	var cn C.longlong
	if err := (Model{scip: p.scip}).call("Prober.Propagate", C.SCIPpropagateProbing(p.scip.raw, rounds, &ccutoff, &cn)); err != nil {
		return false, 0, err
	}
	return ccutoff != 0, int(cn), nil
}

// PropagateImplications applies implication propagation in probing and
// reports whether the node was cut off. It panics on failure.
func (p *Prober) PropagateImplications() bool {
	cutoff, err := p.TryPropagateImplications()
	must(err)
	return cutoff
}

// TryPropagateImplications is PropagateImplications returning an error
// instead of panicking.
func (p *Prober) TryPropagateImplications() (bool, error) {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	if err := p.active("Prober.PropagateImplications", stagesTransformed); err != nil {
		return false, err
	}
	var cutoff C.uint
	if err := (Model{scip: p.scip}).call("Prober.PropagateImplications", C.SCIPpropagateProbingImplications(p.scip.raw, &cutoff)); err != nil {
		return false, err
	}
	return cutoff != 0, nil
}

// SolveLp solves the probing LP with an optional iteration limit (<= 0 means
// unlimited) and reports whether the node was cut off.
func (p *Prober) SolveLp(iterationLimit int) (bool, error) {
	defer runtime.KeepAlive(p.scip)                                   // the C call must outlive the last Go use of the wrapper
	if err := p.active("Prober.SolveLp", stagesSolving); err != nil { // SCIPisLPConstructed
		return false, err
	}
	if !p.scip.isLPConstructed() {
		if _, err := p.scip.constructLP(); err != nil {
			return false, err
		}
	}
	limit := C.int(iterationLimit)
	if iterationLimit <= 0 {
		limit = -1
	}
	var lperror, cutoff C.uint
	if err := (Model{scip: p.scip}).call("Prober.SolveLp", C.SCIPsolveProbingLP(p.scip.raw, limit, &lperror, &cutoff)); err != nil {
		return false, err
	}
	if lperror != 0 {
		return false, RetcodeLpError
	}
	return cutoff != 0, nil
}

// SolveLpWithPricing solves the probing LP with pricing, for at most
// maxPricingRounds rounds (<= 0 means unlimited), and reports whether the
// node was cut off.
func (p *Prober) SolveLpWithPricing(maxPricingRounds int) (bool, error) {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	if err := p.active("Prober.SolveLpWithPricing", stagesSolving); err != nil {
		return false, err
	}
	if !p.scip.isLPConstructed() {
		if _, err := p.scip.constructLP(); err != nil {
			return false, err
		}
	}
	rounds := C.int(maxPricingRounds)
	if maxPricingRounds <= 0 {
		rounds = -1
	}
	var lperror, cutoff C.uint
	const pretendAtRoot C.uint = 0
	const displayInfo C.uint = 1
	if err := (Model{scip: p.scip}).call("Prober.SolveLpWithPricing", C.SCIPsolveProbingLPWithPricing(p.scip.raw, pretendAtRoot, displayInfo,
		rounds, &lperror, &cutoff)); err != nil {
		return false, err
	}
	if lperror != 0 {
		return false, RetcodeLpError
	}
	return cutoff != 0, nil
}

// AddRow adds a row to the probing LP. It panics on failure.
func (p *Prober) AddRow(r Row) { must(p.TryAddRow(r)) }

// TryAddRow is AddRow returning an error instead of panicking.
func (p *Prober) TryAddRow(r Row) error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	m := Model{scip: p.scip}
	if err := p.active("Prober.AddRow", stagesTransformed); err != nil {
		return err
	}
	if err := m.checkHandle("Prober.AddRow", "Row", r.raw != nil, r.scip, r.gen, false); err != nil {
		return err
	}
	return m.call("Prober.AddRow", C.SCIPaddRowProbing(p.scip.raw, r.raw))
}

// End ends probing mode. It panics if SCIP is not probing.
func (p *Prober) End() { must(p.TryEnd()) }

// TryEnd ends probing mode, returning an error if SCIP is not probing.
func (p *Prober) TryEnd() error {
	defer runtime.KeepAlive(p.scip) // the C call must outlive the last Go use of the wrapper
	if err := p.active("Prober.End", stagesTransformed); err != nil {
		return err
	}
	return Model{scip: p.scip}.call("Prober.End", C.SCIPendProbing(p.scip.raw))
}

// InProbing reports whether the model is in probing mode.
func InProbing(m Model) bool {
	defer runtime.KeepAlive(m.scip) // the C call must outlive the last Go use of the wrapper
	must(m.guard("InProbing"))
	if !stagesTransformed.has(m.scip.stage()) {
		return false
	}
	return C.SCIPinProbing(m.scip.raw) != 0
}

// InDive reports whether the model is in diving mode.
func InDive(m Model) bool {
	defer runtime.KeepAlive(m.scip) // the C call must outlive the last Go use of the wrapper
	must(m.guard("InDive"))
	if !stagesInDive.has(m.scip.stage()) {
		return false
	}
	return C.SCIPinDive(m.scip.raw) != 0
}
