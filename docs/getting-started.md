# Getting started

This page builds a first model, solves it and reads the answer. It assumes
SCIP is installed; see [Installation](installation.md) if not.

## A model in twenty lines

```go
package main

import (
	"fmt"

	"github.com/egoisutolabs/scipgo/scip"
)

func main() {
	// maximize 3x + 4y  subject to  2x + y <= 100,  x + 2y <= 80,  x, y integer >= 0
	model := scip.NewModel().
		HideOutput().             // no solver log on stdout
		IncludeDefaultPlugins().  // SCIP's presolvers, heuristics, separators, ...
		CreateProb("example").
		Maximize()

	x := model.AddVar(0, scip.Infinity, 3, "x", scip.VarTypeInteger)
	y := model.AddVar(0, scip.Infinity, 4, "y", scip.VarTypeInteger)
	model.AddCons([]scip.Variable{x, y}, []float64{2, 1}, scip.NegInfinity, 100, "c1")
	model.AddCons([]scip.Variable{x, y}, []float64{1, 2}, scip.NegInfinity, 80, "c2")

	solved := model.Solve()
	fmt.Println("status:", solved.Status())     // Optimal
	fmt.Println("objective:", solved.ObjVal())  // 200

	if sol, ok := solved.BestSol(); ok {
		fmt.Println("x =", sol.Val(x), "y =", sol.Val(y)) // x = 40 y = 20
	}
	solved.Free()
}
```

Step by step:

1. **Create a SCIP instance.** `scip.NewModel()` allocates one. Nothing is
   loaded yet.
2. **Include plugins.** `IncludeDefaultPlugins` registers everything SCIP
   ships with. Without it the solver has no presolvers, heuristics,
   separators, branching rules or node selectors, and cannot solve
   anything. `scip.DefaultModel()` is the shortcut for these two steps
   plus `CreateProb("problem")`.
3. **Create a problem and set the sense.** `CreateProb` names the problem
   and moves the model into the Problem stage, where variables and
   constraints can be added. The default sense is minimisation.
4. **Add variables and constraints.** `AddVar` takes lower bound, upper
   bound, objective coefficient, name and type. `AddCons` takes variables,
   matching coefficients, a left-hand side, a right-hand side and a name.
   `scip.Infinity` and `scip.NegInfinity` make a side unbounded.
5. **Solve.** `Solve` runs presolving and branch-and-bound and returns the
   same model, now in the Solved stage, for chaining.
6. **Read the answer.** `Status` says how the solve ended, `ObjVal` is the
   best objective found and `BestSol` returns the incumbent, if there is one.
7. **Release the instance.** `Free` releases SCIP's memory now rather than
   when the garbage collector gets around to the finalizer.

## The builder style

Every constructor also has a fluent builder. The same model reads:

```go
model := scip.DefaultModel().HideOutput().Maximize()
x := scip.NewVar().Name("x").Int().Obj(3).AddTo(model)
y := scip.NewVar().Name("y").Int().Obj(4).AddTo(model)
model.Add(
	scip.NewCons().Name("c1").Coef(x, 2).Coef(y, 1).Le(100),
	scip.NewCons().Name("c2").Coef(x, 1).Coef(y, 2).Le(80),
)
solved := model.Solve()
```

`NewVar` defaults to a continuous variable in `[0, +inf)` with objective
coefficient zero; `Bin`, `Int`, `IntRange`, `ContRange` and friends change
that. `NewCons` defaults to `-inf <= expr <= +inf`; `Eq`, `Le`, `Ge` and
`Bounds` set the sides. A builder does nothing until `AddTo` (which returns
the created object) or `Model.Add` (which discards it). Plugins are added
the same way, so a whole model can be assembled in one `Add` call.

Both styles are equivalent and can be mixed. The positional form is
shorter for dense loops; the builder form reads better for hand-written
models. See [Modeling](modeling.md) for everything the builders can do.

## Reading a problem from a file

SCIP reads LP, MPS, OPB, CIP, ZIMPL and several other formats, chosen by
file extension:

```go
model, err := scip.NewModel().IncludeDefaultPlugins().ReadProb("instance.mps")
if err != nil {
	return err
}
solved := model.Solve()
```

`ReadProb` creates the problem itself, so `CreateProb` is not needed. Note
that it is one of the few methods that returns an error under its plain
name: file errors are expected in normal operation.

## Controlling the solve

The most common knobs have dedicated methods, all chainable:

```go
model := scip.DefaultModel().
	SetTimeLimit(30).                         // seconds
	SetMemoryLimit(4096).                     // MB
	SetPresolving(scip.ParamSettingAggressive).
	SetHeuristics(scip.ParamSettingOff).
	HideOutput()
```

Everything else is a SCIP parameter, set by name:

```go
model, err := scip.SetParam(model, "limits/gap", 0.01)
```

See [Parameters](parameters.md) for the full API and a list of the
parameters worth knowing, and [Solving](solving.md) for time limits,
cancellation from a `context.Context`, statistics and re-solving.

## Errors: panic or return

Every method that can fail against SCIP exists in two forms. `AddVar`
panics on failure; `TryAddVar` returns an error. The panic value and the
returned error are the same `*scip.Error`, so the two styles can be mixed
freely:

```go
x, err := model.TryAddVar(0, 1, 1, "x", scip.VarTypeBinary)
if err != nil {
	return err
}
```

Use the plain form in scripts, tests and plugin callbacks where a failure
is a bug, and the `Try` form in services where a failure is an outcome to
report. [Errors](errors.md) covers the error types, what they carry and
how to inspect them.

## Where to go next

- [Modeling](modeling.md): variable types, every constraint kind, nonlinear
  expressions, file input and output.
- [Solving](solving.md): statuses, limits, stopping a solve, statistics,
  re-solving, concurrent and exact solving.
- [Solutions](solutions.md): reading solutions and supplying starting
  solutions.
- [Plugins](plugins/README.md): extend SCIP with Go implementations of
  branching rules, heuristics, pricers, separators, constraint handlers,
  event handlers and node selectors.
- [Examples](../examples/README.md): eleven complete programs.
