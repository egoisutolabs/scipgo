package scip

/*
#include "helpers.h"
*/
import "C"

import "runtime"

// PropTiming is the timing mask of a propagator. Values can be combined
// with the | operator. The constants match SCIP's SCIP_PROPTIMING_* flags.
type PropTiming uint32

// Propagator timing masks.
const (
	PropTimingNone         PropTiming = 0x000 // never call the propagator
	PropTimingBeforeLp     PropTiming = 0x001 // call before the LP is solved
	PropTimingDuringLpLoop PropTiming = 0x002 // call after each LP solve in the cut-and-price loop
	PropTimingAfterLpLoop  PropTiming = 0x004 // call after the cut-and-price loop finishes
	PropTimingAfterLpNode  PropTiming = 0x008 // call after a node with a solved LP is finished

	// PropTimingAlways calls the propagator at every timing point.
	PropTimingAlways PropTiming = PropTimingBeforeLp | PropTimingDuringLpLoop | PropTimingAfterLpLoop | PropTimingAfterLpNode
)

// Propagator is a domain propagator. SCIP calls Propagate during node
// processing, independent of any constraint handler. Implementations that
// hold mutable state should use a pointer receiver.
//
// A propagator may also implement PropagatorPresol, to run during
// presolving, and Copyable, to be copied into sub-SCIPs. The callback
// receives the same PropagatorPlugin that Propagators and FindPropagator
// return.
type Propagator interface {
	// Propagate tightens domains at the current node. The registered timing
	// mask is PropagatorPlugin.Timing; this call is not told which bit of
	// that mask fired.
	Propagate(model Model, prop PropagatorPlugin) PropResult
}

// PropagatorPresol is implemented by a Propagator that also reduces the
// problem during presolving. It is registered only when present.
//
// PresolveProp's result counts are the reductions this call performed.
// TightenVarLb and TightenVarUb do not update them: a bound change has to
// be reported in PresolResult.NChgBds or SCIP will not count it.
type PropagatorPresol interface {
	PresolveProp(model Model, prop PropagatorPlugin) PresolResult
}

// PresolTiming is a mask of presolving rounds a presolve callback joins.
// Combine values with |. NONE is 0x002, not 0: SCIP reserved 0x001 for a
// flag it no longer uses.
type PresolTiming uint32

// Presolving timing masks. The numeric values are SCIP's.
const (
	PresolTimingNone       PresolTiming = 0x002 // never call the presolve method
	PresolTimingFast       PresolTiming = 0x004 // fast methods
	PresolTimingMedium     PresolTiming = 0x008 // more expensive methods
	PresolTimingExhaustive PresolTiming = 0x010 // most expensive methods
	PresolTimingFinal      PresolTiming = 0x020 // final round, after the others
	PresolTimingAlways     PresolTiming = PresolTimingFast | PresolTimingMedium | PresolTimingExhaustive
)

// PresolStatus is the result code of a presolving callback.
type PresolStatus int

// Presolving result codes. PresolStatusDelayed is part of the code set the
// presolve headers document, but SCIP 10 rejects it from a propagator's
// presolve method (and from a presolver's exec): returning it fails the
// solve with a *CallbackPanic.
const (
	PresolStatusDidNotRun  PresolStatus = iota // Skipped
	PresolStatusDidNotFind                     // Searched, found no reduction
	PresolStatusSuccess                        // Found a reduction
	PresolStatusCutoff                         // The problem is infeasible
	PresolStatusUnbounded                      // The problem is unbounded
	PresolStatusDelayed                        // Skipped, but should be called again
)

func presolStatusToC(s PresolStatus) C.SCIP_RESULT {
	switch s {
	case PresolStatusDidNotFind:
		return C.SCIP_DIDNOTFIND
	case PresolStatusSuccess:
		return C.SCIP_SUCCESS
	case PresolStatusCutoff:
		return C.SCIP_CUTOFF
	case PresolStatusUnbounded:
		return C.SCIP_UNBOUNDED
	case PresolStatusDelayed:
		return C.SCIP_DELAYED
	default:
		return C.SCIP_DIDNOTRUN
	}
}

// PresolResult is what a presolving callback returns. Status is the SCIP
// result code. The counts are the reductions this call performed; the
// binding adds them to the running totals SCIP passes in, which is how a
// propagator (and, later, a presolver plugin) reports them. Zero counts
// with PresolStatusDidNotRun is the zero value and means "did nothing".
//
// Go presolver plugins are not in this version. They should return this
// same struct: the counts are the ones SCIP tracks for every presolving
// callback (nfixedvars, naggrvars, nchgbds, ndelconss, and the rest).
type PresolResult struct {
	Status       PresolStatus
	NFixedVars   int
	NAggrVars    int
	NChgVarTypes int
	NChgBds      int
	NAddHoles    int
	NDelConss    int
	NAddConss    int
	NUpgdConss   int
	NChgCoefs    int
	NChgSides    int
}

// PropagatorPlugin is a wrapper for a SCIP propagator. Obtain one via
// Model.FindPropagator or Model.Propagators, or receive it in a Propagator
// callback. It reports the propagator's priority, frequency, timing mask
// and call statistics.
type PropagatorPlugin struct {
	raw  *C.SCIP_PROP
	scip *Scip // keeps the owning instance alive and identifies it
}

// live panics with *Error unless the wrapper is usable; see handleErr.
func (h PropagatorPlugin) live(op string) {
	mustLive(op, "PropagatorPlugin", h.raw != nil, h.scip, genNone, true)
}

// Inner returns the raw pointer to the underlying SCIP_PROP.
func (p PropagatorPlugin) Inner() *C.SCIP_PROP { return p.raw }

