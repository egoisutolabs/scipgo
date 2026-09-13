# Solving

Running the solver, reading how it ended, bounding its effort, stopping it
from Go, collecting statistics, solving again after a change, and the
concurrent and exact modes.

## Solve

```go
solved := model.Solve()          // panics on failure
solved, err := model.TrySolve()  // returns the failure
```

Both run presolving, then branch-and-bound, and return the same model in
the Solved stage. The returned value exists for chaining; `model` and
`solved` refer to one SCIP instance. A failure is a `*scip.Error` for a
SCIP-level problem or a `*scip.CallbackPanic` when one of your plugins
panicked; see [Errors](errors.md).

## Status

`Status` reports why the solve ended:

| Status | Meaning | `BestSol` |
| --- | --- | --- |
| `StatusOptimal` | Proven optimal | Available |
| `StatusInfeasible` | Proven infeasible | None |
| `StatusUnbounded` | Proven unbounded | Usually none |
| `StatusInforunbd` | Infeasible or unbounded; presolving could not tell which | None |
| `StatusTimeLimit`, `StatusMemoryLimit`, `StatusNodeLimit`, `StatusTotalNodeLimit`, `StatusStallNodeLimit` | A limit was hit | Available if any solution was found |
| `StatusGapLimit` | The relative gap limit was reached | Available |
| `StatusSolutionLimit`, `StatusBestSolutionLimit` | The `limits/solutions` or `limits/bestsol` count was reached | Available if any solution was found; a limit of 0 stops before the first |
| `StatusPrimalLimit`, `StatusDualLimit` | A bound target was reached | Depends |
| `StatusRestartLimit` | The restart limit was reached | Depends |
| `StatusUserInterrupt` | `Interrupt` or a done context stopped it | Available if any solution was found |
| `StatusTerminate` | The process received SIGTERM | Depends |
| `StatusUnknown` | No solve has run | None |

Always check `BestSol`'s boolean rather than assuming a solution exists
when the status is not optimal.

## Limits and effort

```go
model.SetTimeLimit(60)    // seconds of wall-clock time
model.SetMemoryLimit(2048) // MB
model, _ = scip.SetParam(model, "limits/nodes", int64(10000))
model, _ = scip.SetParam(model, "limits/gap", 0.005) // stop at 0.5% relative gap
model, _ = scip.SetParam(model, "limits/solutions", int32(1)) // stop at the first feasible solution
```

Three emphasis switches trade solve quality for speed in one call each,
mirroring SCIP's `set presolving emphasis` and friends:

```go
model.SetPresolving(scip.ParamSettingAggressive)
model.SetHeuristics(scip.ParamSettingFast)
model.SetSeparating(scip.ParamSettingOff)
```

`scip.MinimalModel()` turns all three off, which is the right starting point
for tests of custom plugins and for models where you want to watch the raw
branch-and-bound. [Parameters](parameters.md) lists more.

## Stopping a solve

A running solve can be stopped from another goroutine, or through a
`context.Context`:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

solved, err := model.SolveContext(ctx)
if errors.Is(err, context.DeadlineExceeded) {
	// status is StatusUserInterrupt; the incumbent, if any, is still there
	if sol, ok := solved.BestSol(); ok {
		report(sol)
	}
}
```

`SolveContext` behaves like `TrySolve` and additionally stops the solve when
the context is done. The error is a `*scip.Error` with `Op` set to
`SolveContext` wrapping the context error, so `errors.Is` against
`context.DeadlineExceeded` and `context.Canceled` works. A context that is
already done prevents the solve from starting at all. If SCIP finishes
before the request lands, the completed result is returned with a nil
error.

`Interrupt` is the underlying primitive: it asks SCIP to stop at the next
opportunity and is the one `Model` method that is safe to call from
another goroutine while a solve is running. It is a no-op when no solve is
in progress, and a stale request from before a solve is discarded when the
next one starts. Calling it from inside a plugin callback stops the solve
the callback belongs to once the callback returns.

A stop is noticed between nodes, presolving rounds, LP iterations and
pricing rounds. One long operation, such as a big root LP or a slow plugin
callback, delays it until that operation returns. A plugin that needs finer
granularity can check its own context between callbacks.

A stopped solve leaves the model usable. Read the incumbent, call
`FreeTransform` and solve again with different settings if you want.

## Statistics

| Method | Returns |
| --- | --- |
| `ObjVal` | Objective of the best solution (the primal bound) |
| `BestBound` | The dual bound proven so far |
| `NNodes` | Branch-and-bound nodes processed |
| `NLpIterations` | Simplex iterations |
| `NSols` | Solutions found |
| `SolvingTime` | Seconds spent solving |
| `StatsJSON`, `WriteStatsJSON(path)` | SCIP's full statistics table as JSON |

Individual plugins report their own statistics: `Heuristics()`,
`Separators()` and `Presolvers()` list SCIP's built-in plugins, and
`FindHeuristic("rens")` and friends fetch one by name. A `HeuristicPlugin`
answers `NCalls`, `NSolsFound` and `NBestSolsFound`; a `PresolverPlugin`
answers `NCalls` and `Time`.

## Solving again

After a solve the model is in the Solved stage and the problem is frozen.
`FreeTransform` discards the transformed problem and returns the model to
the Problem stage, where it can be modified and solved again:

```go
solved := model.Solve()
first := solved.ObjVal()

