package scip

import "testing"

func TestAddConsCoef(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, Infinity, 3, "x1", VarTypeInteger)
	x2 := model.AddVar(0, Infinity, 4, "x2", VarTypeInteger)
	cons := model.AddCons(nil, nil, NegInfinity, 10, "c1")

	model.AddConsCoef(cons, x1, 0)  // x1 is unconstrained
	model.AddConsCoef(cons, x2, 10) // x2 can't be used

	solved := model.Solve()
	if solved.Status() != StatusUnbounded {
		t.Fatalf("got status %v, want Unbounded", solved.Status())
	}
}

func TestSetCoverPartitioningAndPacking(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Minimize()

	x1 := model.AddVar(0, 1, 3, "x1", VarTypeBinary)
	x2 := model.AddVar(0, 1, 4, "x2", VarTypeBinary)
	cons1 := model.AddConsSetPart(nil, "c")
	model.AddConsCoefSetppc(cons1, x1)

	model.AddConsSetCover([]Variable{x2}, "c")
	model.AddConsSetPack([]Variable{x2}, "c")

	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
	if solved.ObjVal() != 7 {
		t.Fatalf("got obj %v, want 7", solved.ObjVal())
	}
}

func TestCardinalityConstraint(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, 10, 4, "x1", VarTypeContinuous)
	x2 := model.AddVar(0, 10, 2, "x2", VarTypeInteger)
	x3 := model.AddVar(0, 10, 3, "x3", VarTypeInteger)
	model.AddConsCardinality([]Variable{x1, x2, x3}, 2, "cardinality")

	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
	if solved.ObjVal() != 70 {
		t.Fatalf("got obj %v, want 70", solved.ObjVal())
	}
	sol, _ := solved.BestSol()
	if sol.Val(x1) != 10 || sol.Val(x2) != 0 || sol.Val(x3) != 10 {
		t.Fatalf("unexpected solution values")
	}
}

func TestIndicatorConstraint(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, 10, 1, "x1", VarTypeInteger)
	x2 := model.AddVar(0, 10, 1, "x2", VarTypeInteger)
	b := model.AddVar(0, 1, 0, "b", VarTypeBinary)

	model.AddConsIndicator(b, []Variable{x1, x2}, []float64{1, -1}, -1, "indicator")
	model.AddCons([]Variable{b}, []float64{1}, 1, 1, "c1")

	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
	if solved.ObjVal() != 19 {
		t.Fatalf("got obj %v, want 19", solved.ObjVal())
	}
	sol, _ := solved.BestSol()
	if sol.Val(x1) != 9 || sol.Val(x2) != 10 || sol.Val(b) != 1 {
		t.Fatalf("unexpected solution values")
	}
}

func TestQuadraticConstraint(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, 1, 1, "x1", VarTypeContinuous)
	x2 := model.AddVar(0, 1, 1, "x2", VarTypeContinuous)
	model.AddConsQuadratic(nil, nil, []Variable{x1, x2}, []Variable{x1, x2},
		[]float64{1, 1}, 0, 1, "circle")

	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
	// max manhattan distance in unit circle = sqrt(2)
	if d := 1.4142135623730951 - solved.ObjVal(); d > 1e-3 || d < -1e-3 {
		t.Fatalf("obj %v, want sqrt(2)", solved.ObjVal())
	}
}

func TestSOS1Constraint(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, 10, 4, "x1", VarTypeContinuous)
	x2 := model.AddVar(0, 10, 2, "x2", VarTypeContinuous)
	x3 := model.AddVar(0, 10, 3, "x3", VarTypeContinuous)
	model.AddConsSOS1([]Variable{x1, x2, x3}, nil, "sos1")

	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v", solved.Status())
	}
	sol, _ := solved.BestSol()
	if sol.Val(x1) != 10 || sol.Val(x2) != 0 || sol.Val(x3) != 0 {
		t.Fatal("unexpected solution")
	}
	if solved.ObjVal() != 40 {
		t.Fatalf("got obj %v, want 40", solved.ObjVal())
	}
}

func TestSOS1ConstraintWithWeights(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, 10, 1, "x1", VarTypeContinuous)
	x2 := model.AddVar(0, 10, 1, "x2", VarTypeContinuous)
	x3 := model.AddVar(0, 10, 1, "x3", VarTypeContinuous)
	model.AddConsSOS1([]Variable{x1, x2, x3}, []float64{3, 1, 2}, "sos1")

	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v", solved.Status())
	}
	sol, _ := solved.BestSol()
	if sol.Val(x1) != 10 || sol.Val(x2) != 0 || sol.Val(x3) != 0 {
		t.Fatal("unexpected solution")
	}
	if solved.ObjVal() != 10 {
		t.Fatalf("got obj %v, want 10", solved.ObjVal())
	}
}

func TestConstraintMemSafety(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()
	x1 := model.AddVar(0, Infinity, 3, "x1", VarTypeInteger)
	cons := model.AddCons([]Variable{x1}, []float64{1}, 4, 4, "cons")
	_ = model
	if cons.Name() != "cons" {
		t.Fatal("constraint name lost")
	}
}

func TestConstraintTransformedNoTransformed(t *testing.T) {
	model := MinimalModel().HideOutput().Maximize()
	x1 := model.AddVar(0, Infinity, 10, "x1", VarTypeContinuous)
	cons := model.AddCons([]Variable{x1}, []float64{1}, 0, 5, "cons")
	model.Solve()
	if _, ok := solvedBestSol(model); !ok {
		t.Fatal("expected a solution")
	}
	if _, ok := cons.Transformed(); ok {
		t.Fatal("expected no transformed constraint")
	}
}

func TestConstraintTransformedWithTransformed(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("prob").
		Maximize()
	x1 := model.AddVar(0, Infinity, 10, "x1", VarTypeContinuous)
	cons := model.AddCons([]Variable{x1}, []float64{1}, 0, 5, "cons")
	model.SetConsModifiable(cons, true)

	model.Solve()
	if _, ok := solvedBestSol(model); !ok {
		t.Fatal("expected a solution")
	}
	trans, ok := cons.Transformed()
	if !ok {
		t.Fatal("expected transformed constraint")
	}
	dual, ok := trans.DualSol()
	if !ok {
		t.Fatal("expected dual solution")
	}
	if dual+10 >= 2.220446049250313e-16 {
		t.Fatalf("dual %v", dual)
	}
}
