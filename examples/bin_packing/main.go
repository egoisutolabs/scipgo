// Bin packing solved with branch (Ryan-Foster) and price (knapsack pricer).
// Variable patterns are priced with a knapsack subproblem over the item
// constraints' duals; branching decisions are tracked per node in the model
// datastore and enforced in the pricer.
package main

import (
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/egoisutolabs/scipgo/scip"
)

type pair struct {
	a, b int
}

type branchingDecisions struct {
	together map[pair]bool
	apart    map[pair]bool
}

// sortedPairs returns the keys of a pair-keyed map in ascending order.
func sortedPairs[V any](m map[pair]V) []pair {
	pairs := make([]pair, 0, len(m))
	for p := range m {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(x, y int) bool {
		if pairs[x].a != pairs[y].a {
			return pairs[x].a < pairs[y].a
		}
		return pairs[x].b < pairs[y].b
	})
	return pairs
}

func newBranchingDecisions() *branchingDecisions {
	return &branchingDecisions{
		together: make(map[pair]bool),
		apart:    make(map[pair]bool),
	}
}

// patternForVar maps var index -> items in the pattern.
type patternForVar struct {
	m map[int][]int
}

// itemToConstraint holds one (modifiable) equality constraint per item.
type itemToConstraint struct {
	cons []scip.Constraint
}

type binPackingInstance struct {
	itemSizes []float64
	capacity  float64
}

// branchingDecisionMap maps BB node number -> decisions active there.
type branchingDecisionMap struct {
	m map[int]*branchingDecisions
}

type knapsackPricer struct{}

func getDuals(itemConstraints []scip.Constraint, farkas bool) []float64 {
	duals := make([]float64, len(itemConstraints))
	for i, cons := range itemConstraints {
		c, ok := cons.Transformed()
		if !ok {
			panic("could not get transformed constraint")
		}
		var d float64
		if farkas {
			d, ok = c.FarkasDualSol()
		} else {
			d, ok = c.DualSol()
		}
		if !ok {
			panic("could not get dual solution")
		}
		duals[i] = d
	}
	return duals
}

func (knapsackPricer) GenerateColumns(model scip.Model, _ scip.PricerPlugin, farkas bool) scip.PricerResult {
	items := scip.MustGetData[*itemToConstraint](model).cons
	instance := scip.MustGetData[*binPackingInstance](model)

	duals := getDuals(items, farkas)

	currentBBNode := model.FocusNode().Number()
	decisions := scip.MustGetData[*branchingDecisionMap](model).m[currentBBNode]
	if decisions == nil {
		panic(fmt.Sprintf("no branching decisions for node %d", currentBBNode))
	}

	solItems, solValue, ok := solveKnapsack(instance.itemSizes, duals, instance.capacity, decisions)
	if !ok {
		return scip.PricerResult{State: scip.PricerResultStateNoColumns}
	}

	objCoef := 1.0
	if farkas {
		objCoef = 0.0
	}
	redcost := objCoef - solValue

	if redcost < -model.Eps() {
		name := fmt.Sprintf("%v", solItems)
		newVar := model.AddPricedVar(0, math.Inf(1), 1, name, scip.VarTypeInteger)

		for _, item := range solItems {
			model.AddConsCoef(items[item], newVar, 1)
		}

		scip.MustGetData[*patternForVar](model).m[newVar.Index()] = solItems

		return scip.PricerResult{State: scip.PricerResultStateFoundColumns}
	}
	return scip.PricerResult{State: scip.PricerResultStateNoColumns}
}

// solveKnapsack solves the knapsack subproblem in a nested SCIP model and
// returns the selected items and total profit.
func solveKnapsack(sizes, profits []float64, capacity float64, decisions *branchingDecisions) ([]int, float64, bool) {
	model := scip.DefaultModel().HideOutput().Maximize()

	vars := make([]scip.Variable, 0, len(sizes))
	for _, profit := range profits {
		vars = append(vars, scip.NewVar().Bin().Obj(profit).AddTo(model))
	}

	capacityCons := scip.NewCons().Le(capacity).Coefs(vars, sizes)
	model.Add(capacityCons)

	// add branching decisions in sorted order so runs are reproducible
	// (the Rust original iterates HashSets here and is not)
	for _, p := range sortedPairs(decisions.together) {
		model.Add(scip.NewCons().Eq(0).Coef(vars[p.a], 1).Coef(vars[p.b], -1))
	}
	for _, p := range sortedPairs(decisions.apart) {
		model.Add(scip.NewCons().Le(1).Coef(vars[p.a], 1).Coef(vars[p.b], 1))
	}

	solved := model.Solve()

	sol, ok := solved.BestSol()
	if !ok {
		return nil, 0, false
	}
	var items []int
	for i, v := range vars {
		if sol.Val(v) > 0.5 {
			items = append(items, i)
		}
	}

	total := 0.0
	for _, i := range items {
		total += sizes[i]
	}
	if total > capacity {
		panic("knapsack solution exceeds capacity")
	}
	return items, sol.ObjVal(), true
}

// ryanFoster implements Ryan-Foster branching on fractional pairs of items.
type ryanFoster struct{}

