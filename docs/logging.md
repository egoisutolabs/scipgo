# Logging

SCIP writes its solver log to the process's stdout by default. The binding
can route it into Go per model, and route SCIP's error messages for the
whole process.

## Routing the solver log

Three adapters cover the usual sinks. Each takes whole lines, without the
trailing newline, assembled from the fragments SCIP emits:

```go
// An io.Writer: each line is written with a newline appended.
var buf bytes.Buffer
model := scip.NewModel().SetLogWriter(&buf).IncludeDefaultPlugins()

// A *slog.Logger: info and dialog lines become Info records, warnings Warn records.
model := scip.NewModel().SetLogger(slog.Default()).IncludeDefaultPlugins()

// A function, with the channel the line came from.
model := scip.NewModel().SetLogFunc(func(level scip.LogLevel, line string) {
	if level == scip.LogWarning {
		metrics.Inc("scip_warnings")
	}
	fmt.Println(line)
}).IncludeDefaultPlugins()
```

`LogLevel` distinguishes SCIP's channels: `LogInfo` is the normal solver
output, `LogWarning` is what SCIP emits through its warning channel, and
`LogDialog` is interactive-shell output, which a library user rarely sees.

Every adapter has a `Try` form that returns an error instead of panicking
when the sink cannot be installed. `SetLogFunc(nil)` restores SCIP's
default stdout handler.

### When to install it

SCIP only lets a message handler be installed while the problem is not
transformed: in the Init or Problem stage, before the first `Solve`. After
a solve, `FreeTransform` brings the model back to Problem and the sink can
be swapped again. Installing one at any other time is an error.

### What gets routed

Routing changes where lines go, not which lines SCIP produces.
`display/verblevel` still applies and `HideOutput` silences everything
before it reaches the sink. Output that SCIP writes to a file you named,
such as `WriteStatsJSON`, goes to that file and not to the sink.

### Rules for the callback

The sink is locked while your function runs, so the function must not call
into SCIP through any model and must not install another sink: a nested
message would wait for the lock forever. Keep the callback quick; a slow
sink slows the solver, since SCIP writes its log synchronously.

During a concurrent solve the handler is copied to the worker instances and
your function may be called from several threads at once. SCIP's callbacks
carry no worker identity, so lines from different workers can interleave
at fragment granularity, exactly as they do on SCIP's own stdout handler.
An `io.Writer` you pass must be safe for concurrent use if you use
`SolveConcurrent`; `bytes.Buffer` is not, `os.Stderr` and a `*slog.Logger`
are.

A callback that captures its model keeps that model reachable, so release
the model with `Free` rather than waiting for the finalizer.

## Error messages

SCIP's error messages, the `[paramset.c:1990] ERROR: ...` lines it prints
when a call fails, do not go through the message handler. They go through
one hook that is global to the process:

```go
scip.SetErrorLogFunc(func(fragment string) {
	log.Print(fragment)
})
defer scip.SetErrorLogFunc(nil) // back to stderr
```

Unlike the solver log, these arrive as fragments and are passed through
unbuffered, so a line may come in several pieces. Calls are serialised,
and the same rule applies: the function must not call into SCIP or call
`SetErrorLogFunc`. Passing nil restores stderr.

Because the hook is process-wide, a library that sets it affects every
model in the process. Set it once from `main`, not from library code.

## Reading the log

The lines worth matching if you parse the log:

- `SCIP Status        : problem is solved [optimal solution found]`, the
  final status.
- `Solving Time (sec) : 0.42`, and the `Solving Nodes` and `Primal Bound`
  lines that follow it.
- The progress table during the solve, one row per `display/freq` nodes,
  whose columns SCIP names in a header line.

Prefer the typed accessors (`Status`, `SolvingTime`, `NNodes`, `ObjVal`,
`StatsJSON`) over parsing when you can; the log format is not a stable
interface.
