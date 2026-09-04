package scip

/*
#include "helpers.h"
*/
import "C"

import "fmt"

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
	if s == nil || s.raw == nil {
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
	if m.scip == nil || m.scip.raw == nil {
		return &Error{Op: op, Stage: StageFree, Retcode: RetcodeInvalidCall, Detail: "model is freed or zero"}
	}
	return nil
}

// wrap converts an error from the internal layer into *Error for op. Retcode
// values keep their code; other errors (argument validation, expression
// building) become RetcodeInvalidData with the message as detail.
func (m Model) wrap(op string, err error, detail string) error {
	switch e := err.(type) {
	case nil:
		return nil
	case *Error:
		return e
	case *CallbackPanic:
		return e
	case Retcode:
		return &Error{Op: op, Stage: m.scip.stage(), Retcode: e, Detail: detail}
	default:
		if detail != "" {
			detail += ": "
		}
		return &Error{Op: op, Stage: m.scip.stage(), Retcode: RetcodeInvalidData, Detail: detail + e.Error()}
	}
}

// invalid builds an *Error for an argument the binding rejects itself.
func (m Model) invalid(op string, rc Retcode, detail string) error {
	return &Error{Op: op, Stage: m.scip.stage(), Retcode: rc, Detail: detail}
}

// call wraps a raw SCIP retcode from a direct C call.
func (m Model) call(op string, rc C.SCIP_RETCODE) error {
	return m.wrap(op, retcodeError(rc), "")
}
