package scip

// The SCIP-prefixed wrapper names shipped in v0.2.0 were renamed with a
// Plugin suffix in v0.2.1. These aliases keep v0.2.0 code compiling; they
// will be removed in v1.0.0.

// Deprecated: use BranchRulePlugin.
type SCIPBranchRule = BranchRulePlugin

// Deprecated: use ConshdlrPlugin.
type SCIPConshdlr = ConshdlrPlugin

// Deprecated: use EventhdlrPlugin.
type SCIPEventhdlr = EventhdlrPlugin

// Deprecated: use NodeselPlugin.
type SCIPNodesel = NodeselPlugin

// Deprecated: use PricerPlugin.
type SCIPPricer = PricerPlugin

// Deprecated: use SeparatorPlugin.
type SCIPSeparator = SeparatorPlugin

// Deprecated: use HeuristicPlugin.
type Heur = HeuristicPlugin

// Deprecated: use PresolverPlugin.
type Presolver = PresolverPlugin

// The v0.3.0 rename unified each plugin family on one stem and casing.
// These aliases and wrappers keep v0.2.x code compiling; they will be
// removed in v1.0.0.

// Deprecated: use SeparatorBuilder.
type SepaBuilder = SeparatorBuilder

// Deprecated: use NewSeparator.
func NewSepa(s Separator) SeparatorBuilder { return NewSeparator(s) }

// Deprecated: use SourceSeparator.
func SourceSepa(sep SeparatorPlugin) RowSource { return SourceSeparator(sep) }

// Deprecated: use HeuristicPlugin.
type HeurPlugin = HeuristicPlugin

// Deprecated: use HeuristicBuilder.
type HeurBuilder = HeuristicBuilder

// Deprecated: use NewHeuristic.
func NewHeur(h Heuristic) HeuristicBuilder { return NewHeuristic(h) }

// Deprecated: use Nodesel.
type NodeSel = Nodesel

// Deprecated: use NodeselBuilder.
type NodeSelBuilder = NodeselBuilder

// Deprecated: use EventhdlrBuilder.
type EventHdlrBuilder = EventhdlrBuilder

// Deprecated: use Heuristics.
func (m Model) Heurs() []HeuristicPlugin { return m.Heuristics() }

// Deprecated: use FindHeuristic.
func (m Model) FindHeur(name string) (HeuristicPlugin, bool) { return m.FindHeuristic(name) }

// Deprecated: use TrySetHeuristicPriority.
func (m Model) TrySetHeurPriority(h HeuristicPlugin, priority int32) error {
	return m.TrySetHeuristicPriority(h, priority)
}

// Deprecated: use SetHeuristicPriority.
func (m Model) SetHeurPriority(h HeuristicPlugin, priority int32) {
	must(m.TrySetHeuristicPriority(h, priority))
}

// Deprecated: use TrySetSeparatorPriority.
func (m Model) TrySetSepaPriority(s SeparatorPlugin, priority int32) error {
	return m.TrySetSeparatorPriority(s, priority)
}

// Deprecated: use SetSeparatorPriority.
func (m Model) SetSepaPriority(s SeparatorPlugin, priority int32) {
	must(m.TrySetSeparatorPriority(s, priority))
}

// Deprecated: use TrySetPresolverPriority.
func (m Model) TrySetPresolPriority(p PresolverPlugin, priority int32) error {
	return m.TrySetPresolverPriority(p, priority)
}

// Deprecated: use SetPresolverPriority.
func (m Model) SetPresolPriority(p PresolverPlugin, priority int32) {
	must(m.TrySetPresolverPriority(p, priority))
}
