# Parameters

SCIP has around two thousand parameters. This page covers how to set and
read them from Go and lists the ones most models need.

## Setting and reading

Typed methods exist for each of SCIP's parameter types. They return the
model for chaining and an error for an unknown name, a value of the wrong
type or a value out of range:

```go
model, err := model.SetIntParam("display/freq", 100)
model, err = model.SetLongintParam("limits/nodes", 10000)
model, err = model.SetRealParam("limits/gap", 0.01)
model, err = model.SetBoolParam("lp/presolving", false)
model, err = model.SetStrParam("visual/vbcfilename", "tree.vbc")

freq := model.IntParam("display/freq") // panics on failure
freq, err = model.TryIntParam("display/freq")
```

`scip.SetParam` and `scip.GetParam` dispatch on the Go type instead:

```go
model, err := scip.SetParam(model, "limits/nodes", int64(10000))
var nodes int64
err = scip.GetParam(model, "limits/nodes", &nodes)
```

`SetParam` accepts `int`, `int32`, `int64`, `float32`, `float64`, `bool`
and `string`. An `int` is sent as SCIP's `int` type, so a long-integer
parameter such as `limits/nodes` must be passed as `int64`. `GetParam`
takes a pointer of the matching type.

Every error from the parameter API, typed or generic, is a `*scip.Error`
whose `Retcode` tells the cause: `RetcodeParameterUnknown`,
`RetcodeParameterWrongType` or `RetcodeParameterWrongVal` from SCIP, and
`RetcodeInvalidData` when `SetParam` or `GetParam` is handed a Go type it
does not support, such as a `uint` value or a `*float32` destination. An
`int` outside the `int32` range is `RetcodeParameterWrongVal`. Failures
SCIP itself reports also print a line to stderr; [Logging](logging.md)
explains how to redirect that.

## Convenience methods

| Method | Parameter |
| --- | --- |
| `SetTimeLimit(seconds)` | `limits/time` |
| `SetMemoryLimit(mb)` | `limits/memory` |
| `HideOutput()`, `ShowOutput()`, `SetDisplayVerbosity(n)` | `display/verblevel` (0 silent, 4 default, 5 verbose) |
| `SetPresolving(s)`, `SetHeuristics(s)`, `SetSeparating(s)` | The emphasis settings, see below |
| `SetObjIntegral()` | Declares the objective integral |

The emphasis settings take a `ParamSetting`:

| Value | Effect |
| --- | --- |
| `ParamSettingDefault` | SCIP's defaults |
| `ParamSettingAggressive` | More effort in that component |
| `ParamSettingFast` | Less effort |
| `ParamSettingOff` | Disabled |

They set whole groups of parameters at once, the same way SCIP's
interactive shell does with `set presolving emphasis aggressive`.

## Tuning individual plugins

The plugin wrappers expose the settings people change most often without
going through parameter strings:

```go
if h, ok := model.FindHeuristic("rens"); ok {
	h.SetFreq(-1)                        // disable
}
if s, ok := model.FindSeparator("gomory"); ok {
	model.SetSeparatorPriority(s, 2000000) // ahead of every built-in separator
}
```

`SetHeuristicPriority`, `SetSeparatorPriority` and `SetPresolverPriority`
change priorities; heuristics and separators have `SetFreq`. Everything
else on a built-in plugin, such as its `maxdepth`, is a parameter named
`heuristics/<name>/<setting>`, `separating/<name>/<setting>` or
`presolving/<name>/<setting>`.

## Parameters worth knowing

| Parameter | Type | Default | Purpose |
| --- | --- | --- | --- |
| `limits/time` | real | 1e20 | Wall-clock limit in seconds |
| `limits/nodes` | longint | -1 | Node limit, -1 for none |
| `limits/gap` | real | 0 | Stop when the relative gap is below this |
| `limits/absgap` | real | 0 | Stop when the absolute gap is below this |
| `limits/solutions` | int | -1 | Stop after this many feasible solutions |
| `limits/memory` | real | 8796093022207 | Memory limit in MB |
| `display/verblevel` | int | 4 | 0 silences all output |
| `display/freq` | int | 100 | Print a log line every this many nodes |
| `randomization/randomseedshift` | int | 0 | Shift every random seed; use to test robustness |
| `parallel/maxnthreads` | int | 8 | Threads for `SolveConcurrent` |
| `parallel/mode` | int | 1 | 0 opportunistic, 1 deterministic |
| `lp/threads` | int | 0 | Threads for the LP solver, 0 for automatic |
| `presolving/maxrounds` | int | -1 | Presolving rounds, 0 disables presolving |
| `misc/usesymmetry` | int | 7 | Symmetry handling, 0 disables it |
| `numerics/feastol` | real | 1e-6 | Primal feasibility tolerance |
| `numerics/epsilon` | real | 1e-9 | Absolute zero tolerance |
| `branching/pscost/priority` | int | 2000 | Raise to make pseudo-cost branching the default rule |

The defaults are SCIP 10's; verify against your build with `GetParam`. The
complete list with descriptions is in the
[SCIP parameter reference](https://www.scipopt.org/doc/html/PARAMETERS.php).
Any setting shown there can be set by the same name here.

## Reproducibility

SCIP is deterministic for a fixed build, parameter set and input. Runs
differ between machines because floating-point LP results differ, and
between thread counts in opportunistic concurrent mode. For runs that must
match exactly, use the same SCIP build, avoid `SolveConcurrent` or set
`parallel/mode` to 1, and keep `randomization/randomseedshift` fixed.
