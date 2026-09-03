// A primal heuristic that performs random rounding at LP solutions.
package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"

	"github.com/egoisutolabs/scipgo/scip"
)

type randomRoundingHeur struct {
	rng *rand.Rand
}

func (h *randomRoundingHeur) Execute(model scip.Model, _ scip.HeurTiming, nodeInf bool) scip.HeurResult {
	// Skip if the node is infeasible
	if nodeInf {
		return scip.HeurResultDidNotRun
	}

	rng := rand.New(rand.NewSource(1))

	// Create a new solution
	sol := model.CreateSol()
	vars := model.Vars()

	// Get current LP solution values; randomly round fractional integer
	// variables (up with probability equal to the fractional part).
	hasFractional := false
	for _, v := range vars {
		lpVal := model.CurrentVal(v)
		t := v.VarType()

		if t == scip.VarTypeInteger || t == scip.VarTypeBinary {
			fracPart := lpVal - math.Trunc(lpVal)
			if fracPart > 1e-6 && fracPart < 1.0-1e-6 {
				hasFractional = true
				var rounded float64
				if rng.Float64() < fracPart {
					rounded = math.Ceil(lpVal)
				} else {
					rounded = math.Floor(lpVal)
				}
				sol.SetVal(v, rounded)
			} else {
				sol.SetVal(v, math.Round(lpVal))
			}
		} else {
			// Keep continuous variables as they are
			sol.SetVal(v, lpVal)
		}
	}

	// Only try to add the solution if we actually rounded something
	if !hasFractional {
		return scip.HeurResultDidNotRun
	}

	fmt.Print("-- RandomRoundingHeur: found a solution: ")
	for _, v := range vars {
		fmt.Printf("%s = %v, ", v.Name(), sol.Val(v))
	}
	fmt.Println()

	solVal := sol.ObjVal()

	// Try to add the rounded solution
	if err := model.AddSol(&sol); err != nil {
		fmt.Println("-- RandomRoundingHeur: Failed to add solution to the model.")
		return scip.HeurResultNoSolFound
	}
	fmt.Printf("-- RandomRoundingHeur: Added solution to the model with val %v.\n", solVal)
	return scip.HeurResultFoundSol
}

func main() {
	model, err := scip.NewModel().
		IncludeDefaultPlugins().
		ReadProb("../../data/test/simple.mps")
	if err != nil {
		fmt.Println("failed to read problem:", err)
		os.Exit(1)
	}
	model = model.
		SetPresolving(scip.ParamSettingOff).
		SetHeuristics(scip.ParamSettingOff).
		SetSeparating(scip.ParamSettingOff)

	// Add our random rounding heuristic
	model.Add(scip.NewHeur(&randomRoundingHeur{}).
		Name("random_round").
		Desc("Random rounding at LP solutions").
		Priority(1000).
		Freq(1).
		Timing(scip.HeurTimingDuringLpLoop))

	solved := model.Solve()

	if solved.NSols() < 2 {
		fmt.Println("expected at least 2 solutions: the heuristic's and the optimal one")
		os.Exit(1)
	}
	fmt.Println("solutions found:", solved.NSols())
}
