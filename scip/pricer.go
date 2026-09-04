package scip

/*
#include "helpers.h"
*/
import "C"

import "runtime"

// Pricer is the interface for SCIP variable pricers.
type Pricer interface {
	// GenerateColumns generates negative reduced cost columns.
	//
	// farkas: if true, the pricer should generate columns to repair
	// feasibility of the LP.
	GenerateColumns(model Model, pricer PricerPlugin, farkas bool) PricerResult
}

// PricerResultState represents the possible states of a PricerResult.
type PricerResultState int

// Pricer result states.
const (
	PricerResultStateDidNotRun    PricerResultState = iota // The pricer did not run
	PricerResultStateFoundColumns                          // The pricer added new columns with negative reduced cost
	PricerResultStateNoColumns                             // No columns with negative reduced cost found (LP solution is optimal)
	PricerResultStateStopEarly                             // The pricer wants to perform early branching
)

// PricerResult is the result of a pricer.
type PricerResult struct {
	// State of the pricer result.
	State PricerResultState
	// LowerBound is an optional calculated lower bound on the objective
	// value of the current node.
	LowerBound *float64
}

func pricerStateToC(s PricerResultState) C.SCIP_RESULT {
	switch s {
	case PricerResultStateDidNotRun:
		return C.SCIP_DIDNOTRUN
	default:
		return C.SCIP_SUCCESS
	}
}

// PricerPlugin is a wrapper around a SCIP pricer object.
type PricerPlugin struct {
	raw  *C.SCIP_PRICER
	scip *Scip // keeps the owning instance alive and identifies it
}

// live panics with *Error unless the wrapper is usable; see handleErr.
func (h PricerPlugin) live(op string) {
	mustLive(op, "PricerPlugin", h.raw != nil, h.scip, genNone, true)
}

// Inner returns the internal raw pointer of the pricer.
func (p PricerPlugin) Inner() *C.SCIP_PRICER { return p.raw }

// Name returns the name of the pricer.
func (p PricerPlugin) Name() string {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PricerPlugin.Name")
	return goString(C.SCIPpricerGetName(p.raw))
}

// Desc returns the description of the pricer.
func (p PricerPlugin) Desc() string {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PricerPlugin.Desc")
	return goString(C.SCIPpricerGetDesc(p.raw))
}

// Priority returns the priority of the pricer.
func (p PricerPlugin) Priority() int32 {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PricerPlugin.Priority")
	return int32(C.SCIPpricerGetPriority(p.raw))
}

// IsDelayed returns the delay setting of the pricer.
func (p PricerPlugin) IsDelayed() bool {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PricerPlugin.IsDelayed")
	return C.SCIPpricerIsDelayed(p.raw) != 0
}

// IsActive returns whether the pricer is active.
func (p PricerPlugin) IsActive() bool {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PricerPlugin.IsActive")
	return C.SCIPpricerIsActive(p.raw) != 0
}
