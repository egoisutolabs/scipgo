package scip

import "testing"

func TestFindHeurByName(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp")).Solve()

	heur, ok := model.FindHeur("completesol")
	if !ok {
		t.Fatal("completesol is a default heuristic")
	}
	if heur.Name() != "completesol" {
		t.Fatalf("name %q", heur.Name())
	}
	if heur.NCalls() != 0 || heur.NSolsFound() != 0 || heur.NBestSolsFound() != 0 {
		t.Fatal("expected zero stats")
	}
	if _, ok := model.FindHeur("definitely_not_a_heuristic"); ok {
		t.Fatal("found a heuristic that does not exist")
	}
}

type noSolutionFoundHeur struct{}

func (noSolutionFoundHeur) Execute(Model, HeurTiming, bool) HeurResult {
	return HeurResultNoSolFound
}

func TestHeur(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeur(noSolutionFoundHeur{}).
		Name("no_sol_found_heur").
		Timing(HeurTimingBeforePresol | HeurTimingAfterPropLoop).
		DispChar('n'))
	model.Solve()
}

type impostorHeur struct{}

func (impostorHeur) Execute(Model, HeurTiming, bool) HeurResult {
	return HeurResultFoundSol
}

func TestImpostorHeur(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from impostor heuristic")
		}
	}()
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeur(impostorHeur{}).
		Name("impostor_heur").
		Timing(HeurTimingBeforeNode | HeurTimingAfterLpNode))
	model.Solve()
}

type delayedHeur struct{}

func (delayedHeur) Execute(Model, HeurTiming, bool) HeurResult {
	return HeurResultDelayed
}

func TestDelayedHeur(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeur(delayedHeur{}).Name("delayed_heur").Timing(HeurTimingBeforeNode))
	model.Solve()
}

type didNotRunHeur struct{}

func (didNotRunHeur) Execute(Model, HeurTiming, bool) HeurResult {
	return HeurResultDidNotRun
}

func TestDidNotRunHeur(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeur(didNotRunHeur{}).Name("did_not_run_heur"))
	model.Solve()
}

type foundSolHeur struct{ t *testing.T }

func (h foundSolHeur) Execute(model Model, _ HeurTiming, _ bool) HeurResult {
	sol := model.CreateSol()
	for _, v := range model.Vars() {
		sol.SetVal(v, 1)
	}
	if sol.ObjVal() != 7 {
		h.t.Error("wrong solution obj value")
	}
	if err := model.AddSol(&sol); err != nil {
		h.t.Error("add_sol failed")
	}
	return HeurResultFoundSol
}

func TestFoundSolHeur(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeur(foundSolHeur{t: t}).Name("found_sol_heur"))
	model.Solve()
}

func TestPluginGetters(t *testing.T) {
	m := NewModel().HideOutput().IncludeDefaultPlugins()
	defer m.Free()
	if len(m.Heurs()) == 0 || len(m.Separators()) == 0 || len(m.Presolvers()) == 0 {
		t.Fatalf("heurs=%d sepas=%d presols=%d", len(m.Heurs()), len(m.Separators()), len(m.Presolvers()))
	}
	h, ok := m.FindHeur("rounding")
	if !ok || h.Desc() == "" {
		t.Fatal("rounding heuristic not found")
	}
	h.SetFreq(-1)
	m.SetHeurPriority(h, 42)
	if h.Freq() != -1 || h.Priority() != 42 {
		t.Fatalf("freq=%d prio=%d", h.Freq(), h.Priority())
	}
	s, ok := m.FindSeparator("gomory")
	if !ok {
		t.Fatal("gomory separator not found")
	}
	s.SetFreq(0)
	m.SetSepaPriority(s, 7)
	if s.Freq() != 0 || s.Priority() != 7 {
		t.Fatalf("sepa freq=%d prio=%d", s.Freq(), s.Priority())
	}
	p, ok := m.FindPresolver("trivial")
	if !ok || p.Name() != "trivial" || p.NCalls() != 0 {
		t.Fatalf("presolver %+v ok=%v", p, ok)
	}
	m.SetPresolPriority(p, 9)
	if p.Priority() != 9 {
		t.Fatalf("presol prio=%d", p.Priority())
	}
	if _, ok := m.FindPresolver("nope"); ok {
		t.Fatal("found nonexistent presolver")
	}
}
