package scip

/*
#include "helpers.h"
*/
import "C"

// BranchRule is the interface for defining custom branching rules.
type BranchRule interface {
	// Execute runs the branching rule on the given candidates and returns the result.
	Execute(model Model, branchrule BranchRulePlugin, candidates []BranchingCandidate) BranchingResult
}

// BranchingResultKind describes the outcome of a branching rule execution.
type BranchingResultKind int

// Branching results.
const (
	BranchingResultDidNotRun       BranchingResultKind = iota // The branching rule did not run
	BranchingResultBranchOn                                   // Initiate branching on the candidate
	BranchingResultCutOff                                     // Current node is infeasible and can be cut off
	BranchingResultCustomBranching                            // A custom branching scheme is implemented
	BranchingResultSeparated                                  // A cutting plane is added
	BranchingResultReduceDom                                  // A variable domain was reduced
	BranchingResultConsAdded                                  // A constraint was added
)

// BranchingResult is the result of a branching rule execution. When Kind is
// BranchingResultBranchOn, Candidate holds the branching candidate to use.
type BranchingResult struct {
	Kind      BranchingResultKind
	Candidate BranchingCandidate
}

// BranchOn constructs a BranchingResult that branches on the given candidate.
func BranchOn(c BranchingCandidate) BranchingResult {
	return BranchingResult{Kind: BranchingResultBranchOn, Candidate: c}
}

func branchResultToC(r BranchingResult) C.SCIP_RESULT {
	switch r.Kind {
	case BranchingResultDidNotRun:
		return C.SCIP_DIDNOTRUN
	case BranchingResultBranchOn:
		return C.SCIP_BRANCHED
	case BranchingResultCutOff:
		return C.SCIP_CUTOFF
	case BranchingResultCustomBranching:
		return C.SCIP_BRANCHED
	case BranchingResultSeparated:
		return C.SCIP_SEPARATED
	case BranchingResultReduceDom:
		return C.SCIP_REDUCEDDOM
	case BranchingResultConsAdded:
		return C.SCIP_CONSADDED
	default:
		return C.SCIP_DIDNOTRUN
	}
}

// BranchingCandidate is a candidate for branching.
type BranchingCandidate struct {
	// VarProbID is the index of the variable in the current subproblem.
	VarProbID int
	// LpSolVal is the LP solution value of the variable.
	LpSolVal float64
	// Frac is the fractional part of the LP solution value.
	Frac float64
}

// BranchRulePlugin is a wrapper for the internal SCIP branch rule object.
type BranchRulePlugin struct {
	raw  *C.SCIP_BRANCHRULE
	scip *Scip // keeps the owning instance alive and identifies it
}

// live panics with *Error unless the wrapper is usable; see handleErr.
func (h BranchRulePlugin) live(op string) {
	mustLive(op, "BranchRulePlugin", h.raw != nil, h.scip, 0, true)
}

// Inner returns the internal raw pointer of the branch rule.
func (b BranchRulePlugin) Inner() *C.SCIP_BRANCHRULE { return b.raw }

// Name returns the name of the branch rule.
func (b BranchRulePlugin) Name() string {
	b.live("BranchRulePlugin.Name")
	return goString(C.SCIPbranchruleGetName(b.raw))
}

// Desc returns the description of the branch rule.
func (b BranchRulePlugin) Desc() string {
	b.live("BranchRulePlugin.Desc")
	return goString(C.SCIPbranchruleGetDesc(b.raw))
}

// Priority returns the priority of the branch rule.
func (b BranchRulePlugin) Priority() int32 {
	b.live("BranchRulePlugin.Priority")
	return int32(C.SCIPbranchruleGetPriority(b.raw))
}

// MaxDepth returns the maxdepth of the branch rule.
func (b BranchRulePlugin) MaxDepth() int32 {
	b.live("BranchRulePlugin.MaxDepth")
	return int32(C.SCIPbranchruleGetMaxdepth(b.raw))
}

// MaxBoundDist returns the maxbounddist of the branch rule.
func (b BranchRulePlugin) MaxBoundDist() float64 {
	b.live("BranchRulePlugin.MaxBoundDist")
	return float64(C.SCIPbranchruleGetMaxbounddist(b.raw))
}
