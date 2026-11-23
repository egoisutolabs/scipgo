# `make verify` is the full verification gate: the same checks CI runs on
# Linux. Score runs it on a staged merge tree before landing any PR, and a
# nonzero exit blocks the merge.
#
# Needs the SCIP C library (10.x). If it is not installed system-wide, point
# cgo at it, e.g.:
#   go env -w CGO_CFLAGS="-I$PREFIX/include" \
#             CGO_LDFLAGS="-L$PREFIX/lib -Wl,-rpath,$PREFIX/lib"
.PHONY: verify fmt-check vet test examples

verify: fmt-check vet test examples

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "These files need gofmt:"; echo "$$unformatted"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

examples:
	@for d in examples/*/; do (cd "$$d" && go build -o /dev/null .) || exit 1; done
