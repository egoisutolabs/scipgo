package scip

import "testing"

// scipStageTable records, for every SCIP function a stage set in stages.go
// cites, the stages that function's SCIPcheckStage call permits, taken from
// the SCIP 10.0.3 sources (scip_*.c). It is the reference the sets are
// checked against; regenerate it from the sources when SCIP is upgraded,
// not from the header @pre lists, which disagree with the implementation in
// several places.
var scipStageTable = map[string][]Stage{
	"SCIPfindCons":         {StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans},
	"SCIPgetBestNode":      {StageSolving},
	"SCIPgetChildren":      {StageSolving, StageSolved},
	"SCIPgetColRedcost":    {StageSolving, StageSolved},
	"SCIPgetConss":         {StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved},
	"SCIPgetDualbound":     {StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve},
	"SCIPgetFocusNode":     {StageInitPresolve, StagePresolving, StageExitPresolve, StageSolving},
	"SCIPgetLPObjval":      {StageSolving},
	"SCIPgetLPSolstat":     {StageSolving},
	"SCIPgetLastDivenode":  {StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans},
	"SCIPgetLeaves":        {StageSolving, StageSolved},
	"SCIPgetNChildren":     {StageSolving, StageSolved},
	"SCIPgetNConss":        {StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved},
	"SCIPgetNLPIterations": {StagePresolving, StagePresolved, StageSolving, StageSolved},
	"SCIPgetNNodes":        {StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans},
	"SCIPgetNOrigConss":    {StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans},
	"SCIPgetNOrigVars":     {StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans},
	"SCIPgetNSols":         {StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve},
	"SCIPgetNVars":         {StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve},
	"SCIPgetOrigVars":      {StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans},
	"SCIPgetPrimalbound":   {StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve},
	"SCIPgetProbingDepth":  {StagePresolving, StageSolving},
	"SCIPgetSiblings":      {StageSolving, StageSolved},
	"SCIPgetSolVal":        {StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans},
	"SCIPgetSols":          {StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve},
	"SCIPgetSolvingTime":   {StageProblem, StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved},
	"SCIPgetVarObjProbing": {StageSolving},
	"SCIPgetVarRedcost":    {StageSolving},
	"SCIPgetVarSol":        {StagePresolved, StageSolving},
	"SCIPgetVars":          {StageProblem, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve},
	"SCIPhasCurrentNodeLP": {StageSolving},
	"SCIPinDive":           {StageTransforming, StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve, StageFreeTrans},
	"SCIPinProbing":        {StageTransformed, StageInitPresolve, StagePresolving, StageExitPresolve, StagePresolved, StageInitSolve, StageSolving, StageSolved, StageExitSolve},
	"SCIPisLPConstructed":  {StageSolving},
}

// setsUnderTest maps each named set to the functions its comment cites.
var setsUnderTest = map[string]struct {
	set stageSet
	fns []string
}{
	"stagesConss":         {stagesConss, []string{"SCIPgetNConss", "SCIPgetConss"}},
	"stagesFocus":         {stagesFocus, []string{"SCIPgetFocusNode"}},
	"stagesInDive":        {stagesInDive, []string{"SCIPinDive", "SCIPgetLastDivenode"}},
	"stagesLPIter":        {stagesLPIter, []string{"SCIPgetNLPIterations"}},
	"stagesOrig":          {stagesOrig, []string{"SCIPgetNOrigVars", "SCIPgetOrigVars", "SCIPgetNOrigConss", "SCIPfindCons", "SCIPgetSolVal", "SCIPgetNNodes"}},
	"stagesProbingDepth":  {stagesProbingDepth, []string{"SCIPgetProbingDepth"}},
	"stagesSolving":       {stagesSolving, []string{"SCIPgetBestNode", "SCIPgetLPObjval", "SCIPgetLPSolstat", "SCIPisLPConstructed", "SCIPhasCurrentNodeLP", "SCIPgetVarRedcost", "SCIPgetVarObjProbing"}},
	"stagesSolvingSolved": {stagesSolvingSolved, []string{"SCIPgetChildren", "SCIPgetNChildren", "SCIPgetLeaves", "SCIPgetSiblings", "SCIPgetColRedcost"}},
	"stagesSolvingTime":   {stagesSolvingTime, []string{"SCIPgetSolvingTime"}},
	"stagesTrans":         {stagesTrans, []string{"SCIPgetNVars", "SCIPgetVars", "SCIPgetNSols", "SCIPgetSols"}},
	"stagesTransformed":   {stagesTransformed, []string{"SCIPinProbing", "SCIPgetPrimalbound", "SCIPgetDualbound"}},
	"stagesVarSol":        {stagesVarSol, []string{"SCIPgetVarSol"}},
}

func TestStageSetsMatchSCIP(t *testing.T) {
	for name, s := range setsUnderTest {
		for _, fn := range s.fns {
			want, ok := scipStageTable[fn]
			if !ok {
				t.Errorf("%s cites %s, which is not in scipStageTable", name, fn)
				continue
			}
			if stages(want...) != s.set {
				t.Errorf("%s disagrees with SCIP for %s: set %v, SCIP permits %v", name, fn, s.set, want)
			}
		}
	}
}
