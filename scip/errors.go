package scip

/*
#include "helpers.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"reflect"
)

// Stage is a SCIP solving stage, as reported by Model.Stage and carried by
// Error.
type Stage int

// SCIP stages, in lifecycle order.
const (
	StageInit         Stage = iota // SCIP created, no problem yet
	StageProblem                   // problem is being built
	StageTransforming              // problem is being transformed
	StageTransformed               // transformed problem exists
	StageInitPresolve              // presolving is being initialised
	StagePresolving                // presolving
	StageExitPresolve              // presolving is finishing
	StagePresolved                 // presolved problem exists
	StageInitSolve                 // solving is being initialised
	StageSolving                   // branch-and-bound is running
	StageSolved                    // solving finished
	StageExitSolve                 // solving data is being freed
	StageFreeTrans                 // transformed problem is being freed
	StageFree                      // instance is being or has been freed
)

func (s Stage) String() string {
	names := [...]string{"Init", "Problem", "Transforming", "Transformed", "InitPresolve",
		"Presolving", "ExitPresolve", "Presolved", "InitSolve", "Solving", "Solved",
		"ExitSolve", "FreeTrans", "Free"}
	if int(s) < 0 || int(s) >= len(names) {
		return fmt.Sprintf("Stage(%d)", int(s))
	}
	return names[s]
}

// stage reads the instance's current stage; a freed instance reports StageFree.
func (s *Scip) stage() Stage {
	if !s.alive() {
		return StageFree
	}
	return Stage(int(C.SCIPgetStage(s.raw))) // SCIP_STAGE_* is a dense enum from 0
}

// Stage returns the current SCIP stage of the model.
func (m Model) Stage() Stage { return m.scip.stage() }

// Error is returned by every Try* method when SCIP reports a failure, and is
// the value the corresponding panicking method panics with.
type Error struct {
	Op      string  // the scipgo method that failed, e.g. "AddVar"
	Stage   Stage   // SCIP stage at the time of the call
	Retcode Retcode // SCIP's return code
	Detail  string  // optional context: a parameter, plugin or variable name
}

func (e *Error) Error() string {
	s := fmt.Sprintf("scip: %s in stage %s: %s", e.Op, e.Stage, e.Retcode)
	if e.Detail != "" {
		s += " (" + e.Detail + ")"
	}
	return s
}

// Unwrap returns the Retcode, so errors.Is(err, scip.RetcodeInvalidCall) works.
func (e *Error) Unwrap() error { return e.Retcode }

// CallbackPanic is returned by TrySolve, TrySolveConcurrent, TryFreeTransform
// and AddSol when a panic escaped a plugin callback during the call. Panics
// cannot unwind through SCIP's C frames, so the binding recovers them, makes
// the callback report SCIP_ERROR, and hands them back here. The panicking
// forms (Solve, FreeTransform) panic with the *CallbackPanic.
type CallbackPanic struct {
	Plugin string           // plugin kind and Go type, e.g. "heuristic *main.myHeur"
	Value  any              // the recovered panic value
	More   []*CallbackPanic // further panics from the same call, if any
}

func (p *CallbackPanic) Error() string {
	s := fmt.Sprintf("scip: panic in %s callback: %v", p.Plugin, p.Value)
	if len(p.More) > 0 {
		s += fmt.Sprintf(" (and %d more)", len(p.More))
	}
	return s
}

// guard rejects calls on a freed or zero Model before they reach C.
func (m Model) guard(op string) error {
	if !m.scip.alive() {
		return &Error{Op: op, Stage: StageFree, Retcode: RetcodeInvalidCall, Detail: "model is freed or zero"}
	}
	return nil
}

// wrap converts an error from the internal layer into *Error for op. Retcode
// values keep their code; other errors (argument validation, expression
// building) become RetcodeInvalidData with the message as detail.
func (m Model) wrap(op string, err error, detail string) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	var cp *CallbackPanic
	if errors.As(err, &cp) {
		return cp
	}
	var rc Retcode
	if errors.As(err, &rc) {
		if err != rc { // wrapped with context (e.g. expression building): keep it
			detail = joinDetail(detail, err.Error())
		}
		return &Error{Op: op, Stage: m.scip.stage(), Retcode: rc, Detail: detail}
	}
	return &Error{Op: op, Stage: m.scip.stage(), Retcode: RetcodeInvalidData, Detail: joinDetail(detail, err.Error())}
}

func joinDetail(a, b string) string {
	if a == "" {
		return b
	}
	return a + ": " + b
}

// handleErr is the one liveness rule for every handle and plugin wrapper,
// shared by the panicking methods (via mustLive) and the Try forms (via
// checkHandle) so both report the same Retcode:
//   - the zero value a failed lookup returns          -> InvalidData
//   - its model has been freed                        -> InvalidCall
//   - it points into a transformed problem that
//     FreeTransform has since released (gen moved on)  -> InvalidCall
//   - it belongs to a different model than m           -> InvalidData
//
// Ownership and liveness are judged by instance identity (Scip.root), not
// by raw pointer, so handles minted inside callbacks with a weak wrapper are
// accepted by their owner and die with it.
func handleErr(op, what string, raw bool, owner *Scip, gen uint64, orig bool, m *Scip) *Error {
	switch {
	case !raw:
		return &Error{Op: op, Stage: owner.stage(), Retcode: RetcodeInvalidData, Detail: "zero " + what}
	case !owner.alive():
		return &Error{Op: op, Stage: StageFree, Retcode: RetcodeInvalidCall, Detail: what + " belongs to a freed model"}
	case !orig && gen != owner.gen():
		return &Error{Op: op, Stage: owner.stage(), Retcode: RetcodeInvalidCall, Detail: what + " belongs to a transformed problem that was freed"}
	case m != nil && owner.root() != m.root():
		return &Error{Op: op, Stage: m.stage(), Retcode: RetcodeInvalidData, Detail: what + " belongs to another model"}
	}
	return nil
}

// mustLive panics with the handleErr of a handle, if any. It is the first
// statement of every handle method.
func mustLive(op, what string, raw bool, owner *Scip, gen uint64, orig bool) {
	if e := handleErr(op, what, raw, owner, gen, orig, nil); e != nil {
		panic(e)
	}
}

// checkHandle returns the handleErr of a handle passed to a method of m.
func (m Model) checkHandle(op, what string, raw bool, owner *Scip, gen uint64, orig bool) error {
	if e := handleErr(op, what, raw, owner, gen, orig, m.scip); e != nil {
		return e
	}
	return nil
}

// checkVars validates every Variable; see handleErr.
func (m Model) checkVars(op string, vars ...Variable) error {
	for _, v := range vars {
		if err := m.checkHandle(op, "Variable", v.raw != nil, v.scip, v.gen, v.orig); err != nil {
			return err
		}
	}
	return nil
}

// checkCons validates a Constraint; see handleErr.
func (m Model) checkCons(op string, c Constraint) error {
	return m.checkHandle(op, "Constraint", c.raw != nil, c.scip, c.gen, c.orig)
}

// checkNode validates a *Node; see handleErr.
func (m Model) checkNode(op string, n *Node) error {
	if n == nil {
		return m.invalid(op, RetcodeInvalidData, "nil Node")
	}
	return m.checkHandle(op, "Node", n.raw != nil, n.scip, n.gen, false)
}

// isNilPlugin reports whether a plugin interface value is nil, including a
// typed nil pointer stored in it, which == nil does not catch.
func isNilPlugin(p any) bool {
	if p == nil {
		return true
	}
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Func, reflect.Interface, reflect.Chan:
		return v.IsNil()
	}
	return false
}

// validVarType reports whether t is one of the declared VarType constants;
// VarType.toC maps anything else to implicit integer, silently.
func validVarType(t VarType) bool { return t >= VarTypeContinuous && t <= VarTypeImplInt }

// invalid builds an *Error for an argument the binding rejects itself.
func (m Model) invalid(op string, rc Retcode, detail string) error {
	return &Error{Op: op, Stage: m.scip.stage(), Retcode: rc, Detail: detail}
}

// call wraps a raw SCIP retcode from a direct C call.
func (m Model) call(op string, rc C.SCIP_RETCODE) error {
	return m.wrap(op, retcodeError(rc), "")
}
