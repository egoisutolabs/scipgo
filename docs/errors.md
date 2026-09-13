# Errors

SCIP reports failures through return codes and, in release builds, keeps
going after some misuse in ways that end in undefined behaviour. The
binding turns every such case into a Go error before it reaches C, and
gives you the choice of receiving it as a panic or a return value.

## Two forms of every method

Every `Model` method that can fail against SCIP exists twice:

```go
x := model.AddVar(0, 1, 1, "x", scip.VarTypeBinary)        // panics on failure
x, err := model.TryAddVar(0, 1, 1, "x", scip.VarTypeBinary) // returns it
```

The panic value and the returned error are the same object, so a recovered
panic can be handled exactly like a returned error. Pick per call site:
the plain form suits scripts, tests and plugin callbacks, where a failure
is a bug; the `Try` form suits services, where a failure is an outcome to
report.

Mutators on `Prober`, `Diver`, `Row`, `Solution` and `Node` follow the same
pattern. The plugin builders have `AddTo` and `TryAddTo`.

A few methods return an error under their plain name because failure is a
normal outcome, not a bug: `ReadProb`, `Write`, `AddSol`, `WriteStatsJSON`
and the `Set*Param` family.

## What the queries do

Getters such as `Status`, `NVars`, `ObjVal`, the tree and LP accessors, and
every method on `Variable`, `Constraint`, `Solution`, `Node`, `Row` and
`Col` check two things before touching SCIP:

- that the model is alive and the handle belongs to a problem that still
  exists, and
- that SCIP permits the query in the current [stage](lifecycle.md#stages).

A violation panics with a `*scip.Error`. SCIP itself would print an error
and continue into undefined behaviour in release builds, so the panic is
the safe outcome. The queries a service might call defensively have `Try`
forms too: `TryStatus`, `TryObjVal`, `TryBestSol`, `TryNVars`, `TryVars`,
`TryConss`, `TryBestBound`, `TrySolvingTime`, `TryStatsJSON`,
`TryFocusNode` and the parameter getters.

The optional tree accessors (`BestNode`, `BestLeaf`, `BestChild`,
`BestSibling`, `PrioChild`, `PrioSibling`, `BestBoundNode`) return nil
rather than panicking when there is no such node, including outside the
solving stage.

## `*scip.Error`

```go
type Error struct {
	Op      string  // the scipgo method that failed, e.g. "AddVar"
	Stage   Stage   // SCIP stage at the time of the call
	Retcode Retcode // SCIP's return code
	Detail  string  // optional context: a parameter, plugin or variable name
	Cause   error   // optional underlying error, e.g. a cancelled context
}
```

Its message reads `scip: AddVar in stage Solved: SCIP_INVALIDCALL (x)`.
`Unwrap` yields the `Retcode` and the `Cause`, so `errors.Is` works against
both:

```go
solved, err := model.SolveContext(ctx)
var e *scip.Error
switch {
case errors.As(err, &e) && errors.Is(err, context.DeadlineExceeded):
	// stopped by the deadline; solved holds the interrupted model
case errors.Is(err, scip.RetcodeInvalidCall):
	// called in a stage that does not permit it
case err != nil:
	log.Printf("%s failed in stage %s: %v", e.Op, e.Stage, e.Retcode)
}
```

The `Retcode` values mirror SCIP's `SCIP_RETCODE`. The ones you will meet:

| Retcode | When |
| --- | --- |
| `RetcodeInvalidCall` | The method is not allowed in the current stage, or the model or handle is dead |
| `RetcodeInvalidData` | An argument was rejected: a zero handle, a handle from another model, an unknown enum value, a nil plugin, a malformed expression |
| `RetcodeParameterUnknown`, `RetcodeParameterWrongType`, `RetcodeParameterWrongVal` | Parameter API misuse |
| `RetcodeNoFile`, `RetcodeReadError`, `RetcodeWriteError`, `RetcodeFileCreateError` | File I/O |
| `RetcodeLpError` | The LP solver failed |
| `RetcodeNoMemory` | SCIP ran out of memory |
| `RetcodePluginNotFound`, `RetcodeKeyAlreadyExisting` | A plugin is missing, or one with that name already exists |
| `RetcodeError` | An unspecified SCIP error, also used when a callback reported failure |

`Retcode` implements `error` itself, so it can be returned on its own; the
binding always wraps it in `*Error` to add the operation and stage.

## `*scip.CallbackPanic`

A panic inside a plugin callback cannot unwind through SCIP's C frames.
The binding recovers it, makes the callback report failure to SCIP, lets
SCIP unwind, and then hands the panic back from the call that triggered
the callback:

```go
solved, err := model.TrySolve()
var cp *scip.CallbackPanic
if errors.As(err, &cp) {
	log.Printf("plugin %s panicked: %v", cp.Plugin, cp.Value)
	for _, more := range cp.More {
		log.Printf("and %s: %v", more.Plugin, more.Value)
	}
}
```

`Plugin` names the plugin kind and Go type, such as
`heuristic *main.myHeur`; `Value` is the recovered panic value; `More`
collects further panics from the same solve, since SCIP may keep calling
plugins while it unwinds. `Solve` re-panics with the `*CallbackPanic`;
`TrySolve`, `TrySolveConcurrent`, `TryFreeTransform` and `AddSol` return
it. The model is usable afterwards.

Panics that escape a `Copy` call of a `Copyable` plugin surface the same
way, from the `Solve` of the model you hold.

## `scip.SolError`

`AddSol` returns `scip.SolErrorInfeasible` when SCIP checked the solution
and would not store it. This is the expected outcome for a heuristic that
guessed wrong, not a failure of the binding.

## Liveness

Handles are judged by instance, not by pointer, so a stale handle is
always detected:

- A handle from a freed model reports `RetcodeInvalidCall` with stage
  `Free`.
- A transformed variable, row, column, node or transformed constraint
  used after `FreeTransform` reports `RetcodeInvalidCall`: it belongs to a
  problem that was freed. Original variables, constraints and solutions
  survive `FreeTransform`.
- Any handle used after `CreateProb` or `ReadProb` replaced the problem
  reports the same.
- A handle passed to a method of a different model reports
  `RetcodeInvalidData`.
- The zero value of a handle type, such as the `Variable` a failed lookup
  returns, reports `RetcodeInvalidData`.

Handles minted inside a plugin callback are bound to the model the
callback ran on, and die with it. A handle from a sub-SCIP copy (a
`Copyable` plugin running in a heuristic's sub-MIP or a concurrent worker)
must not be passed to the parent model; that too is detected.

## Recovering from a panic

Because the plain forms panic with the same values the `Try` forms return,
a wrapper that converts one style to the other is short:

```go
func solve(model scip.Model) (solved scip.Model, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	return model.Solve(), nil
}
```

In practice the `Try` forms make this unnecessary.
