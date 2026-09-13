# Changelog

All notable changes to scipgo are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
uses [Semantic Versioning](https://semver.org/).

## [0.3.0] - 2026-09-13

### Added

- Exact solving: `EnableExactSolving` on a fresh model runs the whole
  solve in rational arithmetic; `Solution.ValExact` and `ObjValExact`
  return `*big.Rat`.
- Log routing per model: `SetLogFunc`, `SetLogWriter` and `SetLogger`
  deliver SCIP's output as whole lines to a callback, an `io.Writer` or a
  `*slog.Logger`. `SetErrorLogFunc` redirects SCIP's process-global error
  messages.
- Stopping a solve: `Interrupt` is safe to call from another goroutine;
  `SolveContext` and `SolveConcurrentContext` stop when a
  `context.Context` is done and return the interrupted model with its
  incumbent.
- Heuristic attribution: `CreateSolFor`, `CreateOrigSolFor` and
  `CreatePartialSolFor` record the creating heuristic on a solution;
  `Solution.Heuristic` reads it back. `HeuristicPlugin` reports
  `NBestSolsFound`.
- An error-returning `Try` form of every fallible method, alongside the
  panicking form. `*Error` carries the operation, stage, return code and
  detail; `*CallbackPanic` carries a panic recovered from a plugin
  callback.
- Query guards: every getter and handle method checks the model's
  liveness and the current stage before calling SCIP, and reports an
  `*Error` instead of letting SCIP continue into undefined behaviour.
  Liveness is judged by instance and problem generation, so handles from
  freed models, replaced problems, released transformed problems and
  other models are all detected.
- Guides under `docs/`, a contributing guide, this changelog, and
  runnable examples on pkg.go.dev.

### Fixed

- `BranchingCandidate.Frac` is SCIP's fractionality, in `[0, 1)`; it was
  a signed remainder and negative for negative LP values.
- `scip.SetParam` and `scip.GetParam` report every failure as `*Error`,
  including an unsupported Go type and an `int` outside the `int32` range,
  like the typed methods.
- `Prober.SolveLp`, `Prober.SolveLpWithPricing` and `Diver.SolveLp` report
  an LP solver error as `*Error` instead of a bare `Retcode`.

### Changed

- Plugin families are named after one stem each: `Heuristic`,
  `Separator`, `Nodesel`, `Eventhdlr`, `Conshdlr`, across interfaces,
  builders, wrappers and `Include*` methods. The former names remain as
  deprecated aliases.
- `AddSol` takes a `*Solution` and invalidates it.
- Plugin callbacks receive a cached wrapper for their model instead of
  allocating one per call.

### Deprecated

- `NewHeur`, `HeurBuilder`, `HeurPlugin`, `Heur`, `Heurs`, `FindHeur`,
  `SetHeurPriority`, `NewSepa`, `SepaBuilder`, `SourceSepa`,
  `SetSepaPriority`, `SetPresolPriority`, `Presolver`, `NodeSel`,
  `NodeSelBuilder`, `EventHdlrBuilder` and their `Try` forms, in favour
  of the unified names. They will be removed in v1.0.0.

## [0.2.1] - 2026-09-03

### Changed

- The plugin wrapper types carry a `Plugin` suffix: `BranchRulePlugin`,
  `ConshdlrPlugin`, `EventhdlrPlugin`, `NodeselPlugin`, `PricerPlugin`,
  `SeparatorPlugin`, `HeuristicPlugin`, `PresolverPlugin`. The
  `SCIP`-prefixed names from 0.2.0 remain as deprecated aliases.
- The cgo boundary helpers moved from `ffi.go` to `cgo.go`.

## [0.2.0] - 2026-09-03

### Added

- Nonlinear constraints: the `Expr` tree with `Variable.Expr`, `Const`,
  `Sum`, `Product`, `Pow` and the other SCIP expression handlers,
  `ParseExpr` for SCIP's text syntax, `AddConsNonlinear`, and
  `ConsBuilder.Expression`.
- Getters for SCIP's built-in plugins: `Heuristics`, `Separators`,
  `Presolvers`, `FindHeuristic`, `FindSeparator`, `FindPresolver`, with
  priorities, frequencies and call statistics.

## [0.1.0] - 2026-09-03

### Added

- Initial port of russcip: the `Model` API, variable, constraint and row
  builders, the seven plugin kinds, probing and diving, the datastore,
  concurrent solving, and the example programs.

[0.3.0]: https://github.com/egoisutolabs/scipgo/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/egoisutolabs/scipgo/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/egoisutolabs/scipgo/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/egoisutolabs/scipgo/releases/tag/v0.1.0
