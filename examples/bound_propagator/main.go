// A propagator that caps one variable's upper bound. With presolving off,
// the cap is what makes the objective 32 rather than 40.
package main

import (
	"fmt"
	"os"

	"github.com/egoisutolabs/scipgo/scip"
)

type capUpper struct {
	v  scip.Variable
	ub float64
}

func (c capUpper) Propagate(model scip.Model, _ scip.PropagatorPlugin) scip.PropResult {
	infeasible, tightened, err := model.TryTightenVarUb(c.v, c.ub)
	if err != nil {
		panic(err)
	}
	if infeasible {
		return scip.PropResultCutOff
	}
	if tightened {
		return scip.PropResultReducedDom
	}
	return scip.PropResultDidNotFind
}

func main() {
	model := scip.DefaultModel().HideOutput().SetPresolving(scip.ParamSettingOff).Maximize()
	x := model.AddVar(0, 10, 1, "x", scip.VarTypeInteger)
	y := model.AddVar(0, 10, 3, "y", scip.VarTypeInteger)
	// Locks both variables upwards. Without it, SCIP's dualfix propagator
	// fixes an unconstrained maximization at the upper bounds before capx runs.
	// The objectives differ so symmetry does not copy the cap from x onto y.
	model.Add(scip.NewCons().Coef(x, 1).Coef(y, 1).Le(100))
	model.Add(scip.NewPropagator(capUpper{v: x, ub: 2}).Name("capx").Desc("x <= 2"))

	solved := model.Solve()
	if solved.Status() != scip.StatusOptimal || solved.ObjVal() != 32 {
		fmt.Printf("got %v %v, want Optimal 32\n", solved.Status(), solved.ObjVal())
		os.Exit(1)
	}
	fmt.Println(solved.Status(), solved.ObjVal())
}
