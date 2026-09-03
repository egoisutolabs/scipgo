package main

import (
	"fmt"

	"github.com/egoisutolabs/scipgo/scip"
)

func main() {
	// 0/1 knapsack: maximize value subject to a weight capacity.
	values := []float64{10, 13, 7, 4, 9, 12}
	weights := []float64{5, 7, 4, 3, 5, 6}
	capacity := 15.0

	model := scip.DefaultModel().HideOutput().Maximize()
	var items []scip.Variable
	for i, v := range values {
		items = append(items, scip.NewVar().
			Name(fmt.Sprintf("x%d", i)).
			Bin().
			Obj(v).
			AddTo(model))
	}
	model.AddCons(items, weights, scip.NegInfinity, capacity, "capacity")

	solved := model.Solve()
	fmt.Println("status:", solved.Status())
	fmt.Println("best value:", solved.ObjVal())

	if sol, ok := solved.BestSol(); ok {
		for i, item := range items {
			if sol.Val(item) > 0.5 {
				fmt.Printf("take item %d (value %.0f, weight %.0f)\n", i, values[i], weights[i])
			}
		}
	}
}
