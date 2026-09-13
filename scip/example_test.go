package scip_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/egoisutolabs/scipgo/scip"
)

func Example() {
	model := scip.NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("example").Maximize()
	x := model.AddVar(0, 10, 3, "x", scip.VarTypeInteger)
	y := model.AddVar(0, 10, 2, "y", scip.VarTypeInteger)
	model.AddCons([]scip.Variable{x, y}, []float64{1, 1}, scip.NegInfinity, 7, "c")

	solved := model.Solve()
	sol, _ := solved.BestSol()
	fmt.Println(solved.Status(), sol.ObjVal(), int(math.Round(sol.Val(x))), int(math.Round(sol.Val(y))))
	solved.Free()
	// Output: Optimal 21 7 0
}

// The builder API states the same model with named steps.
func ExampleNewVar() {
	values := []float64{10, 13, 7, 4, 9, 12}
	weights := []float64{5, 7, 4, 3, 5, 6}

	model := scip.DefaultModel().HideOutput().Maximize()
	defer model.Free()
	items := make([]scip.Variable, len(values))
	for i := range values {
		items[i] = scip.NewVar().Name(fmt.Sprintf("x%d", i)).Bin().Obj(values[i]).AddTo(model)
	}
	model.Add(scip.NewCons().Name("capacity").Coefs(items, weights).Le(15))

	solved := model.Solve()
	sol, _ := solved.BestSol()
	var chosen []string
	for _, item := range items {
		if sol.Val(item) > 0.5 {
			chosen = append(chosen, item.Name())
		}
	}
	fmt.Println(solved.Status(), sol.ObjVal(), chosen)
	// Output: Optimal 29 [x0 x2 x5]
}

// ReadProb loads a problem from a file in any format SCIP reads.
func ExampleModel_ReadProb() {
	model, err := scip.NewModel().HideOutput().IncludeDefaultPlugins().ReadProb("../data/test/simple.mps")
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}
	defer model.Free()

	solved := model.Solve()
	fmt.Println(solved.Status(), solved.ObjVal(), solved.NVars(), "variables")
	// Output: Optimal 27 2 variables
}

// Every fallible method has a Try form that returns the error the plain
// form would panic with.
func ExampleModel_TryAddVar() {
	model := scip.DefaultModel().HideOutput()
	defer model.Free()

	model.Solve() // the model is now Solved; variables cannot be added
	_, err := model.TryAddVar(0, 1, 1, "late", scip.VarTypeBinary)

	var e *scip.Error
	if errors.As(err, &e) {
		fmt.Println(e.Op, e.Stage, errors.Is(err, scip.RetcodeInvalidCall))
	}
	// Output: AddVar Solved true
}

// A solve stops when its context is done; the incumbent found so far stays
// available.
func ExampleModel_SolveContext() {
	model := scip.DefaultModel().HideOutput().Minimize()
	defer model.Free()
	x := model.AddVar(0, 1, 1, "x", scip.VarTypeBinary)
	model.AddCons([]scip.Variable{x}, []float64{1}, 1, 1, "fix")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done: the solve does not start
	_, err := model.SolveContext(ctx)
	fmt.Println(errors.Is(err, context.Canceled))

	solved, err := model.SolveContext(context.Background())
	fmt.Println(err, solved.Status())
	// Output:
	// true
	// <nil> Optimal
}

// Nonlinear constraints are expression trees built from variables.
func ExampleModel_AddConsNonlinear() {
	model := scip.DefaultModel().HideOutput().Minimize()
	defer model.Free()
	x := model.AddVar(0, 10, 1, "x", scip.VarTypeContinuous)
	y := model.AddVar(0, 10, 1, "y", scip.VarTypeContinuous)

	// minimize x + y subject to x*y >= 1
	model.AddConsNonlinear(x.Expr().Mul(y.Expr()), 1, scip.Infinity, "hyperbola")

	solved := model.Solve()
	fmt.Printf("%v %.2f\n", solved.Status(), solved.ObjVal())
	// Output: Optimal 2.00
}

