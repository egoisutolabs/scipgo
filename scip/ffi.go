// Package scip provides Go bindings for the SCIP (Solving Constraint Integer
// Programs) optimization suite, mirroring the API of the Rust crate russcip.
//
// Example:
//
//	model := scip.DefaultModel().Minimize()
//	x := model.AddVar(0, 1, 1, "x", scip.VarTypeBinary)
//	y := model.AddVar(0, 1, 2, "y", scip.VarTypeBinary)
//	model.AddCons([]scip.Variable{x, y}, []float64{1, 1}, 1, 1, "c")
//
//	solved := model.Solve()
//	fmt.Println(solved.Status(), solved.ObjVal())
package scip

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

import (
	"fmt"
	"unsafe"
)

// SCIPBool converts a Go bool into a SCIP_Bool.
func SCIPBool(b bool) C.SCIP_Bool {
	if b {
		return 1
	}
	return 0
}

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

// scipInvalid mirrors SCIP's SCIP_INVALID macro (1e99).
const scipInvalid = 1e99

// Infinity is SCIP's notion of positive infinity.
const Infinity = 1e+20

// NegInfinity is SCIP's notion of negative infinity.
const NegInfinity = -1e+20

// mustOK panics if the given SCIP retcode is not SCIP_OKAY, mirroring the
// Rust scip_call_panic! macro.
func mustOK(rc C.SCIP_RETCODE) {
	if r := retcodeFromC(rc); r != RetcodeOkay {
		panic(fmt.Sprintf("SCIP call failed with retcode %v", r))
	}
}

// retcodeError converts a SCIP retcode into a Retcode error value.
func retcodeError(rc C.SCIP_RETCODE) error {
	if rc == C.SCIP_OKAY {
		return nil
	}
	return retcodeFromC(rc)
}
