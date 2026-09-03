package scip

import "testing"

type lyingPricer struct{}

func (lyingPricer) GenerateColumns(Model, PricerPlugin, bool) PricerResult {
	return PricerResult{State: PricerResultStateFoundColumns}
}

func TestNothingPricer(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from lying pricer")
		}
	}()
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewPricer(lyingPricer{}))
	model.Solve()
}

type earlyStoppingPricer struct{}

func (earlyStoppingPricer) GenerateColumns(Model, PricerPlugin, bool) PricerResult {
	return PricerResult{State: PricerResultStateStopEarly}
}

func TestEarlyStoppingPricer(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from early stopping pricer")
		}
	}()
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewPricer(earlyStoppingPricer{}))
	model.Solve()
}

type optimalPricer struct{}

func (optimalPricer) GenerateColumns(Model, PricerPlugin, bool) PricerResult {
	return PricerResult{State: PricerResultStateNoColumns}
}

func TestOptimalPricer(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewPricer(optimalPricer{}))
	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
}

type addSameColumnPricer struct {
	added bool
	t     *testing.T
}

func (p *addSameColumnPricer) GenerateColumns(model Model, _ PricerPlugin, _ bool) PricerResult {
	if p.added {
		return PricerResult{State: PricerResultStateNoColumns}
	}
	p.added = true
	nVarsBefore := model.NVars()
	v := model.AddPricedVar(0, 1, 1, "x", VarTypeBinary)
	for _, cons := range model.Conss() {
		model.AddConsCoef(cons, v, 1)
	}
	if model.NVars() != nVarsBefore+1 {
		p.t.Error("variable was not added")
	}
	return PricerResult{State: PricerResultStateFoundColumns}
}

func TestAddSameColumnPricer(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	for _, c := range model.Conss() {
		model.SetConsModifiable(c, true)
	}
	model.Add(NewPricer(&addSameColumnPricer{t: t}))
	solved := model.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("got status %v, want Optimal", solved.Status())
	}
}

type internalSCIPPricerTester struct{ t *testing.T }

func (p internalSCIPPricerTester) GenerateColumns(_ Model, pricer PricerPlugin, _ bool) PricerResult {
	if pricer.Name() != "internal" || pricer.Desc() != "internal pricer" {
		p.t.Error("wrong pricer name/desc")
	}
	if pricer.Priority() != 100 {
		p.t.Error("wrong priority")
	}
	if pricer.IsDelayed() || !pricer.IsActive() {
		p.t.Error("wrong pricer flags")
	}
	return PricerResult{State: PricerResultStateNoColumns}
}

func TestInternalPricer(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewPricer(internalSCIPPricerTester{t: t}).
		Name("internal").
		Desc("internal pricer").
		Priority(100).
		Delay(false))
	model.Solve()
}
