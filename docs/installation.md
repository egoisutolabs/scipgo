# Installation

scipgo is a cgo binding. Building a program that imports it needs three
things on the build machine:

| Requirement | Notes |
| --- | --- |
| Go 1.25 or newer | `go version` |
| A C compiler | Xcode Command Line Tools on macOS, `build-essential` or `gcc` on Linux |
| SCIP 10.x, shared library and headers | Not bundled. See the platform sections below |

Nothing is vendored: the package links against the `libscip` that is
installed on the machine, so the same binary picks up whatever SCIP is
present at run time.

## macOS

```bash
brew install scip
go get github.com/egoisutolabs/scipgo/scip
```

The default cgo flags look in `/opt/homebrew` (Apple silicon) and
`/usr/local` (Intel), so a Homebrew SCIP works without any configuration.

## Linux

The SCIP Optimization Suite publishes Debian and Ubuntu packages with every
release. This is what the project's CI uses:

```bash
SCIP_VERSION=10.0.2
wget https://github.com/scipopt/scip/releases/download/v${SCIP_VERSION}/scipoptsuite_${SCIP_VERSION}-1+jammy_amd64.deb
sudo apt-get install -y ./scipoptsuite_${SCIP_VERSION}-1+jammy_amd64.deb
go get github.com/egoisutolabs/scipgo/scip
```

The package installs into `/usr`, which the default cgo flags already search.
Pick the package matching your distribution from the
[SCIP releases page](https://github.com/scipopt/scip/releases): there are
`.deb` packages for Ubuntu 22.04 and 24.04 and for Debian bullseye,
bookworm and trixie, on amd64 and, for the Debian ones, arm64 and armhf,
plus generic Linux tarballs for glibc 2.28 and 2.34 that unpack into a
prefix of your choice (see the next section for pointing cgo at it).

### Building SCIP from source

If no package fits, build the suite as a shared library and install it:

```bash
tar xf scipoptsuite-10.0.2.tgz && cd scipoptsuite-10.0.2
cmake -B build -DCMAKE_BUILD_TYPE=Release -DSHARED=ON -DCMAKE_INSTALL_PREFIX=/opt/scip
cmake --build build --parallel
sudo cmake --install build
```

Then point cgo at it, as described in the next section.

## SCIP in a custom location

Override the include and library paths through the standard cgo
environment variables. The values are read at build time, so export them in
the shell that runs `go build`, `go test` or `go run`:

```bash
export CGO_CFLAGS="-I/opt/scip/include"
export CGO_LDFLAGS="-L/opt/scip/lib -Wl,-rpath,/opt/scip/lib -lscip"
```

The `-rpath` flag records the library directory in the binary, so it can be
run without setting `LD_LIBRARY_PATH` (Linux) or `DYLD_LIBRARY_PATH`
(macOS). Leave it out if you would rather set those at run time.

## Verify the setup

```bash
go test github.com/egoisutolabs/scipgo/scip
```

The suite takes well under a minute and exercises every part of the binding
against the installed SCIP, so a green run means the toolchain, headers and
library are all wired up. For a faster check, print the version SCIP
reports:

```go
package main

import "github.com/egoisutolabs/scipgo/scip"

func main() { scip.NewModel().PrintVersion() }
```

## Docker

A minimal image for a service that uses scipgo:

```dockerfile
FROM golang:1.25-bookworm AS build
ARG SCIP_VERSION=10.0.2
RUN apt-get update && apt-get install -y wget && \
    wget -q https://github.com/scipopt/scip/releases/download/v${SCIP_VERSION}/scipoptsuite_${SCIP_VERSION}-1+bookworm_amd64.deb && \
    apt-get install -y ./scipoptsuite_${SCIP_VERSION}-1+bookworm_amd64.deb
WORKDIR /src
COPY . .
RUN go build -o /out/app ./cmd/app

FROM debian:bookworm-slim
COPY --from=build /usr/lib/libscip* /usr/lib/
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
```

Check the releases page for the exact package name of the Debian or Ubuntu
version you build on. The run-time image only needs the shared library, not
the headers.

## Troubleshooting

**`fatal error: scip/scip.h: No such file or directory`**
The compiler cannot find SCIP's headers. Install a SCIP package that ships
them, or set `CGO_CFLAGS` to the directory that contains `scip/scip.h`.

**`ld: library 'scip' not found` or `cannot find -lscip`**
The linker cannot find `libscip`. Set `CGO_LDFLAGS` with `-L` pointing at
the directory that holds `libscip.so` or `libscip.dylib`.

**`undefined reference to 'SCIPenableExactSolving'` or similar**
The SCIP found at link time is older than 10. The binding uses SCIP 10 APIs
throughout, so a SCIP 9 installation cannot be linked. Check
`scip --version` and which library directory the linker searches first.

**`Library not loaded: @rpath/libscip...` or `error while loading shared libraries: libscip.so.10`**
The program built, but the dynamic loader cannot find the library at run
time. Either add `-Wl,-rpath,<libdir>` to `CGO_LDFLAGS` and rebuild, or set
`DYLD_LIBRARY_PATH` (macOS) or `LD_LIBRARY_PATH` (Linux) when running.

**`build constraints exclude all Go files in .../scipgo/scip`**
cgo is disabled, usually because `CGO_ENABLED=0` is set (cross-compilation
does this by default). scipgo cannot be built without cgo, and
cross-compiling requires a C cross-toolchain and a SCIP built for the
target.

**`TrySolveConcurrent` returns an error immediately**
SCIP was built without thread support (its TPI). The Homebrew and release
packages have it; a source build needs `-DTPI=tny` or `-DTPI=omp`.

**Exact solving fails to enable**
Exact mode needs a SCIP built with rational arithmetic support. The
Homebrew and release packages include it. A source build needs GMP, MPFR
and Boost available at configure time; check the SCIP install notes.

**Console shows `[paramset.c:...] ERROR: parameter <...> unknown`**
That is SCIP's own error log, printed to stderr alongside the `*scip.Error`
the call returned. Route or silence it with `scip.SetErrorLogFunc`; see
[Logging](logging.md).
