package scip

// cgo build settings and the few helpers that cross the Go/C boundary. The
// default flags find a Homebrew or /usr/local SCIP on macOS and a system SCIP
// on Linux; override with CGO_CFLAGS/CGO_LDFLAGS for other locations.

/*
#cgo darwin CFLAGS: -I/opt/homebrew/include -I/usr/local/include
#cgo darwin LDFLAGS: -L/opt/homebrew/lib -L/usr/local/lib -lscip
#cgo linux CFLAGS: -I/usr/include
#cgo linux LDFLAGS: -lscip
#include <stdlib.h>
#include <stdio.h>
#include "helpers.h"
*/
import "C"

import "unsafe"

// cString allocates a C string; pair with defer freeCString.
func cString(s string) *C.char { return C.CString(s) }

func freeCString(cs *C.char) { C.free(unsafe.Pointer(cs)) }

// goString converts a C string pointer into a Go string.
func goString(cs *C.char) string {
	if cs == nil {
		return ""
	}
	return C.GoString(cs)
}

// cBool converts a Go bool into a SCIP_Bool.
func cBool(b bool) C.SCIP_Bool {
	if b {
		return 1
	}
	return 0
}

// cInt converts a Go bool into the int flags the C shims in helpers.c take.
func cInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
