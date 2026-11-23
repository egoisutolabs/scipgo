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

// PropagatorPlugin is a read-only wrapper for a SCIP propagator plugin.
// Obtain one via Model.FindPropagator or Model.Propagators. It reports the
// propagator's priority, frequency, timing mask and call statistics.
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

// Priority returns the priority of the propagator.
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

// Timing returns the timing mask of the propagator.
func (p PropagatorPlugin) Timing() PropTiming {
	defer runtime.KeepAlive(p.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	p.live("PropagatorPlugin.Timing")
	return PropTiming(uint32(C.SCIPpropGetTimingmask(p.raw)))
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
