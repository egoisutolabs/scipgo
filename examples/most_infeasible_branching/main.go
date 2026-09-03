package main

import (
	"fmt"
	"os"

	"github.com/egoisutolabs/scipgo/scip"
)

// mostInfeasibleBranching selects the variable with the highest
// fractionality (closest to 0.5) to branch on.
type mostInfeasibleBranching struct{}

func (mostInfeasibleBranching) Execute(model scip.Model, _ scip.SCIPBranchRule, candidates []scip.BranchingCandidate) scip.BranchingResult {
	best := candidates[0]
	bestFractionality := abs(best.Frac - 0.5)

	for _, cand := range candidates[1:] {
		if f := abs(cand.Frac - 0.5); f > bestFractionality {
			bestFractionality = f
			best = cand
		}
	}

	v, ok := model.VarInProb(best.VarProbID)
	if !ok {
		panic("branching candidate not in problem")
	}
	fmt.Printf("-- MostInfeasibleBranching: branching on variable %s with fractionality %f\n",
		v.Name(), best.Frac)
	return scip.BranchOn(best)
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func main() {
	file := "../../data/test/simple.mps"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}

	model, err := scip.NewModel().
		IncludeDefaultPlugins().
		ReadProb(file)
	if err != nil {
		fmt.Println("Failed to read problem file:", err)
		os.Exit(1)
	}
	model = model.
		SetPresolving(scip.ParamSettingOff).
		SetHeuristics(scip.ParamSettingOff).
		SetSeparating(scip.ParamSettingOff)

	model.Add(scip.NewBranchRule(mostInfeasibleBranching{}).
		Name("MostInfeasible").
		Desc("Most infeasible branching rule"))

	solved := model.Solve()

	if solved.Status() != scip.StatusOptimal {
		fmt.Println("unexpected status:", solved.Status())
		os.Exit(1)
	}
	fmt.Println("objective:", solved.ObjVal(), "nodes:", solved.NNodes())
}