// Name returns the name of the propagator.
func (p PropagatorPlugin) Name() string {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.Name")
	return goString(C.SCIPpropGetName(p.raw))
}

// Desc returns the description of the propagator.
func (p PropagatorPlugin) Desc() string {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.Desc")
	return goString(C.SCIPpropGetDesc(p.raw))
}

// Priority returns the priority of the propagator. Non-negative priorities
// run before constraint handlers, negative priorities after them.
func (p PropagatorPlugin) Priority() int32 {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.Priority")
	return int32(C.SCIPpropGetPriority(p.raw))
}

// Freq returns the calling frequency of the propagator; -1 means disabled.
func (p PropagatorPlugin) Freq() int32 {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.Freq")
	return int32(C.SCIPpropGetFreq(p.raw))
}

// SetFreq sets the calling frequency; -1 disables the propagator.
func (p PropagatorPlugin) SetFreq(freq int32) {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.SetFreq")
	C.SCIPpropSetFreq(p.raw, C.int(freq))
}

// Timing returns the timing mask of the propagator.
func (p PropagatorPlugin) Timing() PropTiming {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.Timing")
	return PropTiming(uint32(C.SCIPpropGetTimingmask(p.raw)))
}

// IsDelayed reports whether the propagator waits for a round in which no
// other propagator found a reduction.
func (p PropagatorPlugin) IsDelayed() bool {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.IsDelayed")
	return C.SCIPpropIsDelayed(p.raw) != 0
}

// WasDelayed reports whether the propagator was delayed at its last call.
func (p PropagatorPlugin) WasDelayed() bool {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.WasDelayed")
	return C.SCIPpropWasDelayed(p.raw) != 0
}

// PresolPriority returns the priority of the presolve hook.
func (p PropagatorPlugin) PresolPriority() int32 {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.PresolPriority")
	return int32(C.SCIPpropGetPresolPriority(p.raw))
}

// PresolTiming returns the timing mask of the presolve hook.
func (p PropagatorPlugin) PresolTiming() PresolTiming {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.PresolTiming")
	return PresolTiming(C.SCIPpropGetPresolTiming(p.raw))
}

// DoesPresolve reports whether the propagator has a presolve method. For a
// Go propagator this is true when it implements PropagatorPresol.
func (p PropagatorPlugin) DoesPresolve() bool {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.DoesPresolve")
	return C.SCIPpropDoesPresolve(p.raw) != 0
}

// NCalls returns the number of times the propagator was called.
func (p PropagatorPlugin) NCalls() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NCalls")
	return int(C.SCIPpropGetNCalls(p.raw))
}

// NCutoffs returns the number of times the propagator detected a cutoff.
func (p PropagatorPlugin) NCutoffs() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NCutoffs")
	return int(C.SCIPpropGetNCutoffs(p.raw))
}

// NDomredsFound returns the number of domain reductions the propagator found.
func (p PropagatorPlugin) NDomredsFound() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NDomredsFound")
	return int(C.SCIPpropGetNDomredsFound(p.raw))
}

// Time returns the time spent in the propagator, in seconds.
func (p PropagatorPlugin) Time() float64 {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.Time")
	return float64(C.SCIPpropGetTime(p.raw))
}

// NPresolCalls returns how many times the presolve method ran and tried to
// find a reduction.
func (p PropagatorPlugin) NPresolCalls() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NPresolCalls")
	return int(C.SCIPpropGetNPresolCalls(p.raw))
}

// PresolTime returns the seconds spent in the presolve method.
func (p PropagatorPlugin) PresolTime() float64 {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.PresolTime")
	return float64(C.SCIPpropGetPresolTime(p.raw))
}

// NFixedVars returns how many variables the presolve method reported fixed.
func (p PropagatorPlugin) NFixedVars() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NFixedVars")
	return int(C.SCIPpropGetNFixedVars(p.raw))
}

// NAggrVars returns how many variables the presolve method reported aggregated.
func (p PropagatorPlugin) NAggrVars() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NAggrVars")
	return int(C.SCIPpropGetNAggrVars(p.raw))
}

// NChgVarTypes returns how many variable-type changes the presolve method reported.
func (p PropagatorPlugin) NChgVarTypes() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NChgVarTypes")
	return int(C.SCIPpropGetNChgVarTypes(p.raw))
}

// NChgBds returns how many bound changes the presolve method reported.
func (p PropagatorPlugin) NChgBds() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NChgBds")
	return int(C.SCIPpropGetNChgBds(p.raw))
}

// NAddHoles returns how many domain holes the presolve method reported.
func (p PropagatorPlugin) NAddHoles() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NAddHoles")
	return int(C.SCIPpropGetNAddHoles(p.raw))
}

// NDelConss returns how many deleted constraints the presolve method reported.
func (p PropagatorPlugin) NDelConss() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NDelConss")
	return int(C.SCIPpropGetNDelConss(p.raw))
}

// NAddConss returns how many added constraints the presolve method reported.
func (p PropagatorPlugin) NAddConss() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NAddConss")
	return int(C.SCIPpropGetNAddConss(p.raw))
}

// NUpgdConss returns how many upgraded constraints the presolve method reported.
func (p PropagatorPlugin) NUpgdConss() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NUpgdConss")
	return int(C.SCIPpropGetNUpgdConss(p.raw))
}

// NChgCoefs returns how many coefficient changes the presolve method reported.
func (p PropagatorPlugin) NChgCoefs() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NChgCoefs")
	return int(C.SCIPpropGetNChgCoefs(p.raw))
}

// NChgSides returns how many side changes the presolve method reported.
func (p PropagatorPlugin) NChgSides() int {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.NChgSides")
	return int(C.SCIPpropGetNChgSides(p.raw))
}
