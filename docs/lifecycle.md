# Model lifecycle

What a `Model` is, which stage it is in, when handles stop being valid,
how memory is released, and the rules for goroutines.

## One type, many stages

russcip encodes SCIP's lifecycle in the type system as `Model<State>`.
scipgo uses one `Model` type and exposes the stage through `Model.Stage`;
every method checks that the stage allows it and reports a `*scip.Error`
with `RetcodeInvalidCall` otherwise. `Model` is a small value type holding
a pointer to the SCIP instance. Copying it does not copy the instance, and
the `Model` returned by chainable methods is the same instance as the
receiver.

## Stages

SCIP's stages, in the order a solve passes through them:

| Stage | Reached by | What is allowed |
| --- | --- | --- |
| `StageInit` | `NewModel` | Include plugins, set parameters, `EnableExactSolving`, install a log sink |
| `StageProblem` | `CreateProb`, `ReadProb`, `FreeTransform` | Add variables and constraints, register plugins, set parameters, add starting solutions, install a log sink |
| `StageTransforming`, `StageTransformed` | Start of `Solve` | Internal: the problem is copied into the working space |
| `StageInitPresolve`, `StagePresolving`, `StageExitPresolve`, `StagePresolved` | `Solve` | Presolving runs. Callbacks may see these stages |
| `StageInitSolve`, `StageSolving` | `Solve` | Branch-and-bound. Plugin callbacks run here; the tree, LP, rows and columns exist |
| `StageSolved` | End of `Solve` | Read status, solutions and statistics |
| `StageExitSolve`, `StageFreeTrans` | `FreeTransform` | Internal: solving data is released |
| `StageFree` | `Free` | Nothing; every call reports an error |

The two stages you interact with directly are Problem, where the model is
built, and Solved, where results are read. Plugins run in the middle ones
and the [tree and LP page](tree-and-lp.md) covers what they can see there.
A few operations are stage-specific in ways worth remembering:

- `EnableExactSolving` works only in Init, before plugins are included.
- A log sink can be installed or swapped only while the problem is not
  transformed, that is in Init or Problem.
- Plugins are registered in Problem (or Init, before the problem exists).
- Queries that need the transformed problem, such as `ObjVal` and `Vars`
  after a solve, work from Transformed onwards; `OrigVars` works from
  Problem onwards.
- `AddVar` during Solving, from a pricer, returns the transformed variable.

## Handles

`Variable`, `Constraint`, `Solution`, `Node`, `Row`, `Col` and the plugin
wrappers are handles: small values that wrap a SCIP pointer plus enough
identity to tell whether it is still valid. Each carries the model it came
from, so no method needs a model argument to check it.

A handle is valid until the thing it points to is released:

| Handle | Lives until |
| --- | --- |
| Original `Variable`, `Constraint`, `Solution` | The problem is replaced (`CreateProb`, `ReadProb`) or the model is freed |
| Transformed `Variable`, global transformed `Constraint` | `FreeTransform` |
| `Constraint` from `AddConsLocal` or `AddConsNode` | Its node's subtree is deleted, which the binding does not detect; use it within the callback |
| `Row`, `Col`, `Node` | `FreeTransform`; SCIP may release a row or free a node earlier, which the binding does not detect, so use them within the callback. The binding releases its own capture on a row when it is added, at `FreeTransform`, at model free, or on `Row.Release` — after that the handle is dead and further use fails
with `RetcodeInvalidCall` |
| Plugin wrappers (`HeuristicPlugin` and friends) | The model is freed |

Using a handle after `Free`, `FreeTransform`, `CreateProb` or `ReadProb`
produces a `*scip.Error`; see [Errors](errors.md#liveness). The binding
does not detect the earlier releases in the table, when SCIP frees a
processed node, a row that left the LP, or a node-local constraint; a
handle used after one of those passes a dangling pointer into C. Keep
those three kinds within the callback that obtained them (see
[Plugins](plugins/README.md#inside-a-callback)). Issue #20 tracks closing
that gap.

`Inner` on any handle returns the raw C pointer for use with SCIP calls
the binding does not wrap. It is subject to the same liveness rules and
the pointer must not outlive the handle.

## Releasing memory

A SCIP instance holds memory on the C heap that the Go garbage collector
cannot see. Two things release it:

- `Model.Free` releases it now. Every handle sharing the instance is
  invalid afterwards. Freeing twice is a no-op.
- A finalizer releases it when the `Model` and every handle derived from it
  become unreachable.

The finalizer is a safety net, not a strategy. The garbage collector does
not know how large a SCIP instance is and will not hurry, and a plugin,
log callback or closure that holds a `Variable`, `Constraint` or `Model`
of the instance keeps it reachable forever. In a long-running service,
`defer model.Free()` after creating a model.

`FreeTransform` releases the transformed problem and the solving data
while keeping the original problem, plugins and solutions; it is the way
to modify and re-solve without rebuilding.

`Prober` and `Diver` sessions started with `StartProbing` and `StartDiving`
must be ended with `End` before the callback that started them returns.

## Goroutines

A `Model` and every handle derived from it belong to one goroutine at a
time. SCIP is not thread-safe, and the binding does not add locking around
it. Two rules make this workable:

- **`Interrupt` is the exception.** It may be called from any goroutine
  while a solve runs on another; that is its purpose. `SolveContext` and
  `SolveConcurrentContext` are built on it.
- **Independent models are independent.** Several goroutines may each own
  a model and solve concurrently, with one caveat: `SolveConcurrent` uses
  SCIP's process-wide thread pool, so concurrent solves on different models
  take turns. Plain `Solve` calls run in parallel without restriction.

Plugin callbacks run on the goroutine that called `Solve`, except for
copies inside `SolveConcurrent` workers, which run on the worker threads
and may run at the same time as each other. Log sinks share that property.
The [Plugins](plugins/README.md) and [Logging](logging.md) pages cover the
consequences.

Sub-SCIPs that SCIP creates internally, for large neighbourhood search
heuristics and for concurrent workers, share their origin's datastore (see
[Plugins](plugins/README.md#sharing-data-with-plugins), including the
locking that sharing requires under `SolveConcurrent`) but are otherwise
distinct instances. A handle from the parent must not be passed to a
sub-SCIP's model and vice versa; both directions are rejected with
`RetcodeInvalidData`.
