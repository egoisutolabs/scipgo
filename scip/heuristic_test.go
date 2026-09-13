package scip

import (
	"strings"
	"testing"
)

func TestFindHeurByName(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp")).Solve()

	heur, ok := model.FindHeuristic("completesol")
	if !ok {
		t.Fatal("completesol is a default heuristic")
	}
	if heur.Name() != "completesol" {
		t.Fatalf("name %q", heur.Name())
	}
	if heur.NCalls() != 0 || heur.NSolsFound() != 0 || heur.NBestSolsFound() != 0 {
		t.Fatal("expected zero stats")
	}
	if _, ok := model.FindHeuristic("definitely_not_a_heuristic"); ok {
		t.Fatal("found a heuristic that does not exist")
	}
}

type noSolutionFoundHeur struct{}

func (noSolutionFoundHeur) Execute(Model, HeuristicPlugin, HeurTiming, bool) HeurResult {
	return HeurResultNoSolFound
}

func TestHeur(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeuristic(noSolutionFoundHeur{}).
		Name("no_sol_found_heur").
		Timing(HeurTimingBeforePresol | HeurTimingAfterPropLoop).
		DispChar('n'))
	model.Solve()
}

type impostorHeur struct{}

func (impostorHeur) Execute(Model, HeuristicPlugin, HeurTiming, bool) HeurResult {
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
	model.Add(NewHeuristic(impostorHeur{}).
		Name("impostor_heur").
		Timing(HeurTimingBeforeNode | HeurTimingAfterLpNode))
	model.Solve()
}

type delayedHeur struct{}

func (delayedHeur) Execute(Model, HeuristicPlugin, HeurTiming, bool) HeurResult {
	return HeurResultDelayed
}

func TestDelayedHeur(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeuristic(delayedHeur{}).Name("delayed_heur").Timing(HeurTimingBeforeNode))
	model.Solve()
}

type didNotRunHeur struct{}

func (didNotRunHeur) Execute(Model, HeuristicPlugin, HeurTiming, bool) HeurResult {
	return HeurResultDidNotRun
}

func TestDidNotRunHeur(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeuristic(didNotRunHeur{}).Name("did_not_run_heur"))
	model.Solve()
}

type foundSolHeur struct{ t *testing.T }

func (h foundSolHeur) Execute(model Model, heur HeuristicPlugin, _ HeurTiming, _ bool) HeurResult {
	// Attribute the solution to this heuristic so it records the creator.
	sol := model.CreateSolFor(heur)
	if creator, ok := sol.Heuristic(); !ok || creator.Name() != "found_sol_heur" {
		h.t.Error("CreateSolFor solution does not record its creator")
	}
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
	model.Add(NewHeuristic(foundSolHeur{t: t}).Name("found_sol_heur"))
	solved := model.Solve()

	heur, ok := solved.FindHeuristic("found_sol_heur")
	if !ok {
		t.Fatal("found_sol_heur not found after solve")
	}
	if heur.NCalls() < 1 {
		t.Fatalf("found_sol_heur ran %d times", heur.NCalls())
	}
	if heur.NSolsFound() != 1 {
		t.Fatalf("NSolsFound = %d, want 1 (solution created via CreateSolFor)", heur.NSolsFound())
	}
	if heur.NBestSolsFound() != 1 {
		t.Fatalf("NBestSolsFound = %d, want 1 (first incumbent)", heur.NBestSolsFound())
	}
}

// TestFoundSolHeurStats asserts the statistics table credits the heuristic
// by name.
func TestFoundSolHeurStats(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeuristic(foundSolHeur{t: t}).Name("found_sol_heur"))
	solved := model.Solve()

	stats := solved.StatsJSON()
	if !strings.Contains(stats, "found_sol_heur") {
		t.Fatalf("statistics JSON does not credit found_sol_heur:\n%s", stats)
	}
}

