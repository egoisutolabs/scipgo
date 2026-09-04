package scip

/*
#include "helpers.h"
*/
import "C"

// Separator is the interface for defining custom separation routines.
type Separator interface {
	// ExecuteLP executes the separation routine on LP solutions.
	ExecuteLP(model Model, sep SeparatorPlugin) SeparationResult
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

// SeparatorPlugin is a wrapper struct for the internal SCIP separator object.
type SeparatorPlugin struct {
	raw  *C.SCIP_SEPA
	scip *Scip // keeps the owning instance alive and identifies it
}

// live panics with *Error unless the wrapper is usable; see handleErr.
func (h SeparatorPlugin) live(op string) {
	mustLive(op, "SeparatorPlugin", h.raw != nil, h.scip, 0, true)
}

// Inner returns the internal raw pointer of the separator.
func (s SeparatorPlugin) Inner() *C.SCIP_SEPA { return s.raw }

// Name returns the name of the separator.
func (s SeparatorPlugin) Name() string {
	s.live("SeparatorPlugin.Name")
	return goString(C.SCIPsepaGetName(s.raw))
}

// Desc returns the description of the separator.
func (s SeparatorPlugin) Desc() string {
	s.live("SeparatorPlugin.Desc")
	return goString(C.SCIPsepaGetDesc(s.raw))
}

// Priority returns the priority of the separator.
func (s SeparatorPlugin) Priority() int32 {
	s.live("SeparatorPlugin.Priority")
	return int32(C.SCIPsepaGetPriority(s.raw))
}

// Freq returns the frequency of the separator.
func (s SeparatorPlugin) Freq() int32 {
	s.live("SeparatorPlugin.Freq")
	return int32(C.SCIPsepaGetFreq(s.raw))
}

// SetFreq sets the frequency of the separator.
func (s SeparatorPlugin) SetFreq(freq int32) {
	s.live("SeparatorPlugin.SetFreq")
	C.SCIPsepaSetFreq(s.raw, C.int(freq))
}

// MaxBoundDist returns the maxbounddist of the separator.
func (s SeparatorPlugin) MaxBoundDist() float64 {
	s.live("SeparatorPlugin.MaxBoundDist")
	return float64(C.SCIPsepaGetMaxbounddist(s.raw))
}

// IsDelayed returns whether the separator is delayed.
func (s SeparatorPlugin) IsDelayed() bool {
	s.live("SeparatorPlugin.IsDelayed")
	return C.SCIPsepaIsDelayed(s.raw) != 0
}

// CreateEmptyRow creates an empty LP row.
func (s SeparatorPlugin) CreateEmptyRow(model Model, name string, lhs, rhs float64, local, modifiable, removable bool) (Row, error) {
	s.live("SeparatorPlugin.CreateEmptyRow")
	return NewRow().Name(name).Bounds(lhs, rhs).Local(local).Modifiable(modifiable).Removable(removable).
		Source(SourceSepa(s)).TryAddTo(model)
}
