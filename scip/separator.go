package scip

/*
#include "helpers.h"
*/
import "C"

// Separator is the interface for defining custom separation routines.
type Separator interface {
	// ExecuteLP executes the separation routine on LP solutions.
	ExecuteLP(model Model, sep SCIPSeparator) SeparationResult
}

// SeparationResult is the result of a separation routine.
type SeparationResult int

// Separation results.
const (
	SeparationResultCutoff        SeparationResult = iota // The node is infeasible in the variable's bounds and can be cut off
	SeparationResultConsAdded                             // Added a constraint to the problem
	SeparationResultReducedDomain                         // Reduced the domain of a variable
	SeparationResultSeparated                             // Added a cutting plane to the LP
	SeparationResultDidNotFind                            // Searched, but found no domain reductions, cutting planes, or cut constraints
	SeparationResultDidNotRun                             // The separator was skipped
	SeparationResultDelayed                               // The separator was skipped, but should be called again
	SeparationResultNewRound                              // A new separation round should be started
)

func separationResultToC(r SeparationResult) C.SCIP_RESULT {
	switch r {
	case SeparationResultCutoff:
		return C.SCIP_CUTOFF
	case SeparationResultConsAdded:
		return C.SCIP_CONSADDED
	case SeparationResultReducedDomain:
		return C.SCIP_REDUCEDDOM
	case SeparationResultSeparated:
		return C.SCIP_SEPARATED
	case SeparationResultDidNotFind:
		return C.SCIP_DIDNOTFIND
	case SeparationResultDidNotRun:
		return C.SCIP_DIDNOTRUN
	case SeparationResultDelayed:
		return C.SCIP_DELAYED
	case SeparationResultNewRound:
		return C.SCIP_NEWROUND
	default:
		return C.SCIP_DIDNOTRUN
	}
}

// SCIPSeparator is a wrapper struct for the internal SCIP separator object.
type SCIPSeparator struct {
	raw *C.SCIP_SEPA
}

// Inner returns the internal raw pointer of the separator.
func (s SCIPSeparator) Inner() *C.SCIP_SEPA { return s.raw }

// Name returns the name of the separator.
func (s SCIPSeparator) Name() string { return goString(C.SCIPsepaGetName(s.raw)) }

// Desc returns the description of the separator.
func (s SCIPSeparator) Desc() string { return goString(C.SCIPsepaGetDesc(s.raw)) }

// Priority returns the priority of the separator.
func (s SCIPSeparator) Priority() int32 { return int32(C.SCIPsepaGetPriority(s.raw)) }

// Freq returns the frequency of the separator.
func (s SCIPSeparator) Freq() int32 { return int32(C.SCIPsepaGetFreq(s.raw)) }

// SetFreq sets the frequency of the separator.
func (s *SCIPSeparator) SetFreq(freq int32) { C.SCIPsepaSetFreq(s.raw, C.int(freq)) }

// MaxBoundDist returns the maxbounddist of the separator.
func (s SCIPSeparator) MaxBoundDist() float64 { return float64(C.SCIPsepaGetMaxbounddist(s.raw)) }

// IsDelayed returns whether the separator is delayed.
func (s SCIPSeparator) IsDelayed() bool { return C.SCIPsepaIsDelayed(s.raw) != 0 }

// CreateEmptyRow creates an empty LP row.
func (s SCIPSeparator) CreateEmptyRow(model Model, name string, lhs, rhs float64, local, modifiable, removable bool) (Row, error) {
	cn := cString(name)
	defer freeCString(cn)
	var row *C.SCIP_ROW
	if err := retcodeError(C.SCIPcreateEmptyRowSepa(model.scip.raw, &row, s.raw, cn,
		C.double(lhs), C.double(rhs), SCIPBool(local), SCIPBool(modifiable), SCIPBool(removable))); err != nil {
		return Row{}, err
	}
	return Row{raw: row, scip: model.scip}, nil
}