func (ryanFoster) Execute(model scip.Model, _ scip.BranchRulePlugin, candidates []scip.BranchingCandidate) scip.BranchingResult {
	patterns := scip.MustGetData[*patternForVar](model).m
	fractionalPair := findFractionalPair(model, patterns, candidates)

	currentBBNode := model.FocusNode().Number()
	currentDecisions := scip.MustGetData[*branchingDecisionMap](model).m[currentBBNode]

	// save branching decisions (for the pricer)
	downChild := model.CreateChild()
	upChild := model.CreateChild()

	downDecisions := newBranchingDecisions()
	for k := range currentDecisions.together {
		downDecisions.together[k] = true
	}
	for k := range currentDecisions.apart {
		downDecisions.apart[k] = true
	}
	downDecisions.apart[fractionalPair] = true

	upDecisions := newBranchingDecisions()
	for k := range currentDecisions.together {
		upDecisions.together[k] = true
	}
	for k := range currentDecisions.apart {
		upDecisions.apart[k] = true
	}
	upDecisions.together[fractionalPair] = true

	decisionMap := scip.MustGetData[*branchingDecisionMap](model)
	decisionMap.m[downChild.Number()] = downDecisions
	decisionMap.m[upChild.Number()] = upDecisions

	// fix infeasible variables
	i, j := fractionalPair.a, fractionalPair.b
	for _, v := range model.Vars() {
		// skip fixed vars
		if v.UbLocal() < model.Eps() {
			continue
		}

		pattern := patterns[v.Index()]
		inI, inJ := false, false
		for _, item := range pattern {
			if item == i {
				inI = true
			}
			if item == j {
				inJ = true
			}
		}

		// down child: fix any variable that uses both nodes of the pair
		if inI && inJ {
			model.SetUbNode(&downChild, v, 0)
		}

		// up child: fix any variable that uses exactly one node of the pair
		if inI != inJ {
			model.SetUbNode(&upChild, v, 0)
		}
	}

	return scip.BranchingResult{Kind: scip.BranchingResultCustomBranching}
}

func findFractionalPair(model scip.Model, patterns map[int][]int, candidates []scip.BranchingCandidate) pair {
	pairVals := make(map[pair]float64)
	for _, cand := range candidates {
		v, ok := model.VarInProb(cand.VarProbID)
		if !ok {
			panic("candidate not in problem")
		}
		pattern := patterns[v.Index()]

		for i := 0; i < len(pattern)-1; i++ {
			for j := i + 1; j < len(pattern); j++ {
				itemI, itemJ := pattern[i], pattern[j]
				if itemI != itemJ {
					p := pair{itemI, itemJ}
					pairVals[p] += cand.LpSolVal
				}
			}
		}
	}

	// find the pair with the largest fractional value, deterministically:
	// iterate pairs in ascending order and let ties pick the last one, like
	// the Rust BTreeMap + max_by original.
	bestVal := math.Inf(-1)
	var best pair
	found := false
	for _, p := range sortedPairs(pairVals) {
		val := pairVals[p]
		frac := val - math.Trunc(val)
		if frac > model.Eps() && val < 1.0-model.Eps() && val >= bestVal {
			bestVal = val
			best = p
			found = true
		}
	}
	if !found {
		panic("no fractional pair found")
	}
	return best
}

func main() {
	capacity := 15.0
	itemSizes := []float64{6, 5, 4, 2, 3, 7, 5, 8, 4, 5}

	model := scip.DefaultModel().
		SetPresolving(scip.ParamSettingOff).
		SetSeparating(scip.ParamSettingOff).
		// every column costs exactly 1 bin, so the objective is integral and
		// SCIP can stop once the dual bound passes 3 (the Rust original omits this
		// and branches for minutes)
		SetObjIntegral().
		Minimize()
	model, err := scip.SetParam(model, "display/freq", int32(1000)) // one log line per 1000 nodes; the Rust original prints every node
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	scip.SetData(model, &patternForVar{m: make(map[int][]int)})
	itemCons := &itemToConstraint{}
	scip.SetData(model, itemCons)
	scip.SetData(model, &binPackingInstance{itemSizes: itemSizes, capacity: capacity})
	scip.SetData(model, &branchingDecisionMap{m: map[int]*branchingDecisions{1: newBranchingDecisions()}})

	for range itemSizes {
		cons := scip.NewCons().Eq(1).Modifiable(true).Removable(false).AddTo(model)
		itemCons.cons = append(itemCons.cons, cons)
	}

	// attach pricer and branching rule plugins
	model.Add(scip.NewPricer(knapsackPricer{}))
	model.Add(scip.NewBranchRule(ryanFoster{}))

	solved := model.Solve()

	fmt.Println("\nSolution:")
	sol, _ := solved.BestSol()
	patterns := scip.MustGetData[*patternForVar](solved).m
	for _, v := range solved.Vars() {
		if value := sol.Val(v); value > 1e-6 {
			fmt.Printf("%v = %v\n", patterns[v.Index()], value)
		}
	}

	if !solved.Eq(sol.ObjVal(), 4.0) {
		fmt.Println("expected 4 bins, got", sol.ObjVal())
		os.Exit(1)
	}
}
