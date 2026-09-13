package scip

/*
#include "helpers.h"
*/
import "C"

import (
	"math/big"
	"runtime"
	"unsafe"
)

// TryEnableExactSolving enables or disables SCIP's exact solving mode, in
// which the solver works in rational arithmetic end to end: LP solves,
// bounds and solutions carry no floating-point roundoff. SCIP only allows
// the switch in the Init stage — before plugins are included or a problem is
// created or read — so call it on a fresh Model; the whole solve then runs
// exact. In exact mode Solution.ValExact and Model.ObjValExact report the
// rational values; the float64 accessors return their projections.
func (m Model) TryEnableExactSolving(enable bool) error {
	defer runtime.KeepAlive(m.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if err := m.guard("EnableExactSolving"); err != nil {
		return err
	}
	if st := m.scip.stage(); st != StageInit {
		return m.invalid("EnableExactSolving", RetcodeInvalidCall,
			"exact solving mode can only be switched on a fresh model (stage Init)")
	}
	return m.call("EnableExactSolving", C.scipgo_enableExact(m.scip.raw, cBool(enable)))
}

// EnableExactSolving enables SCIP's exact solving mode; see
// TryEnableExactSolving. It panics on failure.
func (m Model) EnableExactSolving() Model {
	must(m.TryEnableExactSolving(true))
	return m
}

// ratExact parses the helper's malloc'd string into a Rat and frees it;
// m is the model the error, if any, is reported through.
func (m Model) ratExact(op string, cs *C.char, err error) (*big.Rat, error) {
	if err != nil {
		return nil, err
	}
	if cs == nil {
		return nil, m.invalid(op, RetcodeError, "no exact value")
	}
	str := goString(cs)
	C.free(unsafe.Pointer(cs))
	r, ok := new(big.Rat).SetString(str)
	if !ok {
		return nil, m.invalid(op, RetcodeInvalidData, "unparsable rational "+str)
	}
	return r, nil
}

// TryValExact returns the value of a variable in the solution as an exact
// rational. Only solutions of a solve in exact mode (see
// TryEnableExactSolving) carry rational values; on any other instance the
// call is rejected rather than crashing SCIP, which has no exact value to
// report.
func (s Solution) TryValExact(v Variable) (*big.Rat, error) {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	m := Model{scip: s.scip}
	if err := m.checkHandle("Solution.ValExact", "Solution", s.raw != nil, s.scip, s.gen, s.orig); err != nil {
		return nil, err
	}
	if C.SCIPisExact(s.scip.raw) == 0 {
		return nil, m.invalid("Solution.ValExact", RetcodeInvalidCall, "solution was not produced by an exact solve")
	}
	if err := m.checkVars("Solution.ValExact", v); err != nil {
		return nil, err
	}
	var cs *C.char
	rc := C.scipgo_solValExact(s.scip.raw, s.raw, v.raw, &cs)
	return m.ratExact("Solution.ValExact", cs, m.call("Solution.ValExact", rc))
}

// ValExact returns the value of a variable in the solution as an exact
// rational; see TryValExact. It panics on failure.
func (s Solution) ValExact(v Variable) *big.Rat {
	r, err := s.TryValExact(v)
	must(err)
	return r
}

// TryObjValExact returns the objective value of the solution as an exact
// rational; see TryValExact for when that exists.
func (s Solution) TryObjValExact() (*big.Rat, error) {
	defer runtime.KeepAlive(s.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	m := Model{scip: s.scip}
	if err := m.checkHandle("Solution.ObjValExact", "Solution", s.raw != nil, s.scip, s.gen, s.orig); err != nil {
		return nil, err
	}
	if C.SCIPisExact(s.scip.raw) == 0 {
		return nil, m.invalid("Solution.ObjValExact", RetcodeInvalidCall, "solution was not produced by an exact solve")
	}
	var cs *C.char
	rc := C.scipgo_solOrigObjExact(s.scip.raw, s.raw, &cs)
	return m.ratExact("Solution.ObjValExact", cs, m.call("Solution.ObjValExact", rc))
}

// ObjValExact returns the objective value of the solution as an exact
// rational; see TryObjValExact. It panics on failure.
func (s Solution) ObjValExact() *big.Rat {
	r, err := s.TryObjValExact()
	must(err)
	return r
}
