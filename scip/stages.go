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

	// stagesTransformed: scip_probing.c SCIPinProbing, scip_solvingstats.c
	// SCIPgetPrimalbound/SCIPgetDualbound (everything from TRANSFORMED to EXITSOLVE).
	stagesTransformed = stages(StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved,
		StageInitSolve, StageSolving, StageSolved, StageExitSolve)

	// stagesBestSol: scip_sol.c SCIPgetBestSol permits INIT too, but the
	// binding reads SCIPgetNSols first, which does not, so it takes the
	// SCIPgetNSols set.
	stagesBestSol = stagesTrans

	// stagesConss: scip_prob.c SCIPgetNConss/SCIPgetConss (no EXITSOLVE).
	stagesConss = stages(StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve,
		StagePresolved, StageInitSolve, StageSolving, StageSolved)

	// stagesSolvingTime: scip_solvingstats.c SCIPgetSolvingTime.
	stagesSolvingTime = stages(StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving,
		StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved)

	// stagesLPIter: scip_solvingstats.c SCIPgetNLPIterations.
	stagesLPIter = stages(StagePresolving, StagePresolved, StageSolving, StageSolved)

	// stagesFocus: scip_tree.c SCIPgetFocusNode.
	stagesFocus = stages(StageInitPresolve, StagePresolving, StageExitPresolve, StageSolving)

	// stagesSolvingSolved: scip_tree.c SCIPgetChildren/SCIPgetNChildren/
	// SCIPgetLeaves/SCIPgetSiblings, scip_lp.c SCIPgetColRedcost.
	stagesSolvingSolved = stages(StageSolving, StageSolved)

	// stagesSolving: scip_tree.c SCIPgetBestNode and friends, scip_lp.c
	// SCIPgetLPObjval/SCIPgetLPSolstat/SCIPisLPConstructed/SCIPhasCurrentNodeLP,
	// scip_var.c SCIPgetVarRedcost, scip_probing.c SCIPgetVarObjProbing, the
	// dive getters.
	stagesSolving = stages(StageSolving)

	// stagesVarSol: scip_var.c SCIPgetVarSol; the solution-value getter takes
	// the same set when given a NULL solution (the current LP/pseudo solution).
	stagesVarSol = stages(StagePresolved, StageSolving)

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

// mustStage panics with *Error when a handle method's SCIP getter is not
// permitted in the current stage (it would abort otherwise).
func mustStage(op string, owner *Scip, allowed stageSet) {
	if st := owner.stage(); !allowed.has(st) {
		panic(&Error{Op: op, Stage: st, Retcode: RetcodeInvalidCall, Detail: "not permitted in this stage"})
	}
}
