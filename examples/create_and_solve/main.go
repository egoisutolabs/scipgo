package main

import (
	"fmt"

	"github.com/egoisutolabs/scipgo/scip"
)

func main() {
	// maximize 3x1 + 4x2 subject to 2x1 + x2 <= 100, x1 + 2x2 <= 80
	model := scip.NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, scip.Infinity, 3, "x1", scip.VarTypeInteger)
	x2 := model.AddVar(0, scip.Infinity, 4, "x2", scip.VarTypeInteger)
	model.AddCons([]scip.Variable{x1, x2}, []float64{2, 1}, scip.NegInfinity, 100, "c1")
	model.AddCons([]scip.Variable{x1, x2}, []float64{1, 2}, scip.NegInfinity, 80, "c2")

	solved := model.Solve()
	fmt.Println("status:", solved.Status())
	fmt.Println("objective value:", solved.ObjVal())
	fmt.Println("nodes:", solved.NNodes(), "lp iterations:", solved.NLpIterations())

	if sol, ok := solved.BestSol(); ok {
		fmt.Println("x1 =", sol.Val(x1))
		fmt.Println("x2 =", sol.Val(x2))
	}
}
