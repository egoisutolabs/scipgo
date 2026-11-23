# Propagators

A propagator tightens variable domains at a node, on its own rather than as
part of a constraint handler. Use one when the reduction does not belong to
a constraint type: the handler interface would still demand `Check` and
`Enforce`.

## Interface

```go
type Propagator interface {
	Propagate(model Model, prop PropagatorPlugin) PropResult
}
```

`PropResult` is the same code a constraint handler's propagation returns:

| Result | Meaning |
| --- | --- |
| `PropResultReducedDom` | A domain was reduced |
| `PropResultCutOff` | The node is infeasible |
| `PropResultDidNotFind` | Searched, found nothing |
| `PropResultDidNotRun` | Skipped |
| `PropResultDelayed` | Skipped, call again |
| `PropResultDelayNode` | Postpone the node. Only meaningful before the LP |

The callback is not told which bit of the timing mask fired. The mask it
was registered with is `prop.Timing()`.

## Registration

```go
model.Add(scip.NewPropagator(&cap{}).
	Name("capx").
	Desc("x <= 2").
	Priority(100000).             // default; >= 0 runs before constraint handlers
	Freq(1).                      // default: every depth; 0 root only; -1 off
	Delay(false).                 // default
	Timing(scip.PropTimingBeforeLp)) // default
```

`Timing` is a mask; combine values with `|`:

| Timing | Called |
| --- | --- |
| `PropTimingBeforeLp` | Before the node's LP is solved |
| `PropTimingDuringLpLoop` | After each LP solve in the cut-and-price loop |
| `PropTimingAfterLpLoop` | After the cut-and-price loop |
| `PropTimingAfterLpNode` | After a node with a solved LP finishes |

A priority of 100000, the default, runs after SCIP 10's fast bound
propagators (`rootredcost` 10000000, `dualfix` 8000000, `vbounds` /
`genvbounds` / `pseudoobj` 3000000, `redcost` 1000000) and before
constraint handlers. Negative priorities run after constraint handlers;
`probing` is -100000 and `obbt` is -1000000.

Names must be unique among propagators, including SCIP's own.
`Propagators` and `FindPropagator` return this same `PropagatorPlugin`,
including for SCIP's built-in propagators.

## Tightening a bound

```go
func (c *cap) Propagate(model scip.Model, _ scip.PropagatorPlugin) scip.PropResult {
	infeasible, tightened, err := model.TryTightenVarUb(c.x, 2)
	if err != nil {
		panic(err)
	}
	if infeasible {
		return scip.PropResultCutOff
	}
	if tightened {
		return scip.PropResultReducedDom
	}
	return scip.PropResultDidNotFind
}
```

`TryTightenVarLb` and `TryTightenVarUb` return whether the new domain is
empty and whether the bound moved. A change smaller than SCIP's
bound-strengthening tolerance is ignored. The change applies to the
current node during presolving and solving (global at the root) and to the
original variable in the problem stage. An original variable from `AddVar`
may be passed during solving; SCIP resolves it to the active variable. It
is not recorded as an inference, so conflict analysis treats it like a
branching decision; resolving a propagation (`SCIPinferVarLbProp`) is not
exposed.

Give the variable a constraint that locks the bound you tighten. On an
unconstrained maximization, SCIP's `dualfix` propagator (priority 8000000)
fixes the variable at its upper bound before a propagator at the default
priority runs, and the later tightening is infeasible. Two variables with
the same objective and bounds are symmetric: a bound change on one is
copied onto the other.

The calls are legal in the Problem, Transformed, Presolving and Solving
stages, and rejected while SCIP is diving. Bound changes inside a dive
belong to `Diver.ChgVarLb` and `Diver.ChgVarUb`.

## Presolving

A propagator that also implements `PropagatorPresol` runs during
presolving. The hook is registered only when the method is present.

```go
func (c *cap) PresolveProp(model scip.Model, _ scip.PropagatorPlugin) scip.PresolResult {
	x := c.x
	if tx, ok := c.x.Transformed(); ok {
		x = tx
	}
	_, tightened, err := model.TryTightenVarUb(x, 2)
	if err != nil {
		panic(err)
	}
	if !tightened {
		return scip.PresolResult{Status: scip.PresolStatusDidNotFind}
	}
	// The tightening calls do not update SCIP's presolve counters.
	return scip.PresolResult{Status: scip.PresolStatusSuccess, NChgBds: 1}
}
```

`PresolResult` carries the result code and the reductions this call
performed. SCIP adds the counts to its running totals.

| Status | Meaning |
| --- | --- |
| `PresolStatusSuccess` | Found a reduction |
| `PresolStatusDidNotFind` | Searched, found nothing |
| `PresolStatusDidNotRun` | Skipped |
| `PresolStatusCutoff` | The problem is infeasible |
| `PresolStatusUnbounded` | The problem is unbounded |
| `PresolStatusDelayed` | Documented by SCIP, rejected by SCIP 10 for this hook |

Returning `PresolStatusDelayed` fails the solve with a `*CallbackPanic`.
SCIP 10's propagator presolve does not accept that code, despite the
header listing it.

The builder's `PresolPriority`, `PresolMaxRounds` (-1, the default, means
no limit) and `PresolTiming` configure the hook. The default timing is
`PresolTimingAlways` (fast, medium and exhaustive, not the final round)
and the default presolve priority is 100000. `PresolMaxRounds` cannot be
read back: SCIP stores it and exposes no getter.

Built-in presolve methods at a higher priority can finish the problem
before a hook at the default priority runs. `pseudoobj` (presolve priority
6000000) does this once a heuristic has a solution it can prove optimal,
and SCIP then skips every later presolver. Raise `PresolPriority` above
that when the reduction has to happen first. Tighten the variable from
`Variable.Transformed` when it exists: during presolving that is the
variable the transformed problem solves with. A heuristic that runs before
presolving can also store a solution that stays incumbent after the bound
moves, so turn heuristics off when the hook's bound is supposed to be the
one the solver finds.

`PresolveProp` is not told the round number, the presolve timing that
fired, or how much other presolvers have changed since its last call.

Go presolver plugins are a separate change. They will return this same
`PresolResult`. Fixing, aggregating and deleting constraints
(`SCIPfixVar`, `SCIPaggregateVars`, `SCIPdelCons`) are not exposed yet, so
a presolve hook today tightens bounds and reports the counts.

## Copies

A propagator that implements `Copyable` is copied into sub-SCIPs. `Copy`
returns the value to include, and optional interfaces are detected on that
value: a copy that drops `PropagatorPresol` does not presolve. See the
[plugin overview](README.md#copies-and-sub-scips).

Go propagators are not marked safe for exact solving, so SCIP skips them
while exact mode is on.

## Example

`examples/bound_propagator` caps one variable's upper bound and checks
that the objective follows.