model.FreeTransform()
model.AddCons([]scip.Variable{x}, []float64{1}, scip.NegInfinity, 3, "cap")
solved = model.Solve()
second := solved.ObjVal()
```

Variables and constraints you obtained in the Problem stage survive
`FreeTransform`. Transformed variables, rows, columns, nodes and the
`Solution` handles a solve produced do not; using one afterwards produces
a `*scip.Error` rather than a crash. Read the values you need from a
solution before `FreeTransform`, or call `BestSol` or `GetSols` again after
it, since SCIP keeps the solutions themselves and hands out fresh handles.
Plugins stay registered, and the previous run's solutions are candidates
for the next one.

`FreeTransform` runs plugin callbacks (each plugin's free hook), so like
`Solve` it has a `TryFreeTransform` form that reports a `*CallbackPanic`.

## Concurrent solving

SCIP can run several differently configured solvers on the same problem in
parallel and stop when the first one finishes:

```go
model, _ = model.SetIntParam("parallel/maxnthreads", int32(runtime.NumCPU()))
model, _ = model.SetIntParam("parallel/mode", 1) // 1 deterministic, 0 opportunistic
solved, err := model.TrySolveConcurrent()
```

Each worker is a separate SCIP instance created by copying the model. Two
things follow from that:

- **Custom plugins reach the workers only if they implement `Copyable`.**
  Their callbacks then run on the worker threads, concurrently, so they
  must be safe for that. Plugins without `Copy` do not run in the workers
  at all. See [Plugins](plugins/README.md#copies-and-sub-scips).
- **A log sink installed with `SetLogFunc` is shared** by the workers and
  may be called from several threads at once. See [Logging](logging.md).

SCIP's thread pool is one per process. The binding keeps its creation and
destruction balanced across models, but concurrent solves are serialised:
two goroutines calling `SolveConcurrent` on different models take turns.
`SolveConcurrentContext` is the cancellable form. Its stop relies on an
event handler the binding registers automatically. A model built without
`IncludeDefaultPlugins` whose very first solve is a concurrent one started
from a stage past the Problem stage cannot be interrupted before it
finishes; every other case works as for `SolveContext`.

The SCIP installation must have been built with thread support (its TPI).
The Homebrew and release packages have it. Without it,
`TrySolveConcurrent` returns an error before anything runs.

## Exact solving

SCIP 10 can solve a MIP in rational arithmetic end to end, so that bounds,
LP solutions and the final answer carry no floating-point round-off. Enable
it on a fresh model, before plugins are included or a problem is created:

```go
model := scip.NewModel().
	EnableExactSolving(). // Init stage only
	IncludeDefaultPlugins()
model.ReadProb("instance.mps")
solved := model.Solve()

if sol, ok := solved.BestSol(); ok {
	fmt.Println(sol.ObjValExact()) // *big.Rat, e.g. 7615/1
	for _, v := range solved.OrigVars() {
		fmt.Println(v.Name(), sol.ValExact(v)) // *big.Rat per variable
	}
}
```

`ValExact` and `ObjValExact` return `*big.Rat`. Convert to whatever decimal
representation you need; the float accessors keep working and return the
rational values' nearest `float64`. On a model that did not solve exactly
there is no rational value to report, and the `Try` forms return an error
instead of asking SCIP for one.

SCIP announces an exact solve with a `solving problem in exact solving
mode` line that `HideOutput` does not suppress; a log sink receives it
like any other line. Exact mode is considerably slower than floating-point
solving and supports linear constraints over integer and continuous
variables. It is the right
tool when a proof of optimality has to hold up to scrutiny, or when the
solution feeds exact downstream arithmetic.
