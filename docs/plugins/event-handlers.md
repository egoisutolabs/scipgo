# Event handlers

An event handler is called when something happens during the solve: a node
is focused, an LP is solved, a new incumbent is found. Use one to collect
statistics, drive a progress display, or react to a new solution.

## Interface

```go
type Eventhdlr interface {
	// GetEventMask returns the events to subscribe to.
	GetEventMask() EventMask
	// Execute is called once per event.
	Execute(model Model, eventhdlr EventhdlrPlugin, event Event)
}
```

The mask is a bit set; combine values with `|`. The binding subscribes
with SCIP's global catch when the solve starts, so the events that arrive
are the global ones:

| Mask | Fires when |
| --- | --- |
| `EventMaskNodeFocused` | A node becomes the focus node |
| `EventMaskNodeFeasible`, `EventMaskNodeInfeasible`, `EventMaskNodeBranched` | A node is finished: its LP was feasible, it was cut off, or it was branched on. `EventMaskNodeSolved` is their union |
| `EventMaskNodeDelete` | A node is deleted from the tree |
| `EventMaskFirstLpSolved`, `EventMaskLpSolved` | The node's first LP, or any LP, was solved. `EventMaskLpEvent` is the union |
| `EventMaskBestSolFound`, `EventMaskPoorSolFound` | A new incumbent, or a feasible but worse solution, was found. `EventMaskSolFound` is the union |
| `EventMaskPresolveRound` | A presolving round finished |
| `EventMaskVarAdded`, `EventMaskVarDeleted` | A variable was added to or removed from the transformed problem |
| `EventMaskDualBoundImpr` | The global dual bound improved |
| `EventMaskSync` | A concurrent-solve synchronisation happened |

Per-variable events, such as bound changes and objective changes, and
per-row events need SCIP's per-object catch and are not delivered through
this interface, even if their bits are in the mask.

`Event.EventType` says which event fired, `EventMask.Matches` tests it,
and `Event.Var` returns the variable for a variable event.

## Registration

```go
model.Add(scip.NewEventhdlr(handler).Name("progress").Desc("logs incumbents"))
```

There are no priorities or frequencies; every registered handler receives
every event in its mask.

## Example: watching incumbents

```go
type incumbents struct {
	seen []float64
}

func (h *incumbents) GetEventMask() scip.EventMask { return scip.EventMaskBestSolFound }

func (h *incumbents) Execute(model scip.Model, _ scip.EventhdlrPlugin, _ scip.Event) {
	if sol, ok := model.BestSol(); ok {
		h.seen = append(h.seen, sol.ObjVal())
	}
}
```

After the solve, `h.seen` holds the sequence of incumbent objective values,
in the order they were found. `examples/node_event_handler` does the same
for `EventMaskNodeFocused` and checks the count against `NNodes`.

## Notes

- The handler runs synchronously in the solver; keep it fast.
- Reading the tree in a node event is fine: `model.FocusNode()` is the
  node the event is about for `EventMaskNodeFocused`.
- A handler that implements `Copyable` is copied into sub-SCIPs and
  concurrent workers and receives their events too, on their threads.
  Without `Copy` it sees only the main solve, which is usually what a
  progress display wants.
- The binding registers an event handler of its own to support
  `SolveConcurrentContext`; its name is reserved and it does not appear in
  your handler's events.
