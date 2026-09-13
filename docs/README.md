# scipgo documentation

Guides for using scipgo, the Go binding for the SCIP optimisation suite.
The [API reference](https://pkg.go.dev/github.com/egoisutolabs/scipgo/scip)
on pkg.go.dev documents every type and method; these pages explain how the
pieces fit together.

## Start here

1. [Installation](installation.md): SCIP on macOS and Linux, custom
   locations, Docker, troubleshooting the build.
2. [Getting started](getting-started.md): a first model, the two API
   styles, reading a file, controlling the solve.

## Using the solver

- [Modeling](modeling.md): variables, objective, linear and specialised
  constraints, nonlinear expressions, reading and writing files.
- [Solving](solving.md): statuses, limits, stopping a solve, statistics,
  solving again, concurrent and exact solving.
- [Solutions](solutions.md): reading solutions, MIP starts, partial
  solutions.
- [Parameters](parameters.md): the parameter API, emphasis settings and
  the parameters worth knowing.
- [Logging](logging.md): routing SCIP's log and error output into Go.

## Understanding the binding

- [Errors](errors.md): the `Try` and panicking forms, `*scip.Error`,
  `*scip.CallbackPanic`, liveness.
- [Model lifecycle](lifecycle.md): stages, handles, releasing memory,
  goroutines.
- [Coming from russcip](russcip.md): the mapping between the Rust API and
  this one.

## Extending the solver

- [Plugins overview](plugins/README.md): how plugins are written and
  registered, sharing data, copies, panics, priorities.
- [Branch rules](plugins/branch-rules.md)
- [Constraint handlers](plugins/constraint-handlers.md)
- [Event handlers](plugins/event-handlers.md)
- [Heuristics](plugins/heuristics.md)
- [Node selectors](plugins/node-selectors.md)
- [Pricers](plugins/pricers.md)
- [Separators](plugins/separators.md)
- [Tree and LP access](tree-and-lp.md): nodes, rows, columns and LP
  values from inside a callback.
- [Probing and diving](probing-and-diving.md): tentative bound changes
  with LP support.

## Reference

- [Examples](../examples/README.md): eleven complete programs, from a
  first MIP to branch-and-price.
- [Changelog](../CHANGELOG.md)
- [Contributing](../CONTRIBUTING.md)
