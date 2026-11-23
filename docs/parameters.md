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
model, err = model.SetCharParam("lp/initalgorithm", 'd')
model, err = model.TrySetCharParam("lp/pricing", 's')
model, err = model.SetStrParam("visual/vbcfilename", "tree.vbc")

freq := model.IntParam("display/freq") // panics on failure
freq, err = model.TryIntParam("display/freq")
algo := model.CharParam("lp/initalgorithm")
algo, err = model.TryCharParam("lp/initalgorithm")
```

`SetCharParam` and `TrySetCharParam` are the same call. Both return the
error, as the other `Set*Param` methods do. A character constant is a
`byte` argument when it fits in one byte.

`scip.SetParam` and `scip.GetParam` dispatch on the Go type instead:

```go
model, err := scip.SetParam(model, "limits/nodes", int64(10000))
var nodes int64
err = scip.GetParam(model, "limits/nodes", &nodes)

model, err = scip.SetParam(model, "lp/initalgorithm", 'd')
var algo byte
err = scip.GetParam(model, "lp/initalgorithm", &algo)
```

`SetParam` accepts `int`, `int32`, `int64`, `float32`, `float64`, `bool`,
`string` and `byte`. An `int` is sent as SCIP's `int` type, so a
long-integer parameter such as `limits/nodes` is passed as `int64`. A
`byte` sets a char parameter. A character literal does too when that
parameter is a char: Go gives a literal such as `'d'` the type `rune`, and
`rune` is an alias of `int32`, so `SetParam` reads the parameter's SCIP
type and sends an ASCII `int32` to the char setter for a char parameter.
The same `int32` sent to an integer parameter stays an integer, which
keeps `SetParam(model, "display/verblevel", int32(0))` on the integer
setter. `GetParam` takes a pointer of the matching type. A char parameter
is read into a `*byte`.

Every error from the parameter API, typed or generic, is a `*scip.Error`
whose `Retcode` tells the cause: `RetcodeParameterUnknown`,
`RetcodeParameterWrongType` or `RetcodeParameterWrongVal` from SCIP, and
`RetcodeInvalidData` when `SetParam` or `GetParam` is handed a Go type it
does not support, such as a `uint` value or a `*float32` destination. An
`int` outside the `int32` range is `RetcodeParameterWrongVal`. A rune
outside ASCII for a char parameter is `RetcodeInvalidData`, and that value
is rejected before the SCIP call. A value outside the parameter's allowed
set, such as `'z'` for `lp/pricing`, is `RetcodeParameterWrongVal` from
SCIP. Failures SCIP itself reports also print a line to stderr;
[Logging](logging.md) explains how to redirect that.

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
if b, ok := model.FindBranchRule("relpscost"); ok {
	model.SetBranchRulePriority(b, -1) // let another rule go first
}
if n, ok := model.FindNodesel("bfs"); ok {
	model.SetNodeselStdPriority(n, 0)
	model.SetNodeselMemSavePriority(n, 0)
}
```

`Heuristics`, `Separators`, `Presolvers`, `BranchRules`, `Nodesels`,
`Pricers`, `Conshdlrs`, `Eventhdlrs` and `Propagators` list every plugin
of that kind; the matching `Find*` method fetches one by name.
`SetHeuristicPriority`, `SetSeparatorPriority`, `SetPresolverPriority` and
`SetBranchRulePriority` change priorities. Node selectors have two,
`SetNodeselStdPriority` and `SetNodeselMemSavePriority`. Heuristics and
separators have `SetFreq`. A constraint handler reports `EnfoPriority` and
`CheckPriority`. A propagator reports `Priority`, `Freq`, `Timing` and its
call counts (`NCalls`, `NCutoffs`, `NDomredsFound`, `Time`).

Everything else on a built-in plugin, such as its `maxdepth`, is a
parameter named `heuristics/<name>/<setting>`,
`separating/<name>/<setting>`, `presolving/<name>/<setting>`,
`branching/<name>/<setting>`, `nodeselection/<name>/<setting>`,
`propagating/<name>/<setting>` or `constraints/<name>/<setting>`.

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
| `lp/initalgorithm` | char | s | Initial LP algorithm: `s` simplex, `p` primal, `d` dual, `b` barrier, `c` barrier with crossover |
| `lp/pricing` | char | l | LP pricing strategy, one of `l`, `a`, `f`, `p`, `s`, `q`, `d` |
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
