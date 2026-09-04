package scip

/*
#include "helpers.h"
*/
import "C"

import "runtime"

// Eventhdlr is the interface used to define custom event handlers.
type Eventhdlr interface {
	// GetEventMask returns the type of the events the handler wants to catch.
	GetEventMask() EventMask
	// Execute executes the event handler.
	Execute(model Model, eventhdlr EventhdlrPlugin, event Event)
}

// EventMask represents different states or actions within an optimization
// problem. Values can be combined with the | operator.
type EventMask uint64

// Event types (bitmask values).
const (
	EventMaskDisabled        EventMask = 0x000000000
	EventMaskVarAdded        EventMask = 0x000000001
	EventMaskVarDeleted      EventMask = 0x000000002
	EventMaskVarFixed        EventMask = 0x000000004
	EventMaskVarUnlocked     EventMask = 0x000000008
	EventMaskObjChanged      EventMask = 0x000000010
	EventMaskGlbChanged      EventMask = 0x000000020
	EventMaskGubChanged      EventMask = 0x000000040
	EventMaskLbTightened     EventMask = 0x000000080
	EventMaskLbRelaxed       EventMask = 0x000000100
	EventMaskUbTightened     EventMask = 0x000000200
	EventMaskUbRelaxed       EventMask = 0x000000400
	EventMaskGholeAdded      EventMask = 0x000000800
	EventMaskGholeRemoved    EventMask = 0x000001000
	EventMaskLholeAdded      EventMask = 0x000002000
	EventMaskLholeRemoved    EventMask = 0x000004000
	EventMaskImplAdded       EventMask = 0x000008000
	EventMaskTypeChanged     EventMask = 0x000010000
	EventMaskImplTypeChanged EventMask = 0x000020000
	EventMaskPresolveRound   EventMask = 0x000040000
	EventMaskNodeFocused     EventMask = 0x000080000
	EventMaskNodeFeasible    EventMask = 0x000100000
	EventMaskNodeInfeasible  EventMask = 0x000200000
	EventMaskNodeBranched    EventMask = 0x000400000
	EventMaskNodeDelete      EventMask = 0x000800000
	EventMaskDualBoundImpr   EventMask = 0x001000000
	EventMaskFirstLpSolved   EventMask = 0x002000000
	EventMaskLpSolved        EventMask = 0x004000000
	EventMaskPoorSolFound    EventMask = 0x008000000
	EventMaskBestSolFound    EventMask = 0x010000000
	EventMaskRowAddedSepa    EventMask = 0x020000000
	EventMaskRowDeletedSepa  EventMask = 0x040000000
	EventMaskRowAddedLp      EventMask = 0x080000000
	EventMaskRowDeletedLp    EventMask = 0x100000000
	EventMaskRowCoefChanged  EventMask = 0x200000000
	EventMaskRowConstChanged EventMask = 0x400000000
	EventMaskRowSideChanged  EventMask = 0x800000000
	EventMaskSync            EventMask = 0x1000000000

	// Combined event masks.
	EventMaskGbdChanged     EventMask = EventMaskGlbChanged | EventMaskGubChanged
	EventMaskLbChanged      EventMask = EventMaskLbTightened | EventMaskLbRelaxed
	EventMaskUbChanged      EventMask = EventMaskUbTightened | EventMaskUbRelaxed
	EventMaskBoundTightened EventMask = EventMaskLbTightened | EventMaskUbTightened
	EventMaskBoundRelaxed   EventMask = EventMaskLbRelaxed | EventMaskUbRelaxed
	EventMaskBoundChanged   EventMask = EventMaskLbChanged | EventMaskUbChanged
	EventMaskGholeChanged   EventMask = EventMaskGholeAdded | EventMaskGholeRemoved
	EventMaskLholeChanged   EventMask = EventMaskLholeAdded | EventMaskLholeRemoved
	EventMaskHoleChanged    EventMask = EventMaskGholeChanged | EventMaskLholeChanged
	EventMaskDomChanged     EventMask = EventMaskBoundChanged | EventMaskHoleChanged
	EventMaskVarChanged     EventMask = EventMaskVarFixed | EventMaskVarUnlocked | EventMaskObjChanged |
		EventMaskGbdChanged | EventMaskDomChanged | EventMaskImplAdded | EventMaskVarDeleted | EventMaskTypeChanged
	EventMaskVarEvent   EventMask = EventMaskVarAdded | EventMaskVarChanged | EventMaskTypeChanged
	EventMaskNodeSolved EventMask = EventMaskNodeFeasible | EventMaskNodeInfeasible | EventMaskNodeBranched
	EventMaskNodeEvent  EventMask = EventMaskNodeFocused | EventMaskNodeSolved
	EventMaskLpEvent    EventMask = EventMaskFirstLpSolved | EventMaskLpSolved
	EventMaskSolFound   EventMask = EventMaskPoorSolFound | EventMaskBestSolFound
	EventMaskSolEvent   EventMask = EventMaskSolFound
	EventMaskRowChanged EventMask = EventMaskRowCoefChanged | EventMaskRowConstChanged | EventMaskRowSideChanged
	EventMaskRowEvent   EventMask = EventMaskRowAddedSepa | EventMaskRowDeletedSepa |
		EventMaskRowAddedLp | EventMaskRowDeletedLp | EventMaskRowChanged
)

// Matches reports whether the event mask overlaps with the given mask.
func (m EventMask) Matches(mask EventMask) bool { return m&mask != 0 }

// EventhdlrPlugin is a wrapper for the internal SCIP event handler.
type EventhdlrPlugin struct {
	raw  *C.SCIP_EVENTHDLR
	scip *Scip // keeps the owning instance alive and identifies it
}

// live panics with *Error unless the wrapper is usable; see handleErr.
func (h EventhdlrPlugin) live(op string) {
	mustLive(op, "EventhdlrPlugin", h.raw != nil, h.scip, genNone, true)
}

// Inner returns the internal raw pointer of the event handler.
func (h EventhdlrPlugin) Inner() *C.SCIP_EVENTHDLR { return h.raw }

// Name returns the name of the event handler.
func (h EventhdlrPlugin) Name() string {
	defer runtime.KeepAlive(h.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	h.live("EventhdlrPlugin.Name")
	return goString(C.SCIPeventhdlrGetName(h.raw))
}

// Event is a wrapper for the internal SCIP event.
type Event struct {
	raw  *C.SCIP_EVENT
	scip *Scip
}

// Inner returns the internal raw pointer of the event.
func (e Event) Inner() *C.SCIP_EVENT { return e.raw }

// EventType returns the event type of the event.
func (e Event) EventType() EventMask { return EventMask(C.SCIPeventGetType(e.raw)) }

// Var returns the associated variable for a variable event, if any.
func (e Event) Var() (Variable, bool) {
	if e.EventType().Matches(EventMaskVarEvent | EventMaskVarFixed | EventMaskVarDeleted) {
		varPtr := C.SCIPeventGetVar(e.raw)
		if varPtr == nil {
			return Variable{}, false
		}
		return e.scip.newVar(varPtr), true
	}
	return Variable{}, false
}
