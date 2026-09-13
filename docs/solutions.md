# Solutions

Reading the solutions a solve produced, and handing SCIP solutions of your
own as starting points.

## Reading solutions

```go
solved := model.Solve()

sol, ok := solved.BestSol()
if !ok {
	return errors.New("no feasible solution found")
}
fmt.Println(sol.ObjVal())
fmt.Println(sol.Val(x))
```

`BestSol` returns the incumbent and whether one exists. Check the boolean:
a solve that hit a limit or proved infeasibility may have none. `GetSols`
returns every stored solution, best first; `NSols` counts them.

A `Solution` answers:

| Method | Returns |
| --- | --- |
| `ObjVal` | Objective value |
| `Val(v)` | Value of one variable |
| `AsNameMap` | `map[string]float64` of nonzero values keyed by variable name |
| `AsIDMap` | `map[int]float64` of nonzero values keyed by problem index |
| `Heuristic` | The heuristic that created it, if any |
| `IsPartial` | Whether unset variables are unknown rather than zero |
| `String` | A readable dump, one line per original variable |

`Val` accepts an original variable or its transformed counterpart and
returns the same number. `AsNameMap` walks the transformed problem, so after
a solve its keys carry SCIP's `t_` prefix and presolving may have removed
fixed variables. When you need values by the names you gave, iterate
`OrigVars` and call `Val`.

Values are floating point and honour SCIP's feasibility tolerance. A binary
variable set to 1 may come back as `0.9999999`; compare against `0.5`, and
round integers with `math.Round`, as the examples do.

The solutions of a solve stay readable after the solve, and survive
`FreeTransform`. They do not survive `Free` or a `CreateProb` or `ReadProb`
that replaces the problem.

## Solutions during a solve

Inside a plugin callback the incumbent is available the same way, and
`Model.CurrentVal(v)` gives the variable's value in the current LP or
pseudo solution, which is what a heuristic rounds or a separator checks. A
`Variable.SolVal` reports the same value but only while the model is
presolved or solving; outside those stages it is rejected rather than
producing garbage.

## Providing a starting solution

A known feasible point, from a previous run or a heuristic of your own,
speeds up the solve by giving SCIP an incumbent to prune with. Create a
solution, set values, add it:

```go
start := model.CreateSol()
start.SetVal(x, 3)
start.SetVal(y, 4)
if err := model.AddSol(&start); err != nil {
	// scip.SolErrorInfeasible: SCIP checked it and rejected it
}
solved := model.Solve()
```

`AddSol` takes a pointer because it consumes the solution: SCIP checks
feasibility, stores it if it passes, and the `Solution` value is invalid
afterwards either way. The three outcomes are nil (stored),
`scip.SolErrorInfeasible` (rejected) and a `*scip.Error` or
`*scip.CallbackPanic` when the check itself failed.

Three constructors exist:

| Constructor | Use |
| --- | --- |
| `CreateSol` | A solution over the current problem, all values zero. Before a solve this is the original problem |
| `CreateOrigSol` | Always in the original space, even during a solve |
| `CreatePartialSol` | Values you do not set are unknown, not zero. SCIP's `completesol` heuristic fills in the rest at the start of the solve |

Partial solutions are the practical choice for MIP starts: fix the
variables you are sure about and let the solver complete the rest.

Each constructor has a `For` variant that takes a `HeuristicPlugin` and
records it as the solution's creator; see
[Heuristics](plugins/heuristics.md) for when that matters.

## Exact values

On a model solved in [exact mode](solving.md#exact-solving), `ValExact` and
`ObjValExact` return the rational values as `*big.Rat`. On any other model
their `Try` forms return an error, since SCIP has no rational to report.
