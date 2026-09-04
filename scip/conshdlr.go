package scip

/*
#include "helpers.h"
*/
import "C"

import "runtime"

// Conshdlr is the interface for implementing custom constraint handlers. A
// handler may additionally implement ConshdlrEnfoPS, ConshdlrSepa,
// ConshdlrProp and Copyable.
type Conshdlr interface {
	// Check reports whether the (primal) solution satisfies the constraint.
	Check(model Model, conshdlr ConshdlrPlugin, solution Solution) bool
	// Enforce enforces the constraint for the current sub-problem's (LP)
	// solution.
	Enforce(model Model, conshdlr ConshdlrPlugin) ConshdlrResult
}

// ConshdlrEnfoPS is optionally implemented by a Conshdlr to enforce pseudo
// solutions, i.e. at nodes where no LP was solved. solInfeasible says an
// earlier handler already found the solution infeasible; objInfeasible says
// the pseudo solution's objective exceeds the cutoff bound, in which case
// ConshdlrResultDidNotRun is allowed.
type ConshdlrEnfoPS interface {
	EnforcePseudo(model Model, conshdlr ConshdlrPlugin, solInfeasible, objInfeasible bool) ConshdlrResult
}

// ConshdlrSepa is optionally implemented by a Conshdlr to separate the LP
// solution of the current node.
type ConshdlrSepa interface {
	SeparateLP(model Model, conshdlr ConshdlrPlugin) SeparationResult
}

// ConshdlrProp is optionally implemented by a Conshdlr to propagate variable
// domains before the LP of each node is solved.
type ConshdlrProp interface {
	Propagate(model Model, conshdlr ConshdlrPlugin) PropResult
}

// PropResult is the result of a propagation callback.
type PropResult int

// Propagation results.
const (
	PropResultDidNotRun  PropResult = iota // The propagator was skipped
	PropResultDidNotFind                   // Searched, but found no reductions
	PropResultReducedDom                   // The domain of a variable was reduced
	PropResultCutOff                       // The node is infeasible and can be cut off
	PropResultDelayed                      // Skipped, but should be called again
)

func propResultToC(r PropResult) C.SCIP_RESULT {
	switch r {
	case PropResultDidNotFind:
		return C.SCIP_DIDNOTFIND
	case PropResultReducedDom:
		return C.SCIP_REDUCEDDOM
	case PropResultCutOff:
		return C.SCIP_CUTOFF
	case PropResultDelayed:
		return C.SCIP_DELAYED
	default:
		return C.SCIP_DIDNOTRUN
	}
}

// ConshdlrResult is the result of enforcing a constraint handler.
type ConshdlrResult int

// Constraint handler results.
const (
	ConshdlrResultFeasible   ConshdlrResult = iota // The problem is feasible
	ConshdlrResultCutOff                           // The problem is infeasible
	ConshdlrResultConsAdded                        // Another constraint was added that resolves the infeasibility
	ConshdlrResultReducedDom                       // The domain of a variable was reduced
	ConshdlrResultSeparated                        // A cutting plane separated the LP solution
	ConshdlrResultSolveLP                          // Request to resolve the LP
	ConshdlrResultBranched                         // A branching was created
	ConshdlrResultInfeasible                       // Infeasible, nothing resolved it; SCIP has to branch
	ConshdlrResultDidNotRun                        // Skipped (EnforcePseudo only, when objInfeasible)
)

func conshdlrResultToC(r ConshdlrResult) C.SCIP_RESULT {
	switch r {
	case ConshdlrResultFeasible:
		return C.SCIP_FEASIBLE
	case ConshdlrResultCutOff:
		return C.SCIP_CUTOFF
	case ConshdlrResultConsAdded:
		return C.SCIP_CONSADDED
	case ConshdlrResultReducedDom:
		return C.SCIP_REDUCEDDOM
	case ConshdlrResultSeparated:
		return C.SCIP_SEPARATED
	case ConshdlrResultSolveLP:
		return C.SCIP_SOLVELP
	case ConshdlrResultBranched:
		return C.SCIP_BRANCHED
	case ConshdlrResultInfeasible:
		return C.SCIP_INFEASIBLE
	case ConshdlrResultDidNotRun:
		return C.SCIP_DIDNOTRUN
	default:
		return C.SCIP_FEASIBLE
	}
}

// ConshdlrPlugin is a wrapper for the internal SCIP constraint handler.
type ConshdlrPlugin struct {
	raw  *C.SCIP_CONSHDLR
	scip *Scip // keeps the owning instance alive and identifies it
}

// live panics with *Error unless the wrapper is usable; see handleErr.
func (h ConshdlrPlugin) live(op string) {
	mustLive(op, "ConshdlrPlugin", h.raw != nil, h.scip, genNone, true)
}

// Inner returns a raw pointer to the underlying SCIP_CONSHDLR.
func (c ConshdlrPlugin) Inner() *C.SCIP_CONSHDLR { return c.raw }

// Name returns the name of the constraint handler.
func (c ConshdlrPlugin) Name() string {
	defer runtime.KeepAlive(c.scip) // the C call must outlive the last Go use of the wrapper
	c.live("ConshdlrPlugin.Name")
	return goString(C.SCIPconshdlrGetName(c.raw))
}

// Desc returns the description of the constraint handler.
func (c ConshdlrPlugin) Desc() string {
	defer runtime.KeepAlive(c.scip) // the C call must outlive the last Go use of the wrapper
	c.live("ConshdlrPlugin.Desc")
	return goString(C.SCIPconshdlrGetDesc(c.raw))
}

// CreateEmptyRow creates an empty row for the constraint handler.
func (c ConshdlrPlugin) CreateEmptyRow(model Model, name string, lhs, rhs float64, local, modifiable, removable bool) (Row, error) {
	c.live("ConshdlrPlugin.CreateEmptyRow")
	return NewRow().Name(name).Bounds(lhs, rhs).Local(local).Modifiable(modifiable).Removable(removable).
		Source(SourceConshdlr(c)).TryAddTo(model)
}
