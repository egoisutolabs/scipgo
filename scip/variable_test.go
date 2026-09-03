package scip

import "testing"

func TestVarData(t *testing.T) {
	model := NewModel().IncludeDefaultPlugins().CreateProb("test")
	v := model.AddVar(0, 1, 2, "x", VarTypeContinuous)

	if v.Index() != 0 {
		t.Fatalf("index %d", v.Index())
	}
	if v.Lb() != 0 || v.LbLocal() != 0 || v.LbGlobal() != 0 {
		t.Fatal("wrong lower bound")
	}
	if v.Ub() != 1 || v.UbLocal() != 1 || v.UbGlobal() != 1 {
		t.Fatal("wrong upper bound")
	}
	if v.Obj() != 2 {
		t.Fatal("wrong objective")
	}
	if v.Name() != "x" {
		t.Fatal("wrong name")
	}
	if v.VarType() != VarTypeContinuous {
		t.Fatal("wrong type")
	}
	if v.Status() != VarStatusOriginal {
		t.Fatal("wrong status")
	}
	if v.IsInLP() || v.IsDeleted() || v.IsTransformed() || v.IsNegated() || v.IsRemovable() {
		t.Fatal("unexpected flags")
	}
	if !v.IsOriginal() || !v.IsActive() {
		t.Fatal("expected original and active")
	}
	if v.Inner() == nil {
		t.Fatal("nil inner pointer")
	}
}

func TestVarMemorySafety(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()
	x1 := model.AddVar(0, Infinity, 3, "x1", VarTypeInteger)

	_ = model // model dropped here in Rust; keep x1 alive past it
	if x1.Name() != "x1" {
		t.Fatal("variable name lost")
	}
}

func TestVarSolVal(t *testing.T) {
	model := MinimalModel()
	x := model.AddVar(0, 1, 1, "x", VarTypeBinary)
	model.AddCons([]Variable{x}, []float64{1}, 1, 1, "cons1")
	model.Solve()
	if x.SolVal() != 1 {
		t.Fatalf("sol val %v", x.SolVal())
	}
}

type redcostPricer struct{ t *testing.T }

func (p redcostPricer) GenerateColumns(model Model, _ PricerPlugin, farkas bool) PricerResult {
	if len(model.Vars()) > 3 {
		return PricerResult{State: PricerResultStateNoColumns}
	}
	conss := model.Conss()
	cons1, cons2 := conss[0], conss[1]
	dual1, ok1 := cons1.DualSol()
	dual2, ok2 := cons2.DualSol()
	if !ok1 || !ok2 {
		p.t.Fatal("missing duals")
		return PricerResult{State: PricerResultStateNoColumns}
	}
	c := 1.0
	rc := c - dual1 - dual2
	v := model.AddPricedVar(0, 1, c, "testvar", VarTypeContinuous)
	model.AddConsCoef(cons1, v, 1)
	model.AddConsCoef(cons2, v, 1)
	if got, ok := v.Redcost(); !ok || got != rc {
		p.t.Fatalf("redcost %v (ok=%v), want %v", got, ok, rc)
	}
	return PricerResult{State: PricerResultStateFoundColumns}
}

func TestVarRedcost(t *testing.T) {
	model := MinimalModel()
	model, err := model.SetLongintParam("limits/nodes", 3)
	if err != nil {
		t.Fatal(err)
	}
	model = model.Minimize()
	x := model.AddVar(0, 1, 10.3, "x", VarTypeBinary)
	y := model.AddVar(0, 1, 5.5, "y", VarTypeBinary)
	cons1 := model.AddCons(nil, nil, 5, Infinity, "")
	cons2 := model.AddCons(nil, nil, 10, Infinity, "")
	model.SetConsModifiable(cons1, true)
	model.SetConsModifiable(cons2, true)
	model.AddConsCoef(cons1, y, 10)
	model.AddConsCoef(cons2, x, 10)

	model.Add(NewPricer(redcostPricer{t: t}))

	if _, ok := x.Redcost(); ok {
		t.Fatal("expected no redcost before solving")
	}

	model.Solve()
	if _, ok := x.Redcost(); !ok {
		t.Fatal("expected redcost after solving")
	}
	if _, ok := x.Transformed(); !ok {
		t.Fatal("expected transformed variable")
	}
}
