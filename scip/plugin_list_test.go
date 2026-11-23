package scip

import (
	"errors"
	"testing"
)

// TestBuiltinPluginListing checks that SCIP 10's built-in plugins are
// reachable by name and that a plugin registered from Go shows up in the
// same list. These getters are legal in every stage, including Init; the
// freed-model case lives in queryCalls.
func TestBuiltinPluginListing(t *testing.T) {
	m := NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("plugins")
	defer m.Free()

	rule, ok := m.FindBranchRule("relpscost")
	if !ok || rule.Name() != "relpscost" || rule.Desc() != "reliability branching on pseudo cost values" {
		t.Fatalf("relpscost: ok=%v name=%q desc=%q", ok, rule.Name(), rule.Desc())
	}
	if rule.Priority() != 10000 || rule.MaxDepth() != -1 || rule.MaxBoundDist() != 1 {
		t.Fatalf("relpscost settings: prio=%d depth=%d dist=%v", rule.Priority(), rule.MaxDepth(), rule.MaxBoundDist())
	}
	if !hasPlugin(m.BranchRules(), "relpscost") {
		t.Fatal("BranchRules does not contain relpscost")
	}
	if _, ok := m.FindBranchRule("no-such-rule"); ok {
		t.Fatal("found nonexistent branch rule")
	}
	m.SetBranchRulePriority(rule, 17)
	if rule.Priority() != 17 {
		t.Fatalf("branch priority %d", rule.Priority())
	}
	m.IncludeBranchRule("listed-rule", "a listed rule", 3, -1, 1, listedBranchRule{})
	listed, ok := m.FindBranchRule("listed-rule")
	if !ok || listed.Priority() != 3 || listed.Desc() != "a listed rule" || !hasPlugin(m.BranchRules(), "listed-rule") {
		t.Fatalf("custom branch rule: ok=%v prio=%d desc=%q", ok, listed.Priority(), listed.Desc())
	}

	bfs, ok := m.FindNodesel("bfs")
	if !ok || bfs.Name() != "bfs" || bfs.Desc() != "best first search" {
		t.Fatalf("bfs: ok=%v name=%q desc=%q", ok, bfs.Name(), bfs.Desc())
	}
	if bfs.StdPriority() != 100000 || bfs.MemSavePriority() != 0 {
		t.Fatalf("bfs priorities: std=%d mem=%d", bfs.StdPriority(), bfs.MemSavePriority())
	}
	if !hasPlugin(m.Nodesels(), "bfs") {
		t.Fatal("Nodesels does not contain bfs")
	}
	m.SetNodeselStdPriority(bfs, 11)
	m.SetNodeselMemSavePriority(bfs, 13)
	if bfs.StdPriority() != 11 || bfs.MemSavePriority() != 13 {
		t.Fatalf("nodesel priorities: std=%d mem=%d", bfs.StdPriority(), bfs.MemSavePriority())
	}
	m.IncludeNodesel("listed-nodesel", "a listed selector", 4, 5, listedNodesel{})
	ns, ok := m.FindNodesel("listed-nodesel")
	if !ok || ns.StdPriority() != 4 || ns.MemSavePriority() != 5 || !hasPlugin(m.Nodesels(), "listed-nodesel") {
		t.Fatalf("custom nodesel: ok=%v std=%d mem=%d", ok, ns.StdPriority(), ns.MemSavePriority())
	}

	if len(m.Pricers()) != 0 {
		t.Fatalf("default model has %d pricers", len(m.Pricers()))
	}
	if _, ok := m.FindPricer("listed-pricer"); ok {
		t.Fatal("pricer found before inclusion")
	}
	m.IncludePricer("listed-pricer", "a listed pricer", 8, true, listedPricer{})
	pr, ok := m.FindPricer("listed-pricer")
	if !ok || pr.Name() != "listed-pricer" || pr.Desc() != "a listed pricer" || pr.Priority() != 8 || !pr.IsDelayed() {
		t.Fatalf("pricer: ok=%v name=%q desc=%q prio=%d delay=%v", ok, pr.Name(), pr.Desc(), pr.Priority(), pr.IsDelayed())
	}
	if !pr.IsActive() || !hasPlugin(m.Pricers(), "listed-pricer") {
		t.Fatal("included pricer is not active or not listed")
	}

	linear, ok := m.FindConshdlr("linear")
	if !ok || linear.Name() != "linear" || linear.Desc() != "linear constraints of the form  lhs <= a^T x <= rhs" {
		t.Fatalf("linear: ok=%v name=%q desc=%q", ok, linear.Name(), linear.Desc())
	}
	if linear.EnfoPriority() != -1000000 || linear.CheckPriority() != -1000000 {
		t.Fatalf("linear priorities: enfo=%d check=%d", linear.EnfoPriority(), linear.CheckPriority())
	}
	if !hasPlugin(m.Conshdlrs(), "linear") {
		t.Fatal("Conshdlrs does not contain linear")
	}
	m.IncludeConshdlr("listed-conshdlr", "a listed handler", -2, -3, listedConshdlr{})
	ch, ok := m.FindConshdlr("listed-conshdlr")
	if !ok || ch.EnfoPriority() != -2 || ch.CheckPriority() != -3 || !hasPlugin(m.Conshdlrs(), "listed-conshdlr") {
		t.Fatalf("custom conshdlr: ok=%v enfo=%d check=%d", ok, ch.EnfoPriority(), ch.CheckPriority())
	}

	ev, ok := m.FindEventhdlr("softtimelimit")
	if !ok || ev.Name() != "softtimelimit" || !hasPlugin(m.Eventhdlrs(), "softtimelimit") {
		t.Fatalf("softtimelimit: ok=%v name=%q", ok, ev.Name())
	}
	// The linear constraint handler installs its own event handler of the
	// same name; the two lookups must not be the same object.
	linEv, ok := m.FindEventhdlr("linear")
	if !ok || linEv.Inner() == nil || linEv.Name() != "linear" {
		t.Fatal("linear event handler not found")
	}
	m.IncludeEventhdlr("listed-event", "a listed handler", listedEventhdlr{})
	if found, ok := m.FindEventhdlr("listed-event"); !ok || found.Name() != "listed-event" || !hasPlugin(m.Eventhdlrs(), "listed-event") {
		t.Fatal("custom event handler not listed")
	}
	if _, ok := m.FindEventhdlr("no-such-event"); ok {
		t.Fatal("found nonexistent event handler")
	}

	prop, ok := m.FindPropagator("probing")
	if !ok || prop.Name() != "probing" || prop.Desc() != "probing propagator on binary variables" {
		t.Fatalf("probing: ok=%v name=%q desc=%q", ok, prop.Name(), prop.Desc())
	}
	if prop.Priority() != -100000 || prop.Freq() != -1 || prop.Timing() != PropTimingAfterLpLoop {
		t.Fatalf("probing settings: prio=%d freq=%d timing=%#x", prop.Priority(), prop.Freq(), prop.Timing())
	}
	if prop.NCalls() != 0 || prop.NCutoffs() != 0 || prop.NDomredsFound() != 0 || prop.Time() != 0 {
		t.Fatalf("probing stats: calls=%d cutoffs=%d domreds=%d time=%v", prop.NCalls(), prop.NCutoffs(), prop.NDomredsFound(), prop.Time())
	}
	if !hasPlugin(m.Propagators(), "probing") {
		t.Fatal("Propagators does not contain probing")
	}
	if _, ok := m.FindPropagator("no-such-prop"); ok {
		t.Fatal("found nonexistent propagator")
	}

	expectErrorPanic(t, "zero branch rule", RetcodeInvalidData, func() { (BranchRulePlugin{}).Name() })
	expectErrorPanic(t, "zero propagator", RetcodeInvalidData, func() { (PropagatorPlugin{}).Priority() })
	expectErrorPanic(t, "zero conshdlr priority", RetcodeInvalidData, func() { (ConshdlrPlugin{}).EnfoPriority() })

	m.Free()
	for name, f := range map[string]func() error{
		"SetBranchRulePriority":     func() error { return m.TrySetBranchRulePriority(rule, 1) },
		"SetNodeselStdPriority":     func() error { return m.TrySetNodeselStdPriority(bfs, 1) },
		"SetNodeselMemSavePriority": func() error { return m.TrySetNodeselMemSavePriority(bfs, 1) },
	} {
		if err := f(); !errors.Is(err, RetcodeInvalidCall) {
			t.Errorf("%s on freed model: %v", name, err)
		}
	}
}

func hasPlugin[T interface{ Name() string }](plugins []T, name string) bool {
	for _, p := range plugins {
		if p.Name() == name {
			return true
		}
	}
	return false
}

type listedBranchRule struct{}

func (listedBranchRule) Execute(Model, BranchRulePlugin, []BranchingCandidate) BranchingResult {
	return BranchingResult{Kind: BranchingResultDidNotRun}
}

type listedNodesel struct{}

func (listedNodesel) Select(Model) *Node { return nil }

func (listedNodesel) Comp(Node, Node) int { return 0 }

type listedPricer struct{}

func (listedPricer) GenerateColumns(Model, PricerPlugin, bool) PricerResult {
	return PricerResult{State: PricerResultStateDidNotRun}
}

type listedConshdlr struct{}

func (listedConshdlr) Check(Model, ConshdlrPlugin, Solution) bool { return true }

func (listedConshdlr) Enforce(Model, ConshdlrPlugin) ConshdlrResult {
	return ConshdlrResultFeasible
}

type listedEventhdlr struct{}

func (listedEventhdlr) GetEventMask() EventMask { return EventMaskBestSolFound }

func (listedEventhdlr) Execute(Model, EventhdlrPlugin, Event) {}
