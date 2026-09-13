# Contributing

Thanks for taking the time. This page explains how to get a working
checkout, what the test suite expects, and the conventions that keep the
binding safe.

## Development setup

You need Go 1.25 or newer, a C compiler and SCIP 10 with headers; the
[installation guide](docs/installation.md) covers each platform. Then:

```bash
git clone https://github.com/egoisutolabs/scipgo.git
cd scipgo
go test ./...
```

The suite runs in well under a minute. CI runs it on Ubuntu against the
SCIP release package and on macOS against Homebrew's, together with
`gofmt`, `go vet` and a build of every example.

## Before opening a pull request

```bash
gofmt -l .                       # must print nothing
go vet ./...
go test ./...
for d in examples/*/; do (cd "$d" && go build -o /dev/null .) || exit 1; done
```

Keep commits focused and their messages plain: a short summary line, and a
body when the why is not obvious from the diff. The project does not use
commit trailers.

## Conventions

The binding sits on top of a C library that, in release builds, continues
into undefined behaviour after some misuse. The conventions below exist so
that no such misuse can reach it. Tests enforce several of them; a change
that breaks one fails the suite.

**Two forms of every fallible method.** A method that can fail against
SCIP is written once as `TryX`, returning `error`, and again as `X`, which
calls `TryX` and panics with the returned value through `must`. The
handful of methods that return an error under their plain name
(`ReadProb`, `Write`, `AddSol`, the `Set*Param` family) do so because
failure is a normal outcome for them.

**Guard before C.** Every method that touches SCIP first checks that the
model is alive (`guard`) and, for queries, that the current stage permits
the call (`query`, `mustStage`). Handle methods check the handle's
liveness and ownership first (`mustLive`, `checkHandle`). The stage sets
come from a table derived from SCIP's sources, and
`stages_table_test.go` checks the table against the getters it guards.

**Pin the instance across every C call.** Each method that calls into C
defers `runtime.KeepAlive` on the strong instance so that the garbage
collector cannot finalize it mid-call. `keepalive_test.go` scans the
package and fails when a C call lacks the pin.

**Handles go through their constructors.** `Variable`, `Constraint`,
`Solution`, `Node`, `Row`, `Col` and the plugin wrappers are created only
by the constructors in the package that record the owner and generation;
`handle_literals_test.go` rejects composite literals elsewhere.

**Errors carry the operation.** A failure is wrapped into `*Error` with
`Op` set to the exported method name (without the `Try` prefix), the
stage at the time, SCIP's return code and a detail string naming the
argument at fault. Argument validation before the C call reports
`RetcodeInvalidData`; a stage or liveness violation reports
`RetcodeInvalidCall`.

**Callbacks recover.** Every exported callback defers `catchPanic`, sets
its return code to `SCIP_ERROR` first and to `SCIP_OKAY` only at the end,
and never lets a panic reach C.

**Fix the class, not the instance.** When a review or a bug names one
place where a rule is broken, look for every other place with the same
pattern and fix them together, with a test that covers the pattern.

## Adding to the API

For a new `Model` method:

1. Add the internal call on `*Scip` in `scipptr.go` or the file for its
   area, returning `error`.
2. Add `TryX` on `Model` with the guard, the stage check if it is a query,
   the `KeepAlive` pin and the `wrap` into `*Error`; then `X` as the
   panicking form.
3. Add a test in the matching `_test.go` file. Cover the happy path, the
   freed-model case and the wrong-stage case.
4. Document it: a doc comment on both forms, and a line in the relevant
   guide under `docs/` if it is user-facing. Add a `CHANGELOG.md` entry
   under the section for the next release, adding an Unreleased section
   if there is none yet.

For a new plugin kind, mirror an existing one end to end: the interface
and result types, the C trampolines in `helpers.c` and `helpers.h`, the
exported Go callbacks in `callbacks.go`, the builder in
`pluginbuilders.go`, the `Include*` pair on `Model`, and a page under
`docs/plugins/`.

## Documentation

The guides under `docs/` are Markdown and are meant to be read on GitHub.
Code in them should compile against the current API; when an API changes,
grep the docs for the old name. Runnable examples for pkg.go.dev live in
`scip/example_test.go` and are checked by `go test`.

## Reporting issues

Include the SCIP version (`scip --version`), the platform, and a program
that reproduces the problem. If SCIP printed an error to stderr, include
that line; it names the C source location the failure came from.

## Releases

Releases are tagged from `main` once CI is green. Patch versions carry
fixes and additions; renames ship deprecated aliases and are removed no
earlier than the next major version.
