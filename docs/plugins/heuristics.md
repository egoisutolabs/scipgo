# Heuristics

A primal heuristic tries to find a feasible solution quickly, from the
current LP solution, the incumbent, or nothing at all. A good one gives
branch-and-bound an early incumbent to prune with.

## Interface

```go
type Heuristic interface {
	Execute(model Model, heur HeuristicPlugin, timing HeurTiming, nodeInfeasible bool) HeurResult
}
```

`timing` says which of the subscribed timing points triggered this call and
`nodeInfeasible` whether the current node was already found infeasible.
The result reports what happened:

| Result | Meaning |
| --- | --- |
| `HeurResultFoundSol` | A solution was added |
| `HeurResultNoSolFound` | Ran, found nothing |
| `HeurResultDidNotRun` | Skipped |
| `HeurResultDelayed` | Skipped, but SCIP should call again at the next opportunity |

## Registration

```go
model.Add(scip.NewHeuristic(&rounding{}).
	Name("myround").
	Desc("rounds the LP solution").
	Priority(100000).                     // default
	Freq(1).                               // default: every depth
	FreqOfs(0).                            // default
	MaxDepth(-1).                          // default: any depth
	Timing(scip.HeurTimingAfterLpNode).    // default: HeurTimingBeforeNode
	DispChar('r').                         // default '?': the column in SCIP's log
	UsesSubscip(false))                    // default
```

The timing mask decides when SCIP calls the heuristic; combine values with
`|`:

| Timing | Called |
| --- | --- |
| `HeurTimingBeforeNode` | Before the node is processed (no LP yet) |
| `HeurTimingDuringLpLoop` | After each LP solve in the cut loop |
| `HeurTimingAfterLpLoop` | After the cut loop finishes |
| `HeurTimingAfterLpNode` | After a node with a solved LP is finished |
| `HeurTimingAfterPseudoNode` | After a node without an LP is finished |
| `HeurTimingAfterLpPlunge`, `HeurTimingAfterPseudoPlunge` | After the last node of a plunge |
| `HeurTimingDuringPricingLoop` | During pricing |
| `HeurTimingBeforePresol`, `HeurTimingDuringPresolLoop` | Around presolving |
| `HeurTimingAfterPropLoop` | After propagation, before the LP |

A rounding heuristic wants an LP solution, so `HeurTimingDuringLpLoop` or
`HeurTimingAfterLpNode`. A construction heuristic that ignores the LP
runs at `HeurTimingBeforeNode`. `UsesSubscip` tells SCIP the heuristic
solves a sub-MIP, so that it can account for nested solves and avoid
running sub-MIP heuristics recursively.

## Creating and submitting solutions

```go
func (h *rounding) Execute(model scip.Model, heur scip.HeuristicPlugin, _ scip.HeurTiming, nodeInf bool) scip.HeurResult {
	if nodeInf {
		return scip.HeurResultDidNotRun
	}
	sol := model.CreateSolFor(heur)
	for _, v := range model.Vars() {
		val := model.CurrentVal(v)
		if v.VarType() != scip.VarTypeContinuous {
			val = math.Round(val) // continuous variables keep their LP value
		}
		sol.SetVal(v, val)
	}
	if err := model.AddSol(&sol); err != nil {
		if errors.Is(err, scip.SolErrorInfeasible) {
			return scip.HeurResultNoSolFound // SCIP checked it and rejected it
		}
		panic(err) // a SCIP failure or a panic in a constraint handler's Check
	}
	return scip.HeurResultFoundSol
}
```

`CreateSolFor(heur)` records the heuristic as the solution's creator, so
`Solution.Heuristic` reports it afterwards. SCIP's own statistics
(`HeuristicPlugin.NSolsFound`, `NBestSolsFound`) credit whichever
heuristic is executing when `AddSol` is called, regardless of which
constructor made the solution; both forms of attribution show up in the
solve log and `StatsJSON`. `AddSol` consumes the solution and returns
`scip.SolErrorInfeasible` if SCIP's check rejected it, which is the normal
outcome for a guess that did not work. Any other error is a SCIP failure
or a panic recovered from a constraint handler's `Check`; do not swallow
it, since `AddSol` has already collected that panic and returning
`NoSolFound` would hide it from the enclosing `Solve`. Panicking with the
error inside the callback re-raises it properly.

`CreateOrigSolFor` and `CreatePartialSolFor` are the original-space and
partial variants; see [Solutions](../solutions.md).

## Example

`examples/random_rounding` rounds each fractional integer variable up
with probability equal to its fractional part, runs during the LP loop,
and afterwards prints how many solutions the heuristic is credited with.

## Notes

- `model.Vars()` during a solve returns the transformed variables; that is
  what `CurrentVal` and `SetVal` expect.
- A heuristic can start a [dive or probing session](../probing-and-diving.md)
  to explore roundings with LP support before committing to one.
- Between calls, keep state in the plugin's fields. A copy running in a
  concurrent worker (see `Copyable`) must not share mutable state with the
  original without synchronisation; the simplest rule is to return a
  fresh value from `Copy`.
