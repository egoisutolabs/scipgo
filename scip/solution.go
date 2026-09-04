package scip

/*
#include "helpers.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
)

// Solution is a wrapper for a SCIP solution. Solutions created via
// CreateSol/CreateOrigSol/CreatePartialSol are owned by the caller until
// passed to Model.AddSol, which consumes them.
type Solution struct {
	raw  *C.SCIP_SOL
	scip *Scip
	gen  uint64 // transform generation at creation; see handleErr
	orig bool   // original-problem handles survive FreeTransform
}

func (s *Scip) newSol(raw *C.SCIP_SOL) Solution {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	h := Solution{raw: raw, scip: s}
	if raw != nil {
		h.orig = C.SCIPsolIsOriginal(raw) != 0
		h.gen = s.gen(h.orig)
	}
	return h
}

// live panics with *Error unless the handle is usable; see handleErr.
func (h Solution) live(op string) { mustLive(op, "Solution", h.raw != nil, h.scip, h.gen, h.orig) }

// Inner returns the raw pointer to the underlying SCIP_SOL.
func (s Solution) Inner() *C.SCIP_SOL { return s.raw }

// ObjVal returns the objective value of the solution.
func (s Solution) ObjVal() float64 {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	s.live("Solution.ObjVal")
	mustStage("Solution.ObjVal", s.scip, stagesOrig)
	return float64(C.SCIPgetSolOrigObj(s.scip.raw, s.raw))
}

// Val returns the value of a variable in the solution.
func (s Solution) Val(v Variable) float64 {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	s.live("Solution.Val")
	mustStage("Solution.Val", s.scip, stagesOrig)
	must(Model{scip: s.scip}.checkVars("Solution.Val", v))
	return float64(C.SCIPgetSolVal(s.scip.raw, s.raw, v.raw))
}

// SetVal sets the value of a variable in the solution.
func (s Solution) SetVal(v Variable, val float64) {
	s.live("Solution.SetVal")
	must(s.TrySetVal(v, val))
}

// TrySetVal is SetVal returning an error instead of panicking.
func (s Solution) TrySetVal(v Variable, val float64) error {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	m := Model{scip: s.scip}
	if err := m.checkHandle("Solution.SetVal", "Solution", s.raw != nil, s.scip, s.gen, s.orig); err != nil {
		return err
	}
	if err := m.checkVars("Solution.SetVal", v); err != nil {
		return err
	}
	return m.call("Solution.SetVal", C.SCIPsetSolVal(s.scip.raw, s.raw, v.raw, C.double(val)))
}

// IsPartial returns whether this is a partial solution: unset variables are
// UNKNOWN rather than zero.
func (s Solution) IsPartial() bool {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	s.live("Solution.IsPartial")
	return C.SCIPsolIsPartial(s.raw) == 1
}

// AsNameMap returns the solution as a var-name to value map, skipping values
// that are zero within tolerance.
func (s Solution) AsNameMap() map[string]float64 {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	s.live("Solution.AsNameMap")
	mustStage("Solution.AsNameMap", s.scip, stagesTrans) // SCIPgetVars
	vars := C.SCIPgetVars(s.scip.raw)
	nVars := int(C.SCIPgetNVars(s.scip.raw))
	m := make(map[string]float64)
	for i := 0; i < nVars; i++ {
		v := cVarAt(vars, i)
		val := float64(C.SCIPgetSolVal(s.scip.raw, s.raw, v))
		eps := float64(C.SCIPepsilon(s.scip.raw))
		if val > eps || val < -eps {
			m[goString(C.SCIPvarGetName(v))] = val
		}
	}
	return m
}

// AsIDMap returns the solution as a var-probindex to value map, skipping
// values that are zero within tolerance.
func (s Solution) AsIDMap() map[int]float64 {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	s.live("Solution.AsIDMap")
	mustStage("Solution.AsIDMap", s.scip, stagesTrans) // SCIPgetVars
	vars := C.SCIPgetVars(s.scip.raw)
	nVars := int(C.SCIPgetNVars(s.scip.raw))
	m := make(map[int]float64)
	for i := 0; i < nVars; i++ {
		v := cVarAt(vars, i)
		val := float64(C.SCIPgetSolVal(s.scip.raw, s.raw, v))
		eps := float64(C.SCIPepsilon(s.scip.raw))
		if val > eps || val < -eps {
			m[int(C.SCIPvarGetProbindex(v))] = val
		}
	}
	return m
}

// String implements fmt.Stringer, mirroring the Rust Debug impl.
func (s Solution) String() string {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	s.live("Solution.String")
	mustStage("Solution.String", s.scip, stagesOrig) // SCIPgetOrigVars, SCIPgetSolVal
	var b strings.Builder
	fmt.Fprintf(&b, "Solution with obj val: %v\n", s.ObjVal())
	vars := C.SCIPgetOrigVars(s.scip.raw)
	nVars := int(C.SCIPgetNOrigVars(s.scip.raw))
	for i := 0; i < nVars; i++ {
		v := cVarAt(vars, i)
		val := float64(C.SCIPgetSolVal(s.scip.raw, s.raw, v))
		eps := float64(C.SCIPepsilon(s.scip.raw))
		if val > eps || val < -eps {
			fmt.Fprintf(&b, "Var %s=%v\n", goString(C.SCIPvarGetName(v)), val)
		}
	}
	return b.String()
}

// SolError represents an error that can occur when adding a solution.
type SolError int

const (
	// SolErrorInfeasible means the solution is infeasible.
	SolErrorInfeasible SolError = iota
)

// Error implements the error interface.
func (e SolError) Error() string {
	if e == SolErrorInfeasible {
		return "solution is infeasible"
	}
	return "unknown solution error"
}
