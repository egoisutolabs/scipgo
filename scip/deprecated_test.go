package scip

import "testing"

// Every name deprecated by the v0.3.0 rename stays usable and behaves like
// its replacement.
func TestDeprecatedNames(t *testing.T) {
	model := DefaultModel()

	// Types: aliases of the renamed ones.
	var hb HeurBuilder = NewHeur(nil)
	_ = hb
	var sb SepaBuilder = NewSepa(nil)
	_ = sb
	var eb EventHdlrBuilder = NewEventhdlr(nil)
	_ = eb
	var nb NodeSelBuilder = NewNodesel(nil)
	_ = nb
	var _ HeurPlugin = model.Heuristics()[0]
	var _ Heur = HeuristicPlugin{}
	var _ NodeSel // the interface alias

	// Package functions.
	_ = SourceSepa(SeparatorPlugin{})

	// Model methods.
	if len(model.Heurs()) != len(model.Heuristics()) {
		t.Fatal("Heurs and Heuristics disagree")
	}
	heur := model.Heuristics()[0]
	if err := model.TrySetHeurPriority(heur, heur.Priority()); err != nil {
		t.Fatalf("TrySetHeurPriority: %v", err)
	}
	model.SetHeurPriority(heur, heur.Priority())

	sepa := model.Separators()[0]
	if err := model.TrySetSepaPriority(sepa, sepa.Priority()); err != nil {
		t.Fatalf("TrySetSepaPriority: %v", err)
	}
	model.SetSepaPriority(sepa, sepa.Priority())

	presol := model.Presolvers()[0]
	if err := model.TrySetPresolPriority(presol, presol.Priority()); err != nil {
		t.Fatalf("TrySetPresolPriority: %v", err)
	}
	model.SetPresolPriority(presol, presol.Priority())
}
