package scip

/*
#include "helpers.h"
*/
import "C"

// Heur is a primal heuristic that is part of the model, giving access to its
// runtime statistics (how often it ran and how many solutions it found).
// Obtain one via Model.FindHeur.
type Heur struct {
	raw  *C.SCIP_HEUR
	scip *Scip
}

// Inner returns a pointer to the underlying SCIP_HEUR.
func (h Heur) Inner() *C.SCIP_HEUR { return h.raw }

// Name returns the name of the heuristic.
func (h Heur) Name() string { return goString(C.SCIPheurGetName(h.raw)) }

// Desc returns the description of the heuristic.
func (h Heur) Desc() string { return goString(C.SCIPheurGetDesc(h.raw)) }

// Priority returns the priority of the heuristic.
func (h Heur) Priority() int32 { return int32(C.SCIPheurGetPriority(h.raw)) }

// Freq returns the calling frequency of the heuristic; -1 means disabled.
func (h Heur) Freq() int32 { return int32(C.SCIPheurGetFreq(h.raw)) }

// SetFreq sets the calling frequency of the heuristic; -1 disables it.
func (h Heur) SetFreq(freq int32) { C.SCIPheurSetFreq(h.raw, C.int(freq)) }

// NCalls returns the number of times the heuristic was called during the
// solving process.
func (h Heur) NCalls() int { return int(C.SCIPheurGetNCalls(h.raw)) }

// NSolsFound returns the number of solutions the heuristic found during the
// solving process.
func (h Heur) NSolsFound() int { return int(C.SCIPheurGetNSolsFound(h.raw)) }

// NBestSolsFound returns the number of new best (incumbent) solutions the
// heuristic found during the solving process.
func (h Heur) NBestSolsFound() int { return int(C.SCIPheurGetNBestSolsFound(h.raw)) }

// Heuristic is the interface for defining custom primal heuristics.
// Implementations should use a pointer receiver when they need mutation.
type Heuristic interface {
	// Execute executes the heuristic.
	//
	// timing is the timing mask of the heuristic's execution and nodeInf
	// indicates whether the current node is infeasible.
	Execute(model Model, timing HeurTiming, nodeInf bool) HeurResult
}

// HeurResult is the result of a primal heuristic execution.
type HeurResult int

// Heuristic results.
const (
	HeurResultFoundSol   HeurResult = iota // The heuristic found a new incumbent solution
	HeurResultNoSolFound                   // The heuristic did not find a new solution
	HeurResultDidNotRun                    // The heuristic was not executed
	HeurResultDelayed                      // The heuristic is delayed (skipped but should be called again)
)

// String implements fmt.Stringer.
func (r HeurResult) String() string {
	switch r {
	case HeurResultFoundSol:
		return "FoundSol"
	case HeurResultNoSolFound:
		return "NoSolFound"
	case HeurResultDidNotRun:
		return "DidNotRun"
	case HeurResultDelayed:
		return "Delayed"
	default:
		return "Unknown"
	}
}

func heurResultToC(r HeurResult) C.SCIP_RESULT {
	switch r {
	case HeurResultFoundSol:
		return C.SCIP_FOUNDSOL
	case HeurResultNoSolFound:
		return C.SCIP_DIDNOTFIND
	case HeurResultDidNotRun:
		return C.SCIP_DIDNOTRUN
	case HeurResultDelayed:
		return C.SCIP_DELAYED
	default:
		return C.SCIP_DIDNOTRUN
	}
}

// HeurTiming represents timing masks for the execution of a heuristic.
// Values can be combined with the | operator.
type HeurTiming uint32

// Heuristic timing masks.
const (
	HeurTimingBeforeNode        HeurTiming = 0x001 // call heuristic before the processing of the node starts
	HeurTimingDuringLpLoop      HeurTiming = 0x002 // call heuristic after each LP solving during cut-and-price loop
	HeurTimingAfterLpLoop       HeurTiming = 0x004 // call heuristic after the cut-and-price loop was finished
	HeurTimingAfterLpNode       HeurTiming = 0x008 // call heuristic after the processing of a node with solved LP was finished
	HeurTimingAfterPseudoNode   HeurTiming = 0x010 // call heuristic after the processing of a node without solved LP was finished
	HeurTimingAfterLpPlunge     HeurTiming = 0x020 // call heuristic after the processing of the last node in the current plunge was finished (with solved LP)
	HeurTimingAfterPseudoPlunge HeurTiming = 0x040 // call heuristic after the processing of the last node in the current plunge was finished (without solved LP)
	HeurTimingDuringPricingLoop HeurTiming = 0x080 // call heuristic during pricing loop
	HeurTimingBeforePresol      HeurTiming = 0x100 // call heuristic before presolving
	HeurTimingDuringPresolLoop  HeurTiming = 0x200 // call heuristic during presolving loop
	HeurTimingAfterPropLoop     HeurTiming = 0x400 // call heuristic after propagation which is performed before solving the LP
)

func heurTimingFromC(t uint32) HeurTiming { return HeurTiming(t) }

func (t HeurTiming) toC() C.uint { return C.uint(t) }
