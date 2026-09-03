package scip

import (
	"sync"
	"testing"
)

type countingEventHdlr struct {
	mu      sync.Mutex
	counter int
}

func (h *countingEventHdlr) GetEventMask() EventMask {
	return EventMaskLpEvent | EventMaskNodeEvent
}

func (h *countingEventHdlr) Execute(_ Model, _ SCIPEventhdlr, _ Event) {
	h.mu.Lock()
	h.counter++
	h.mu.Unlock()
}

func TestEventhdlr(t *testing.T) {
	h := &countingEventHdlr{}
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewEventhdlr(h).Name("CountingEventHdlr"))
	model.Solve()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.counter <= 1 {
		t.Fatalf("event handler ran %d times", h.counter)
	}
}

type internalSCIPEventHdlrTester struct{ t *testing.T }

func (internalSCIPEventHdlrTester) GetEventMask() EventMask {
	return EventMaskLpEvent | EventMaskNodeEvent
}

func (h internalSCIPEventHdlrTester) Execute(_ Model, eventhdlr SCIPEventhdlr, event Event) {
	if !(EventMaskLpEvent | EventMaskNodeEvent).Matches(event.EventType()) {
		h.t.Error("unexpected event type")
	}
	if eventhdlr.Name() != "InternalSCIPEventHdlrTester" {
		h.t.Errorf("handler name %q", eventhdlr.Name())
	}
	if _, ok := event.Var(); ok {
		h.t.Error("expected no variable for LP/node event")
	}
}

func TestInternalEventhdlr(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewEventhdlr(internalSCIPEventHdlrTester{t: t}).Name("InternalSCIPEventHdlrTester"))
	model.Solve()
}
