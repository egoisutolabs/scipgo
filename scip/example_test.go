package scip_test

import (
	"fmt"
	"math"

	"github.com/egoisutolabs/scipgo/scip"
)

func Example() {
	model := scip.NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("example").Maximize()
	x := model.AddVar(0, 10, 3, "x", scip.VarTypeInteger)
	y := model.AddVar(0, 10, 2, "y", scip.VarTypeInteger)
	model.AddCons([]scip.Variable{x, y}, []float64{1, 1}, scip.NegInfinity, 7, "c")

	solved := model.Solve()
	sol, _ := solved.BestSol()
	fmt.Println(solved.Status(), sol.ObjVal(), math.Round(sol.Val(x)), math.Round(sol.Val(y)))
	solved.Free()
	// Output: Optimal 21 7 0
}
