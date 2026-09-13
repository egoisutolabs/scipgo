# Coming from russcip

scipgo is a port of [russcip](https://github.com/scipopt/russcip), the
Rust interface to SCIP, and keeps its API shape wherever Go allows. This
page maps one to the other and lists what differs.

## Concepts

| russcip | scipgo |
| --- | --- |
| `Model<State>` typestate (`Model<ProblemCreated>`, `Model<Solved>`, ...) | One `Model` type; `Model.Stage()` reports the state and every method checks it |
| `Result<T, Retcode>` | Two forms of each method: `TryX` returns `error`, `X` panics with the same `*scip.Error` |
| `Retcode` | `scip.Retcode`, also an `error`; wrapped in `*scip.Error` with the operation and stage |
| `Model::default()` | `scip.DefaultModel()` |
| `Model::new().include_default_plugins().create_prob("p")` | `scip.NewModel().IncludeDefaultPlugins().CreateProb("p")` |
| `model.add(var().name("x").bin())` | `model.Add(scip.NewVar().Name("x").Bin())` or `scip.NewVar()...AddTo(model)` |
| `cons().eq(1).coef(&x, 1)` | `scip.NewCons().Eq(1).Coef(x, 1)` |
| `row().le(1)` | `scip.NewRow().Le(1)` |
| `model.add_var(lb, ub, obj, "x", VarType::Binary)` | `model.AddVar(lb, ub, obj, "x", scip.VarTypeBinary)` |
| `model.solve()` | `model.Solve()` or `model.TrySolve()` |
| `model.best_sol()` returning `Option<Solution>` | `model.BestSol()` returning `(Solution, bool)` |
| `Rc<Variable>`, `Rc<Constraint>` | `scip.Variable`, `scip.Constraint` value types |
| `BranchRule`, `Pricer`, `Heuristic`, ... traits | Interfaces of the same names |
| `SCIPBranchRule`, `SCIPHeur`, ... wrappers | `BranchRulePlugin`, `HeuristicPlugin`, ... |
| `model.set_data(v)`, `model.get_data::<T>()` | `scip.SetData(model, v)`, `scip.GetData[T](model)` |
| `model.set_param::<T>(name, v)`, `model.param::<T>(name)` | `scip.SetParam(model, name, v)`, `scip.GetParam(model, name, &out)`, and typed forms |
| `Drop` on `Model`, `Prober`, `Diver` | `Model.Free`, `Prober.End`, `Diver.End`, plus a finalizer for models |

Method names follow Go conventions: `add_cons_set_part` is
`AddConsSetPart`, `n_vars` is `NVars`, `obj_val` is `ObjVal`. Plugin
families are named after SCIP's own stems, `Heuristic`, `Separator`,
`Nodesel`, `Eventhdlr`, `Conshdlr`, consistently across interface,
builder, wrapper and `Include*` method.

## What is different

**No typestate.** Go has no affine types, so the compile-time guarantee
that you cannot add a variable to a solved model becomes a run-time check
that reports `RetcodeInvalidCall`. Every query and handle method checks
the stage and the liveness of the model and handle before touching SCIP.

**Errors are values, or panics, per call.** russcip returns `Result` from
fallible methods. scipgo gives you both: use `Try*` where you would match
on the `Result`, the plain form where you would `unwrap`.

**No destructors.** `Prober` and `Diver` sessions are ended with an
explicit `End`. SCIP instances are freed by `Model.Free`, or by a
finalizer when everything referring to the instance is unreachable;
prefer `Free` in long-running programs.

**Solutions are consumed explicitly.** `AddSol` takes a `*Solution` so it
can invalidate the handle it consumes, where russcip takes the value by
move.

**Panics cannot cross C.** A panic in a plugin callback is captured, the
callback reports an error to SCIP, and the panic is re-raised from `Solve`
as a `*scip.CallbackPanic` once SCIP has unwound. russcip's callbacks abort
on panic.

**The datastore is a Go-side registry** keyed by the SCIP instance instead
of a hidden plugin, and is visible from sub-SCIP copies.

**Copyable is opt-in.** A plugin participates in SCIP's sub-SCIP copies
only if it implements `Copyable`; see the [plugin overview](plugins/README.md#copies-and-sub-scips).

**Concurrent solves are serialised** across goroutines, because SCIP's
thread pool is a process-wide global that each instance creates and
destroys. Several models may still use `SolveConcurrent` over the process
lifetime.

**Things scipgo adds** that have no russcip counterpart: `Interrupt`,
`SolveContext` and `SolveConcurrentContext`; per-model log routing with
`SetLogFunc`, `SetLogWriter` and `SetLogger`; exact solving with
`EnableExactSolving`, `ValExact` and `ObjValExact`; the query guards
described in [Errors](errors.md); and heuristic attribution through
`CreateSolFor` and `Solution.Heuristic`.

## Deprecated names

The names from v0.2.0 remain as deprecated aliases and will be removed in
v1.0.0:

| Deprecated | Use |
| --- | --- |
| `SCIPBranchRule`, `SCIPConshdlr`, `SCIPEventhdlr`, `SCIPNodesel`, `SCIPPricer`, `SCIPSeparator` | `BranchRulePlugin`, `ConshdlrPlugin`, `EventhdlrPlugin`, `NodeselPlugin`, `PricerPlugin`, `SeparatorPlugin` |
| `Heur`, `HeurPlugin`, `HeurBuilder`, `NewHeur` | `HeuristicPlugin`, `HeuristicPlugin`, `HeuristicBuilder`, `NewHeuristic` |
| `Presolver` | `PresolverPlugin` |
| `SepaBuilder`, `NewSepa`, `SourceSepa` | `SeparatorBuilder`, `NewSeparator`, `SourceSeparator` |
| `NodeSel`, `NodeSelBuilder` | `Nodesel`, `NodeselBuilder` |
| `EventHdlrBuilder` | `EventhdlrBuilder` |
| `Heurs`, `FindHeur` | `Heuristics`, `FindHeuristic` |
| `SetHeurPriority`, `SetSepaPriority`, `SetPresolPriority` and their `Try` forms | `SetHeuristicPriority`, `SetSeparatorPriority`, `SetPresolverPriority` |

gopls and staticcheck flag uses of the deprecated names.