// ParseExpr accepts SCIP's own expression syntax; variables are resolved
// by name when the constraint is added.
func ExampleParseExpr() {
	model := scip.DefaultModel().HideOutput().Minimize()
	defer model.Free()
	x := model.AddVar(-2, 2, 1, "x", scip.VarTypeContinuous)
	y := model.AddVar(-2, 2, 1, "y", scip.VarTypeContinuous)

	// minimize x + y on the unit disc
	model.AddConsNonlinear(scip.ParseExpr("<x>^2 + <y>^2"), scip.NegInfinity, 1, "disc")

	solved := model.Solve()
	sol, _ := solved.BestSol()
	fmt.Printf("%v x=%.3f y=%.3f\n", solved.Status(), sol.Val(x), sol.Val(y))
	// Output: Optimal x=-0.707 y=-0.707
}

// A starting solution gives SCIP an incumbent before the solve begins.
func ExampleModel_AddSol() {
	model := scip.DefaultModel().HideOutput().Maximize()
	defer model.Free()
	x := model.AddVar(0, 10, 3, "x", scip.VarTypeInteger)
	y := model.AddVar(0, 10, 2, "y", scip.VarTypeInteger)
	model.AddCons([]scip.Variable{x, y}, []float64{1, 1}, scip.NegInfinity, 7, "c")

	start := model.CreateSol()
	start.SetVal(x, 3)
	start.SetVal(y, 4)
	fmt.Println("accepted:", model.AddSol(&start) == nil)

	solved := model.Solve()
	fmt.Println(solved.Status(), solved.ObjVal())
	// Output:
	// accepted: true
	// Optimal 21
}

// FreeTransform returns a solved model to the Problem stage so it can be
// changed and solved again.
func ExampleModel_FreeTransform() {
	model := scip.DefaultModel().HideOutput().Maximize()
	defer model.Free()
	x := model.AddVar(0, 10, 3, "x", scip.VarTypeInteger)
	y := model.AddVar(0, 10, 2, "y", scip.VarTypeInteger)
	model.AddCons([]scip.Variable{x, y}, []float64{1, 1}, scip.NegInfinity, 7, "c")

	first := model.Solve().ObjVal()

	model.FreeTransform()
	model.AddCons([]scip.Variable{x}, []float64{1}, scip.NegInfinity, 3, "cap")
	second := model.Solve().ObjVal()

	fmt.Println(first, second)
	// Output: 21 17
}

// SetParam and GetParam address any SCIP parameter by name.
func ExampleSetParam() {
	model := scip.DefaultModel().HideOutput()
	defer model.Free()

	model, err := scip.SetParam(model, "limits/nodes", int64(500))
	fmt.Println("set:", err)

	var nodes int64
	err = scip.GetParam(model, "limits/nodes", &nodes)
	fmt.Println("get:", nodes, err)

	_, err = scip.SetParam(model, "limits/nosuch", 1)
	fmt.Println(errors.Is(err, scip.RetcodeParameterUnknown))
	// Output:
	// set: <nil>
	// get: 500 <nil>
	// true
}

// SCIP's log can be routed into any io.Writer instead of stdout.
func ExampleModel_SetLogWriter() {
	var buf bytes.Buffer
	model, err := scip.NewModel().SetLogWriter(&buf).IncludeDefaultPlugins().ReadProb("../data/test/simple.mps")
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}
	defer model.Free()

	model.Solve()
	fmt.Println(strings.Contains(buf.String(), "SCIP Status"))
	// Output: true
}

// Exact mode solves in rational arithmetic and reports rational values.
func ExampleModel_EnableExactSolving() {
	model := scip.NewModel().EnableExactSolving().HideOutput().IncludeDefaultPlugins()
	defer model.Free()
	if _, err := model.ReadProb("../data/test/simple.mps"); err != nil {
		fmt.Println("read failed:", err)
		return
	}

	solved := model.Solve()
	sol, _ := solved.BestSol()
	fmt.Println(solved.Status(), sol.ObjValExact().RatString())
	// Output: Optimal 27
}

