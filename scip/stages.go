package scip

// stageSet is a bitmask over Stage. SCIP's SCIPget* accessors validate their
// stage with SCIP_CALL_ABORT, which in a release build prints an error and
// continues into undefined behaviour (stale reads, NULL dereferences) rather
// than returning a retcode, so every query in the binding checks the stage
// first. The sets below are taken from the SCIPcheckStage calls in the SCIP
// sources named in each comment (the header @pre lists disagree in places);
// rebuild them from the sources, not from memory.
type stageSet uint16

func stages(ss ...Stage) stageSet {
	var set stageSet
	for _, s := range ss {
		set |= 1 << uint(s)
	}
	return set
}

func (s stageSet) has(st Stage) bool { return s&(1<<uint(st)) != 0 }

var (
	// stagesAny: no stage restriction; only a freed instance is rejected.
	stagesAny = stages(StageInit, StageProblem, StageTransforming, StageTransformed, StageInitPresolve,
		StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved,
		StageExitSolve, StageFreeTrans)

	// stagesOrig: scip_prob.c SCIPgetNOrigVars/SCIPgetOrigVars/SCIPgetNOrigConss/
	// SCIPfindCons, scip_sol.c SCIPgetSolVal, scip_solvingstats.c SCIPgetNNodes.
	// The objective getter used by Solution.ObjVal permits this set for
	// original solutions and drops PROBLEM for transformed ones, which cannot
	// exist in PROBLEM anyway.
	stagesOrig = stages(StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving,
		StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans)

	// stagesTrans: scip_prob.c SCIPgetNVars/SCIPgetVars, scip_sol.c
	// SCIPgetNSols/SCIPgetSols.
	stagesTrans = stages(StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve,
		StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve)

	// stagesInProbing: scip_probing.c SCIPinProbing (no PROBLEM stage).
	stagesInProbing = stages(StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved,
		StageInitSolve, StageSolving, StageSolved, StageExitSolve)

	// stagesBestSol: scip_sol.c SCIPgetBestSol (also legal before a problem exists).
	stagesBestSol = stages(StageInit, StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve,
		StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve)

	// stagesConss: scip_prob.c SCIPgetNConss/SCIPgetConss (no EXITSOLVE).
	stagesConss = stages(StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve,
		StagePresolved, StageInitSolve, StageSolving, StageSolved)

	// stagesBounds: scip_solvingstats.c SCIPgetPrimalbound/SCIPgetDualbound.
	stagesBounds = stages(StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved,
		StageInitSolve, StageSolving, StageSolved, StageExitSolve)

	// stagesSolvingTime: scip_solvingstats.c SCIPgetSolvingTime.
	stagesSolvingTime = stages(StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving,
		StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved)

	// stagesLPIter: scip_solvingstats.c SCIPgetNLPIterations.
	stagesLPIter = stages(StagePresolving, StagePresolved, StageSolving, StageSolved)

	// stagesFocus: scip_tree.c SCIPgetFocusNode.
	stagesFocus = stages(StageInitPresolve, StagePresolving, StageExitPresolve, StageSolving)

	// stagesChildren: scip_tree.c SCIPgetChildren/SCIPgetNChildren.
	stagesChildren = stages(StageSolving, StageSolved)

	// stagesLeaves: scip_tree.c SCIPgetLeaves/SCIPgetSiblings.
	stagesLeaves = stages(StageSolving, StageSolved)

	// stagesSolving: scip_tree.c SCIPgetBestNode and friends, scip_lp.c
	// SCIPgetLPObjval/SCIPgetLPSolstat/SCIPisLPConstructed, scip_var.c
	// SCIPgetVarRedcost, scip_probing.c SCIPgetVarObjProbing, the dive getters.
	stagesSolving = stages(StageSolving)

	// stagesVarSol: scip_var.c SCIPgetVarSol.
	stagesVarSol = stages(StagePresolved, StageSolving)

	// stagesColRedcost: scip_lp.c SCIPgetColRedcost.
	stagesColRedcost = stages(StageSolving, StageSolved)

	// stagesProbingDepth: scip_probing.c SCIPgetProbingDepth.
	stagesProbingDepth = stages(StagePresolving, StageSolving)

	// stagesInDive: scip_lp.c SCIPinDive/SCIPgetLastDivenode.
	stagesInDive = stages(StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve,
		StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans)
)

// query rejects a query that would make SCIP abort (wrong stage) or
// dereference a freed instance, returning *Error instead.
func (m Model) query(op string, allowed stageSet) error {
	if err := m.guard(op); err != nil {
		return err
	}
	if st := m.scip.stage(); !allowed.has(st) {
		return &Error{Op: op, Stage: st, Retcode: RetcodeInvalidCall, Detail: "not permitted in this stage"}
	}
	return nil
}

// mustLive panics with *Error when a handle is the zero value or its model
// has been freed, instead of letting SCIP dereference a nil or dangling
// pointer. It is the first statement of every handle method.
func mustLive(op, what string, raw bool, owner *Scip) {
	switch {
	case !raw:
		panic(&Error{Op: op, Stage: owner.stage(), Retcode: RetcodeInvalidData, Detail: "zero " + what})
	case owner == nil || owner.raw == nil:
		panic(&Error{Op: op, Stage: StageFree, Retcode: RetcodeInvalidCall, Detail: what + " belongs to a freed model"})
	}
}

// mustStage panics with *Error when a handle method's SCIP getter is not
// permitted in the current stage (it would abort otherwise).
func mustStage(op string, owner *Scip, allowed stageSet) {
	if st := owner.stage(); !allowed.has(st) {
		panic(&Error{Op: op, Stage: st, Retcode: RetcodeInvalidCall, Detail: "not permitted in this stage"})
	}
}
