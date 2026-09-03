# scipgo

[![Go Reference](https://pkg.go.dev/badge/github.com/egoisutolabs/scipgo/scip.svg)](https://pkg.go.dev/github.com/egoisutolabs/scipgo/scip)
[![ci](https://github.com/egoisutolabs/scipgo/actions/workflows/ci.yml/badge.svg)](https://github.com/egoisutolabs/scipgo/actions/workflows/ci.yml)

Go (cgo) bindings for [SCIP](https://www.scipopt.org/), the solver for mixed
integer programming (MIP) and mixed integer nonlinear programming (MINLP).
This is a port of the Rust crate
[russcip](https://github.com/scipopt/russcip) to Go; the API follows
russcip's closely.

## Layout

- `scip/` — the library (a single Go package `scip`). All cgo glue, the
  `Model` API, builders, and plugin callbacks live here. (They cannot be
  split across packages: cgo `//export` trampolines must sit in the same
  package as the C helpers, and cgo C types are per-package.)
- `examples/` — example programs:
  - `create_and_solve` — build a MIP from scratch and solve it
  - `knapsack` — 0/1 knapsack via the variable builder API
  - `bin_packing`, `cutting_stock` — column generation with a pricer
  - `tsp` — subtour elimination with a constraint handler
  - `clique_separator` — custom separator
  - `most_infeasible_branching` — custom branching rule
  - `depth_first_node_selection` — custom node selector
  - `node_event_handler` — event handler
  - `random_rounding` — primal heuristic
  - `concurrent_solve` — SCIP's concurrent solvers
  (run from within an example's directory: `go run .`)
- `data/test/` — small LP/MPS instances used by the tests and examples
  (taken from russcip).

## Requirements

- Go 1.25+
- SCIP 10.x, e.g. `brew install scip` (macOS) or built from source. The
  default cgo flags look in `/opt/homebrew` and `/usr/local` on macOS and
  `/usr` on Linux; override with `CGO_CFLAGS`/`CGO_LDFLAGS` if SCIP lives
  elsewhere.

## Install

```bash
go get github.com/egoisutolabs/scipgo/scip
```

The package uses cgo, so a C compiler and a SCIP installation are needed on
the build machine (see Requirements). Nothing is bundled.

## Usage

```go
package main

import "github.com/egoisutolabs/scipgo/scip"

func main() {
	model := scip.DefaultModel().Minimize()
	x := model.AddVar(0, 1, 1, "x", scip.VarTypeBinary)
	y := model.AddVar(0, 1, 2, "y", scip.VarTypeBinary)
	model.AddCons([]scip.Variable{x, y}, []float64{1, 1}, 1, 1, "c")

	solved := model.Solve()
	fmt.Println(solved.Status(), solved.ObjVal())
}
```

With the builder API:

```go
model := scip.DefaultModel().Minimize()
x := scip.NewVar().Name("x").Bin().Obj(1).AddTo(model)
y := scip.NewVar().Name("y").Bin().Obj(2).AddTo(model)
model.Add(scip.NewCons().Name("c").Eq(1).Coef(x, 1).Coef(y, 1))
solved := model.Solve()
```

## Nonlinear constraints

Build an expression tree from variables and constants and add it as a
constraint; nothing touches SCIP until the constraint is added:

```go
x := model.AddVar(-1, 1, 1, "x", scip.VarTypeContinuous)
y := model.AddVar(-1, 1, 1, "y", scip.VarTypeContinuous)
model.AddConsNonlinear(x.Expr().Pow(2).Add(y.Expr().Pow(2)), scip.NegInfinity, 1, "disc")
// or in SCIP's own syntax, resolved by variable name when added:
model.AddConsNonlinear(scip.ParseExpr("<x>^2 + <y>^2"), scip.NegInfinity, 1, "disc")
// or through the builder, mixing a linear part:
model.Add(scip.NewCons().Expression(x.Expr().Mul(y.Expr())).Coef(x, 2).Le(1))
```

`Sum`, `WeightedSum`, `Product`, `Pow`, `SignPower`, `Exp`, `Log`, `Sin`,
`Cos`, `Abs` and `Entropy` cover SCIP's expression handlers.

## Plugins included by default

`Heurs`, `Separators` and `Presolvers` list SCIP's built-in plugins;
`FindHeur`, `FindSeparator` and `FindPresolver` look one up by name. Each
wrapper exposes its name, priority and statistics, and frequency/priority
setters let you tune or disable a plugin without touching parameter strings.

## Custom plugins

Plugins (branch rules, pricers, heuristics, separators, event handlers,
constraint handlers, node selectors) are Go interfaces registered through
builders, mirroring russcip's traits:

```go
type firstBranchRule struct{}

func (firstBranchRule) Execute(model scip.Model, _ scip.BranchRulePlugin,
	cands []scip.BranchingCandidate) scip.BranchingResult {
	return scip.BranchOn(cands[0])
}

model.Add(scip.NewBranchRule(firstBranchRule{}).Name("first"))
```

A constraint handler implements `Check` and `Enforce`; it may also implement
`ConshdlrEnfoPS` (pseudo solutions), `ConshdlrSepa` (LP separation) and
`ConshdlrProp` (propagation), which are registered only when present.

A plugin that implements `Copyable` (`Copy() any`) is copied into the
sub-SCIPs SCIP creates for LNS heuristics and `SolveConcurrent` workers,
like a C plugin with a copy callback. Return the receiver for stateless
plugins or a fresh object otherwise; copies in concurrent workers run on
the worker threads. Panics raised inside a copy surface from the `Solve`
of the model the user holds, and `GetData` inside a copy reads that model's
datastore. Plugins without `Copy` never run in sub-SCIPs; a constraint
handler without it marks every copy invalid, which disables SCIP's sub-MIP
heuristics.

## Differences from russcip

- Go has no destructors: `Diver`/`Prober` are ended with an explicit
  `End()` call, and SCIP instances are freed via finalizers or an explicit
  `Model.Free()`. `AddSol` takes a `*Solution` so it can invalidate the
  handle it consumes.
- Panics inside plugin callbacks cannot unwind through C; they are captured
  and re-raised when the enclosing `Solve` returns.
- The generic datastore (`SetData`/`GetData`) is a Go-side registry instead
  of a hidden plugin.
- SCIP's thread pool for `SolveConcurrent` is one process-wide global that
  each instance creates on its first concurrent solve and destroys when
  freed. The binding keeps those balanced, so several models may run
  concurrent solves during the process lifetime, but concurrent solves are
  serialised across goroutines.

## License

scipgo is licensed under the [MIT License](LICENSE), Copyright (c) 2026
[Egoisuto Labs](https://egoisuto.com).

It is a port of [russcip](https://github.com/scipopt/russcip) by Mohammed
Ghannam and contributors, licensed under the Apache License 2.0; the derived
parts (API design, tests, examples, `data/test`) keep that license, see
[`LICENSE-russcip`](LICENSE-russcip) and [`NOTICE`](NOTICE). Keep both files
with any redistribution. [SCIP](https://www.scipopt.org) itself is Apache-2.0
and is linked, not bundled.