// firstCandidate is a branching rule that always branches on the first
// fractional candidate.
type firstCandidate struct{ calls int }

func (r *firstCandidate) Execute(_ scip.Model, _ scip.BranchRulePlugin, cands []scip.BranchingCandidate) scip.BranchingResult {
	r.calls++
	return scip.BranchOn(cands[0])
}

// Plugins are Go values implementing an interface, registered through a
// builder.
func ExampleNewBranchRule() {
	model, err := scip.MinimalModel().HideOutput().ReadProb("../data/test/simple.mps")
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}
	defer model.Free()

	rule := &firstCandidate{}
	model.Add(scip.NewBranchRule(rule).Name("first").Desc("branch on the first candidate"))

	solved := model.Solve()
	fmt.Println(solved.Status(), solved.ObjVal(), rule.calls > 0)
	// Output: Optimal 27 true
}

// incumbentLog records every new best solution's objective value.
type incumbentLog struct{ objs []float64 }

func (l *incumbentLog) GetEventMask() scip.EventMask { return scip.EventMaskBestSolFound }

func (l *incumbentLog) Execute(model scip.Model, _ scip.EventhdlrPlugin, _ scip.Event) {
	if sol, ok := model.BestSol(); ok {
		l.objs = append(l.objs, sol.ObjVal())
	}
}

// An event handler reacts to what happens during the solve.
func ExampleNewEventhdlr() {
	model, err := scip.NewModel().HideOutput().IncludeDefaultPlugins().ReadProb("../data/test/simple.mps")
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}
	defer model.Free()

	log := &incumbentLog{}
	model.Add(scip.NewEventhdlr(log).Name("incumbents"))

	solved := model.Solve()
	fmt.Println(solved.Status(), len(log.objs) > 0, log.objs[len(log.objs)-1])
	// Output: Optimal true 27
}

// roundingHeur rounds the LP solution and offers it to SCIP.
type roundingHeur struct{}

func (roundingHeur) Execute(model scip.Model, heur scip.HeuristicPlugin, _ scip.HeurTiming, nodeInf bool) scip.HeurResult {
	if nodeInf {
		return scip.HeurResultDidNotRun
	}
	sol := model.CreateSolFor(heur)
	for _, v := range model.Vars() {
		val := model.CurrentVal(v)
		if v.VarType() != scip.VarTypeContinuous {
			val = math.Round(val)
		}
		sol.SetVal(v, val)
	}
	if err := model.AddSol(&sol); err != nil {
		if errors.Is(err, scip.SolErrorInfeasible) {
			return scip.HeurResultNoSolFound
		}
		panic(err) // a SCIP failure or a constraint handler panic: re-raise it
	}
	return scip.HeurResultFoundSol
}

// A heuristic creates solutions attributed to itself and submits them.
func ExampleNewHeuristic() {
	model, err := scip.MinimalModel().HideOutput().ReadProb("../data/test/simple.mps")
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}
	defer model.Free()

	model.Add(scip.NewHeuristic(roundingHeur{}).Name("myround").Timing(scip.HeurTimingAfterLpNode))

	solved := model.Solve()
	h, _ := solved.FindHeuristic("myround")
	fmt.Println(solved.Status(), h.NCalls() > 0)
	// Output: Optimal true
}

// problemData is shared with plugins through the model datastore.
type problemData struct{ label string }

// SetData attaches a value to the model, keyed by its type; callbacks and
// sub-SCIP copies retrieve it with GetData or MustGetData.
func ExampleSetData() {
	model := scip.DefaultModel()
	defer model.Free()

	scip.SetData(model, &problemData{label: "shift roster"})

	data, ok := scip.GetData[*problemData](model)
	fmt.Println(ok, data.label)
	_, ok = scip.GetData[string](model)
	fmt.Println(ok)
	// Output:
	// true shift roster
	// false
}
