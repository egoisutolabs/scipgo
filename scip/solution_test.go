package scip

import "testing"

func TestCreateSol(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Minimize()

	x1 := model.AddVar(0, 1, 3, "x1", VarTypeBinary)
	x2 := model.AddVar(0, 1, 4, "x2", VarTypeBinary)
	cons1 := model.AddConsSetPart(nil, "c")
	model.AddConsCoefSetppc(cons1, x1)
	model.AddConsSetPack([]Variable{x2}, "c")

	infSol := model.CreateOrigSol()
	infSol.SetVal(x1, 2)
	if err := model.AddSol(&infSol); err == nil {
		t.Fatal("expected infeasible solution error")
	}

	sol := model.CreateOrigSol()
	if sol.ObjVal() != 0 {
		t.Fatalf("fresh sol obj %v, want 0", sol.ObjVal())
	}
	sol.SetVal(x1, 1)
	sol.SetVal(x2, 1)
	if sol.ObjVal() != 7 {
		t.Fatalf("sol obj %v, want 7", sol.ObjVal())
	}
	if err := model.AddSol(&sol); err != nil {
		t.Fatalf("add_sol: %v", err)
	}
	if model.NSols() != 1 {
		t.Fatalf("expected 1 solution, got %d", model.NSols())
	}

	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
	if solved.NSols() < 2 {
		t.Fatalf("expected at least 2 solutions, got %d", solved.NSols())
	}
}

func TestCreatePartialSol(t *testing.T) {
	solveAndCount := func(seed bool) (int, int) {
		model := NewModel().
			HideOutput().
			IncludeDefaultPlugins().
			CreateProb("test").
			Minimize().
			SetPresolving(ParamSettingOff)

		x1 := model.AddVar(0, 1, 1, "x1", VarTypeBinary)
		x2 := model.AddVar(0, 1, 1, "x2", VarTypeBinary)
		x3 := model.AddVar(0, 1, 1, "x3", VarTypeBinary)
		model.AddCons([]Variable{x1, x2}, []float64{1, 1}, 1, 2, "e12")
		model.AddCons([]Variable{x2, x3}, []float64{1, 1}, 1, 2, "e23")
		model.AddCons([]Variable{x1, x3}, []float64{1, 1}, 1, 2, "e13")

		if seed {
			partial := model.CreatePartialSol()
			if !partial.IsPartial() {
				t.Fatal("partial solution not reported as partial")
			}
			partial.SetVal(x1, 1)
			full := model.CreateOrigSol()
			if full.IsPartial() {
				t.Fatal("full solution reported as partial")
			}
			_ = model.AddSol(&full) // infeasible all-zero; consumes
			if err := model.AddSol(&partial); err != nil {
				t.Fatalf("add partial: %v", err)
			}
		}

		solved := model.Solve()
		if solved.Status() != StatusOptimal {
			t.Fatalf("got status %v", solved.Status())
		}
		completesol, ok := solved.FindHeur("completesol")
		if !ok {
			t.Fatal("completesol heuristic not found")
		}
		return completesol.NCalls(), completesol.NSolsFound()
	}

	calls, found := solveAndCount(true)
	if calls < 1 {
		t.Fatal("completesol should run when a partial solution exists")
	}
	if found < 1 {
		t.Fatal("completesol should complete the partial seed")
	}
	if calls2, _ := solveAndCount(false); calls2 != 0 {
		t.Fatal("completesol should never fire without a partial solution")
	}
}

func TestAddSolConsumes(t *testing.T) {
	model := NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("t").Minimize()
	x := model.AddVar(0, 1, 1, "x", VarTypeBinary)
	sol := model.CreateOrigSol()
	sol.SetVal(x, 1)
	if err := model.AddSol(&sol); err != nil {
		t.Fatal(err)
	}
	if sol.raw != nil {
		t.Fatal("solution still holds a freed pointer")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic using a consumed solution")
		}
	}()
	sol.Val(x)
}

func TestGetSols(t *testing.T) {
	model := MinimalModel().SetDisplayVerbosity(0).Maximize()
	model.Add(NewVar().Bin())
	solved := model.Solve()
	sols := solved.GetSols()
	if solved.NSols() != len(sols) {
		t.Fatalf("n_sols %d != len(sols) %d", solved.NSols(), len(sols))
	}
	if len(sols) > 1 {
		t.Fatal("expected at most one solution")
	}
}

func solvedBestSol(m Model) (Solution, bool) { return m.BestSol() }

func TestSolMethods(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp")).Solve()

	if model.Status() != StatusOptimal {
		t.Fatalf("got status %v", model.Status())
	}

	sol, ok := model.BestSol()
	if !ok || sol.Inner() == nil {
		t.Fatal("missing best sol")
	}

	debugStr := sol.String()
	if !contains(debugStr, "Solution with obj val: 200") ||
		!contains(debugStr, "Var x1=40") || !contains(debugStr, "Var x2=20") {
		t.Fatalf("bad solution string: %q", debugStr)
	}

	vars := model.Vars()
	if sol.Val(vars[0]) != 40 || sol.Val(vars[1]) != 20 {
		t.Fatal("wrong solution values")
	}
	if sol.ObjVal() != model.ObjVal() {
		t.Fatal("obj values differ")
	}

	nameMap := sol.AsNameMap()
	if nameMap["t_x1"] != 40 || nameMap["t_x2"] != 20 {
		t.Fatalf("bad name map: %v", nameMap)
	}
	idMap := sol.AsIDMap()
	if idMap[0] != 40 || idMap[1] != 20 {
		t.Fatalf("bad id map: %v", idMap)
	}
}
