# scipgo

[![Go Reference](https://pkg.go.dev/badge/github.com/egoisutolabs/scipgo/scip.svg)](https://pkg.go.dev/github.com/egoisutolabs/scipgo/scip)
[![CI](https://github.com/egoisutolabs/scipgo/actions/workflows/ci.yml/badge.svg)](https://github.com/egoisutolabs/scipgo/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/egoisutolabs/scipgo)](https://goreportcard.com/report/github.com/egoisutolabs/scipgo)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Go bindings for [SCIP](https://www.scipopt.org/), one of the fastest
non-commercial solvers for mixed integer programming (MIP) and mixed
integer nonlinear programming (MINLP). scipgo is a port of the Rust crate
[russcip](https://github.com/scipopt/russcip) and follows its API closely,
so the two are easy to move between.

```go
model := scip.DefaultModel().HideOutput().Maximize()
x := scip.NewVar().Name("x").Int().Obj(3).AddTo(model)
y := scip.NewVar().Name("y").Int().Obj(4).AddTo(model)
model.Add(
	scip.NewCons().Coef(x, 2).Coef(y, 1).Le(100),
	scip.NewCons().Coef(x, 1).Coef(y, 2).Le(80),
)

solved := model.Solve()
sol, _ := solved.BestSol()
fmt.Println(solved.Status(), sol.ObjVal(), sol.Val(x), sol.Val(y))
// Optimal 200 40 20
```

## Features

- **The whole modeling surface.** Continuous, integer, binary and implicit
  integer variables; linear, set partitioning, packing and covering,
  cardinality, SOS1, indicator, quadratic and general nonlinear
  constraints; expression trees and SCIP's own expression syntax; reading
  and writing LP, MPS and the other formats SCIP knows.
- **Plugins in Go.** Branching rules, primal heuristics, separators,
  pricers, constraint handlers, event handlers and node selectors are Go
  interfaces, registered with a builder. Panics in callbacks are captured
  and re-raised from `Solve` instead of crashing the process.
- **Safe by construction.** Every method exists in a panicking and an
  error-returning form. Every query checks the solver stage and the
  liveness of the model and handle before touching SCIP, so a call in the
  wrong stage, on a freed model, or with a handle from a freed or replaced
  problem produces a Go error instead of undefined behaviour.
- **Fits a Go service.** Solves stop on a `context.Context`. SCIP's log
  routes into an `io.Writer`, a `*slog.Logger` or a callback. Memory is
  released explicitly with `Free` or by a finalizer.
- **Concurrent and exact solving.** SCIP's parallel portfolio through
  `SolveConcurrent`, and end-to-end rational arithmetic through
  `EnableExactSolving` with `*big.Rat` results.

## Installation

scipgo links against an installed SCIP 10 through cgo. Nothing is bundled.

```bash
# macOS
brew install scip

# Ubuntu 22.04 (packages for other distributions on the SCIP releases page)
wget https://github.com/scipopt/scip/releases/download/v10.0.2/scipoptsuite_10.0.2-1+jammy_amd64.deb
sudo apt-get install -y ./scipoptsuite_10.0.2-1+jammy_amd64.deb

go get github.com/egoisutolabs/scipgo/scip
```

Go 1.25 or newer and a C compiler are required. SCIP in a custom location,
Docker images and build errors are covered in the
[installation guide](docs/installation.md).

## Documentation

The [documentation](docs/README.md) walks through the binding from the
first model to branch-and-price; the
[API reference](https://pkg.go.dev/github.com/egoisutolabs/scipgo/scip)
documents every method.

| Guide | Covers |
| --- | --- |
| [Getting started](docs/getting-started.md) | A first model, builders, reading a file, controlling the solve |
| [Modeling](docs/modeling.md) | Variables, every constraint kind, nonlinear expressions, file I/O |
| [Solving](docs/solving.md) | Statuses, limits, stopping a solve, statistics, re-solving, concurrent and exact modes |
| [Solutions](docs/solutions.md) | Reading solutions, MIP starts, partial solutions |
| [Parameters](docs/parameters.md) | The parameter API and the parameters worth knowing |
| [Logging](docs/logging.md) | Routing SCIP's log and error output |
| [Errors](docs/errors.md) | `Try` and panicking forms, error types, liveness |
| [Model lifecycle](docs/lifecycle.md) | Stages, handles, memory, goroutines |
| [Plugins](docs/plugins/README.md) | Writing branch rules, heuristics, separators, pricers, constraint handlers, event handlers and node selectors |
| [Coming from russcip](docs/russcip.md) | The mapping between the Rust API and this one |

## Examples

Eleven complete programs live under [`examples/`](examples/README.md),
each solving a real problem and checking its answer: a first MIP, a
knapsack, custom branching, node selection, event handling, a rounding
heuristic, a clique separator, TSP with subtour elimination, cutting stock
and bin packing by branch-and-price, and a concurrent solve. Run one from
its directory with `go run .`.

## Repository layout

| Path | Contents |
| --- | --- |
| `scip/` | The library, a single Go package. cgo glue, the `Model` API, builders and plugin callbacks live together because cgo's exported trampolines must sit in the package that owns the C helpers |
| `examples/` | Example programs |
| `docs/` | The guides |
| `data/test/` | Small LP and MPS instances used by the tests and examples |

## Status

scipgo is pre-1.0. The API is stable in shape, and renames ship with
deprecated aliases that stay until the next major version; see the
[changelog](CHANGELOG.md). It is tested on macOS and Linux against SCIP
10 on every push.

## Contributing

Bug reports, questions and pull requests are welcome. The
[contributing guide](CONTRIBUTING.md) covers the development setup, the
test suite and the conventions the code follows.

## License

scipgo is licensed under the [MIT License](LICENSE), Copyright (c) 2026
[Egoisuto Labs](https://egoisuto.com).

It is a port of [russcip](https://github.com/scipopt/russcip) by Mohammed
Ghannam and contributors, licensed under the Apache License 2.0. The
derived parts (API design, tests, examples, `data/test`) keep that
license; see [`LICENSE-russcip`](LICENSE-russcip) and [`NOTICE`](NOTICE),
and keep both files with any redistribution. [SCIP](https://www.scipopt.org)
itself is Apache-2.0 and is linked, not bundled.
