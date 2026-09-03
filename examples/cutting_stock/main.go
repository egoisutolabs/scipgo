// Cutting stock problem solved with branch and price, following
// https://scipbook.readthedocs.io/en/latest/bpp.html
//
// The master problem selects cutting patterns; the pricer solves a knapsack
// subproblem on the demand constraints' dual values to generate new
// patterns.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/egoisutolabs/scipgo/scip"
)

type cspPricer struct {
	stockLength float64
	itemSizes   []float64
}

func (p *cspPricer) GenerateColumns(model scip.Model, _ scip.PricerPlugin, farkas bool) scip.PricerResult {
	// Pricing has no idea what branching decisions were made by SCIP, so we
	// only run the pricer at the root node.
	if model.FocusNode().Depth() > 0 {
		return scip.PricerResult{State: scip.PricerResultStateNoColumns}
	}

	if farkas {
		panic("unexpected infeasibility; root node should be feasible by construction")
	}

	pricingModel := scip.DefaultModel().HideOutput().Maximize()

	vars := make([]scip.Variable, 0, len(p.itemSizes))
	for i := range p.itemSizes {
		cons, ok := model.FindCons(fmt.Sprintf("demand_for_item_%d", i))
		if !ok {
			panic("demand constraint not found")
		}
		dualVal, ok := cons.DualSol()
		if !ok {
			panic("no dual value found for linear constraint")
		}
		vars = append(vars, scip.NewVar().
			Int().
			Name(fmt.Sprintf("demand_for_item_%d", i)).
			Obj(dualVal).
			AddTo(pricingModel))
	}

	// sum_i w_i * z_i <= W
	pairs := make([]scip.CoefPair, len(vars))
	for i := range vars {
		pairs[i] = scip.CoefPair{Var: vars[i], Coef: p.itemSizes[i]}
	}
	pricingModel.Add(scip.NewCons().
		Name("is_valid_pattern_constraint").
		Expr(pairs...).
		Le(p.stockLength))

	solved := pricingModel.Solve()

	sol, ok := solved.BestSol()
	if !ok {
		return scip.PricerResult{State: scip.PricerResultStateNoColumns}
	}
	reducedCost := 1.0 - sol.ObjVal()
	if reducedCost >= -1e-6 {
		fmt.Printf("    Didn't find column (obj_value = %v, reduced_cost = %v)\n",
			model.ObjVal(), reducedCost)
		return scip.PricerResult{State: scip.PricerResultStateNoColumns}
	}

	pattern := make([]string, len(vars))
	vals := make([]float64, len(vars))
	for i, v := range vars {
		vals[i] = sol.Val(v)
		pattern[i] = fmt.Sprintf("%d", int(sol.Val(v)))
	}

	// add variable for new cutting pattern
	newVarName := "pattern_" + join(pattern, "-")
	for _, v := range model.Vars() {
		if v.Name() == newVarName {
			// avoid adding the same pattern twice
			return scip.PricerResult{State: scip.PricerResultStateNoColumns}
		}
	}
	fmt.Println("    Adding " + newVarName)
	newVar := model.AddPricedVar(0, math.Inf(1), 1, newVarName, scip.VarTypeInteger)
	for i := range p.itemSizes {
		cons, _ := model.FindCons(fmt.Sprintf("demand_for_item_%d", i))
		model.AddConsCoef(cons, newVar, vals[i])
	}
	return scip.PricerResult{State: scip.PricerResultStateFoundColumns}
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func main() {
	stockLength := 9.0
	itemSizes := []float64{6, 5, 4, 2, 3, 7, 5, 8, 4, 5}
	demand := []float64{2, 3, 4, 4, 2, 2, 2, 2, 2, 1}

	// Vector of cutting patterns, initially populated with the trivial ones
	// that contain exactly one item.
	mainProblem := scip.DefaultModel().
		SetPresolving(scip.ParamSettingOff).
		Minimize()

	cuttingPatternVars := make([]scip.Variable, len(itemSizes))
	for i := range itemSizes {
		pattern := make([]string, len(itemSizes))
		for j := range itemSizes {
			if j == i {
				pattern[j] = "1"
			} else {
				pattern[j] = "0"
			}
		}
		cuttingPatternVars[i] = scip.NewVar().
			Int().
			Obj(1).
			Name("pattern_" + join(pattern, "-")).
			AddTo(mainProblem)
	}

	for i, count := range demand {
		demandCons := mainProblem.AddCons(
			[]scip.Variable{cuttingPatternVars[i]}, []float64{1}, count, math.Inf(1),
			fmt.Sprintf("demand_for_item_%d", i))
		mainProblem.SetConsModifiable(demandCons, true)
	}

	mainProblem.Add(scip.NewPricer(&cspPricer{stockLength: stockLength, itemSizes: itemSizes}).
		Name("CSPPricer"))

	solved := mainProblem.Solve()

	fmt.Println("\nSolution")
	sol, _ := solved.BestSol()
	for _, v := range solved.Vars() {
		if value := sol.Val(v); value != 0 {
			fmt.Printf("  %s=%v\n", v.Name(), value)
		}
	}

	if r := math.Round(sol.ObjVal()); r != 13 {
		fmt.Println("expected 13 rolls, got", r)
		os.Exit(1)
	}
}
