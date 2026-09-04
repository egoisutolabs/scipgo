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

// Deprecated: use HeurPlugin.
type Heur = HeurPlugin

// Deprecated: use PresolverPlugin.
type Presolver = PresolverPlugin
