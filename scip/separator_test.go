package scip

import "testing"

type notRunningSeparator struct{}

func (notRunningSeparator) ExecuteLP(Model, SeparatorPlugin) SeparationResult {
	return SeparationResultDidNotRun
}

func TestNotRunningSeparator(t *testing.T) {
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 2)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewSepa(notRunningSeparator{}).
		Name("NotRunningSeparator").
		Desc("Does not run the separation routine"))
	model.Solve()
}

type consAddingSeparator struct{}

func (consAddingSeparator) ExecuteLP(model Model, _ SeparatorPlugin) SeparationResult {
	vars := model.Vars()
	coefs := make([]float64, len(vars))
	for i := range coefs {
		coefs[i] = 1.0
	}
	model.AddCons(vars, coefs, 5, 5, "sep_cons")
	return SeparationResultConsAdded
}

func TestConsAddingSeparator(t *testing.T) {
	model := MinimalModel().HideOutput().Maximize()
	x := model.AddVar(0, 1, 1, "x", VarTypeBinary)
	y := model.AddVar(0, 1, 1, "y", VarTypeBinary)
	model.AddCons([]Variable{x, y}, []float64{1, 1}, 1, 1, "cons1")
	model.Add(NewSepa(consAddingSeparator{}).
		Name("ConsAddingSeparator").
		Desc("Adds a constraint to the model"))
	solved := model.Solve()
	if solved.Status() != StatusInfeasible {
		t.Fatalf("got status %v, want Infeasible", solved.Status())
	}
}

type internalSeparatorDataTester struct{ t *testing.T }

func (s internalSeparatorDataTester) ExecuteLP(model Model, sep SeparatorPlugin) SeparationResult {
	if sep.Name() != "InternalSeparatorDataTester" || sep.Desc() != "Internal separator data tester" {
		s.t.Error("wrong separator name/desc")
	}
	if sep.Priority() != 1000000 || sep.Freq() != 1 || sep.MaxBoundDist() != 1.0 || sep.IsDelayed() {
		s.t.Error("wrong separator settings")
	}

	row := NewRow().
		Bounds(0, 1).
		Removable(false).
		Local(false).
		Modifiable(true).
		Name("test").
		Source(SourceSepa(sep)).
		AddTo(model)
	if row.Name() != "test" || row.Lhs() != 0 || row.Rhs() != 1 {
		s.t.Error("wrong row name/bounds")
	}
	if row.NNonZeroes() != 0 || !row.IsModifiable() || row.IsLocal() || row.IsRemovable() {
		s.t.Error("wrong row flags")
	}
	if row.OriginType() != RowOriginSeparator {
		s.t.Error("wrong row origin")
	}
	return SeparationResultDidNotRun
}

func TestInternalScipSeparator(t *testing.T) {
	model := NewModel()
	model, err := model.SetLongintParam("limits/nodes", 2)
	if err != nil {
		t.Fatal(err)
	}
	model = model.HideOutput().IncludeDefaultPlugins()
	model = mustRead(t, model, testFile("gen-ip054.mps"))
	model.Add(NewSepa(internalSeparatorDataTester{t: t}).
		Name("InternalSeparatorDataTester").
		Desc("Internal separator data tester").
		Priority(1000000).
		Freq(1).
		MaxBoundDist(1.0).
		UsesSubscip(false).
		Delay(false))
	model.Solve()
}

type cutsAddingSeparator struct{ t *testing.T }

func (c cutsAddingSeparator) ExecuteLP(model Model, sep SeparatorPlugin) SeparationResult {
	row := NewRow().
		Name("test").
		Eq(5).
		Local(true).
		Modifiable(false).
		Removable(false).
		Source(SourceSepa(sep)).
		AddTo(model)
	if row.Lhs() != 5 || row.Rhs() != 5 {
		c.t.Error("wrong row bounds")
	}
	if row.NNonZeroes() != 0 || !row.IsLocal() || row.IsModifiable() || row.IsRemovable() {
		c.t.Error("wrong row flags")
	}
	if row.OriginType() != RowOriginSeparator {
		c.t.Error("wrong row origin")
	}

	vars := model.Vars()
	for _, v := range vars {
		row.SetCoeff(v, 1)
	}
	model.AddCut(row, true)
	nConssBefore := model.NConss()
	model.AddConsLocal(NewCons().Ge(7).Coef(vars[0], 2).Coef(vars[1], 1))
	if model.NConss() != nConssBefore+1 {
		c.t.Error("local constraint not added")
	}
	return SeparationResultSeparated
}

func TestCutsAdding(t *testing.T) {
	model := MinimalModel().HideOutput().Maximize()
	x := model.AddVar(0, 1, 1, "x", VarTypeBinary)
	y := model.AddVar(0, 1, 1, "y", VarTypeBinary)
	model.AddCons([]Variable{x, y}, []float64{1, 1}, 1, 1, "cons1")
	model.Add(NewSepa(cutsAddingSeparator{t: t}).
		Name("CutsAddingSeparator").
		Desc("Adds a cut to the model"))
	solved := model.Solve()
	if solved.Status() != StatusInfeasible {
		t.Fatalf("got status %v, want Infeasible", solved.Status())
	}
}
