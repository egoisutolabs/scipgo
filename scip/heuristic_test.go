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