// TestSolutionHeur pins the two attribution mechanisms SCIP has. The creator
// recorded on the solution is set by the constructor: only the For variants
// record one. The per-heuristic counters are driven by when the solution is
// added: SCIP credits the heuristic executing at AddSol time, whichever
// constructor created the solution.
func TestSolutionHeur(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeuristic(unattributedHeur{t: t}).Name("unattributed_heur"))
	solved := model.Solve()

	heur, ok := solved.FindHeuristic("unattributed_heur")
	if !ok {
		t.Fatal("unattributed_heur not found after solve")
	}
	// The solution was created with plain CreateSol, so it records no
	// creator, but it was added while the heuristic executed, so SCIP
	// still credits the count.
	if heur.NSolsFound() != 1 {
		t.Fatalf("NSolsFound = %d, want 1 (added while executing)", heur.NSolsFound())
	}
}

type unattributedHeur struct{ t *testing.T }

func (h unattributedHeur) Execute(model Model, _ HeuristicPlugin, _ HeurTiming, _ bool) HeurResult {
	sol := model.CreateSol()
	if _, ok := sol.Heuristic(); ok {
		h.t.Error("plain CreateSol solution records a creator")
	}
	for _, v := range model.Vars() {
		sol.SetVal(v, 1)
	}
	if err := model.AddSol(&sol); err != nil {
		h.t.Error("add_sol failed")
	}
	return HeurResultFoundSol
}

// TestMipStartAttribution documents the other side: a MIP-start seed added
// before Solve records its creator (when created with a For constructor) but
// counts for no heuristic, because no heuristic is executing when it is
// added.
func TestMipStartAttribution(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		IncludeDefaultPlugins(), testFile("simple.lp"))
	model.Add(NewHeuristic(neverRunsHeur{}).Name("seed_heur").Freq(-1))
	h, ok := model.FindHeuristic("seed_heur")
	if !ok {
		t.Fatal("seed_heur not found")
	}
	seed := model.CreateOrigSolFor(h)
	if creator, ok := seed.Heuristic(); !ok || creator.Name() != "seed_heur" {
		t.Fatal("CreateOrigSolFor seed does not record its creator")
	}
	for _, v := range model.Vars() {
		seed.SetVal(v, 1)
	}
	if err := model.AddSol(&seed); err != nil {
		t.Fatalf("add seed: %v", err)
	}
	solved := model.Solve()
	heur, _ := solved.FindHeuristic("seed_heur")
	if heur.NCalls() != 0 || heur.NSolsFound() != 0 {
		t.Fatalf("calls=%d sols=%d, want 0/0: nothing was added while it executed", heur.NCalls(), heur.NSolsFound())
	}
}

type neverRunsHeur struct{}

func (neverRunsHeur) Execute(Model, HeuristicPlugin, HeurTiming, bool) HeurResult {
	return HeurResultDidNotRun
}

func TestPluginGetters(t *testing.T) {
	m := NewModel().HideOutput().IncludeDefaultPlugins()
	defer m.Free()
	if len(m.Heuristics()) == 0 || len(m.Separators()) == 0 || len(m.Presolvers()) == 0 {
		t.Fatalf("heurs=%d sepas=%d presols=%d", len(m.Heuristics()), len(m.Separators()), len(m.Presolvers()))
	}
	h, ok := m.FindHeuristic("rounding")
	if !ok || h.Desc() == "" {
		t.Fatal("rounding heuristic not found")
	}
	h.SetFreq(-1)
	m.SetHeuristicPriority(h, 42)
	if h.Freq() != -1 || h.Priority() != 42 {
		t.Fatalf("freq=%d prio=%d", h.Freq(), h.Priority())
	}
	s, ok := m.FindSeparator("gomory")
	if !ok {
		t.Fatal("gomory separator not found")
	}
	s.SetFreq(0)
	m.SetSeparatorPriority(s, 7)
	if s.Freq() != 0 || s.Priority() != 7 {
		t.Fatalf("sepa freq=%d prio=%d", s.Freq(), s.Priority())
	}
	p, ok := m.FindPresolver("trivial")
	if !ok || p.Name() != "trivial" || p.NCalls() != 0 {
		t.Fatalf("presolver %+v ok=%v", p, ok)
	}
	m.SetPresolverPriority(p, 9)
	if p.Priority() != 9 {
		t.Fatalf("presol prio=%d", p.Priority())
	}
	if _, ok := m.FindPresolver("nope"); ok {
		t.Fatal("found nonexistent presolver")
	}
}
