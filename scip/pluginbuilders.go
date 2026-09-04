package scip

// This file contains the builders for the plugin types, mirroring the Rust
// builder module (branchrule.rs, pricer.rs, eventhdlr.rs, heur.rs, sepa.rs,
// nodesel.rs). Each builder can be passed to Model.Add or its AddTo method
// can be called directly.

// BranchRuleBuilder is a builder for easily creating branch rules.
type BranchRuleBuilder struct {
	name         *string
	desc         *string
	priority     int32
	maxdepth     int32
	maxbounddist float64
	rule         BranchRule
}

// NewBranchRule creates a new BranchRuleBuilder wrapping the given rule.
//
// Defaults: empty name/desc, priority 100000, maxdepth -1 (unlimited),
// maxbounddist 1.0 (all nodes).
func NewBranchRule(rule BranchRule) BranchRuleBuilder {
	return BranchRuleBuilder{priority: 100000, maxdepth: -1, maxbounddist: 1.0, rule: rule}
}

// Name sets the name of the branch rule.
func (b BranchRuleBuilder) Name(name string) BranchRuleBuilder { b.name = &name; return b }

// Desc sets the description of the branch rule.
func (b BranchRuleBuilder) Desc(desc string) BranchRuleBuilder { b.desc = &desc; return b }

// Priority sets the priority of the branch rule.
func (b BranchRuleBuilder) Priority(p int32) BranchRuleBuilder { b.priority = p; return b }

// MaxDepth sets the maximum depth level up to which this branch rule should
// be used; -1 means any depth.
func (b BranchRuleBuilder) MaxDepth(d int32) BranchRuleBuilder { b.maxdepth = d; return b }

// MaxBoundDist sets the maximum relative distance from the current node's
// dual bound to primal bound compared to the best node's dual bound for
// applying the branch rule.
func (b BranchRuleBuilder) MaxBoundDist(d float64) BranchRuleBuilder { b.maxbounddist = d; return b }

// AddTo includes the branch rule in a model in the ProblemCreated stage.
func (b BranchRuleBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the branch rule, returning an error on failure.
func (b BranchRuleBuilder) TryAddTo(m Model) error {
	return m.TryIncludeBranchRule(strOrEmpty(b.name), strOrEmpty(b.desc), b.priority, b.maxdepth, b.maxbounddist, b.rule)
}

// PricerBuilder is a builder for easily creating pricers.
type PricerBuilder struct {
	name     *string
	desc     *string
	priority int32
	delay    bool
	pricer   Pricer
}

// NewPricer creates a new PricerBuilder wrapping the given pricer.
//
// Defaults: empty name/desc, priority 100000, delay false.
func NewPricer(p Pricer) PricerBuilder {
	return PricerBuilder{priority: 100000, pricer: p}
}

// Name sets the name of the pricer.
func (b PricerBuilder) Name(name string) PricerBuilder { b.name = &name; return b }

// Desc sets the description of the pricer.
func (b PricerBuilder) Desc(desc string) PricerBuilder { b.desc = &desc; return b }

// Priority sets the priority of the pricer.
func (b PricerBuilder) Priority(p int32) PricerBuilder { b.priority = p; return b }

// Delay sets whether the pricer should be delayed.
func (b PricerBuilder) Delay(d bool) PricerBuilder { b.delay = d; return b }

// AddTo includes the pricer in a model in the ProblemCreated stage.
func (b PricerBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the pricer, returning an error on failure.
func (b PricerBuilder) TryAddTo(m Model) error {
	return m.TryIncludePricer(strOrEmpty(b.name), strOrEmpty(b.desc), b.priority, b.delay, b.pricer)
}

// EventHdlrBuilder is a builder for easily creating event handlers.
type EventHdlrBuilder struct {
	name      *string
	desc      *string
	eventhdlr Eventhdlr
}

// NewEventhdlr creates a new EventHdlrBuilder wrapping the given handler.
func NewEventhdlr(e Eventhdlr) EventHdlrBuilder { return EventHdlrBuilder{eventhdlr: e} }

// Name sets the name of the event handler.
func (b EventHdlrBuilder) Name(name string) EventHdlrBuilder { b.name = &name; return b }

// Desc sets the description of the event handler.
func (b EventHdlrBuilder) Desc(desc string) EventHdlrBuilder { b.desc = &desc; return b }

// AddTo includes the event handler in a model in the ProblemCreated stage.
func (b EventHdlrBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the event handler, returning an error on failure.
func (b EventHdlrBuilder) TryAddTo(m Model) error {
	return m.TryIncludeEventhdlr(strOrEmpty(b.name), strOrEmpty(b.desc), b.eventhdlr)
}

// HeurBuilder is a builder for easily creating primal heuristics.
type HeurBuilder struct {
	name        *string
	desc        *string
	priority    int32
	dispchar    *byte
	freq        int32
	freqofs     int32
	maxdepth    int32
	timing      *HeurTiming
	usessubscip bool
	heur        Heuristic
}

// NewHeur creates a new HeurBuilder wrapping the given heuristic.
//
// Defaults: empty name/desc, dispchar '?', timing BEFORE_NODE, priority
// 100000, freq 1, freqofs 0, maxdepth -1, usessubscip false.
func NewHeur(h Heuristic) HeurBuilder {
	return HeurBuilder{priority: 100000, freq: 1, freqofs: 0, maxdepth: -1, heur: h}
}

// Name sets the name of the heuristic.
func (b HeurBuilder) Name(name string) HeurBuilder { b.name = &name; return b }

// Desc sets the description of the heuristic.
func (b HeurBuilder) Desc(desc string) HeurBuilder { b.desc = &desc; return b }

// Priority sets the priority of the heuristic.
func (b HeurBuilder) Priority(p int32) HeurBuilder { b.priority = p; return b }

// DispChar sets the display character of the heuristic.
func (b HeurBuilder) DispChar(c byte) HeurBuilder { b.dispchar = &c; return b }

// Freq sets the frequency for calling the heuristic.
func (b HeurBuilder) Freq(f int32) HeurBuilder { b.freq = f; return b }

// FreqOfs sets the frequency offset for calling the heuristic.
func (b HeurBuilder) FreqOfs(f int32) HeurBuilder { b.freqofs = f; return b }

// MaxDepth sets the maximum depth up to which the heuristic is used.
func (b HeurBuilder) MaxDepth(d int32) HeurBuilder { b.maxdepth = d; return b }

// Timing sets the timing mask of the heuristic.
func (b HeurBuilder) Timing(t HeurTiming) HeurBuilder { b.timing = &t; return b }

// UsesSubscip sets whether the heuristic should use a secondary SCIP instance.
func (b HeurBuilder) UsesSubscip(v bool) HeurBuilder { b.usessubscip = v; return b }

// AddTo includes the heuristic in a model in the ProblemCreated stage.
func (b HeurBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the heuristic, returning an error on failure.
func (b HeurBuilder) TryAddTo(m Model) error {
	dispchar := byte('?')
	if b.dispchar != nil {
		dispchar = *b.dispchar
	}
	timing := HeurTimingBeforeNode
	if b.timing != nil {
		timing = *b.timing
	}
	return m.TryIncludeHeur(strOrEmpty(b.name), strOrEmpty(b.desc), b.priority, dispchar,
		b.freq, b.freqofs, b.maxdepth, timing, b.usessubscip, b.heur)
}

// SepaBuilder is a builder for easily creating separators.
type SepaBuilder struct {
	name         *string
	desc         *string
	priority     int32
	freq         int32
	maxbounddist float64
	usesubscip   bool
	delay        bool
	sepa         Separator
}

// NewSepa creates a new SepaBuilder wrapping the given separator.
//
// Defaults: empty name/desc, priority 100000, freq 1, maxbounddist 1.0,
// usesubscip false, delay false.
func NewSepa(s Separator) SepaBuilder {
	return SepaBuilder{priority: 100000, freq: 1, maxbounddist: 1.0, sepa: s}
}

// Name sets the name of the separator.
func (b SepaBuilder) Name(name string) SepaBuilder { b.name = &name; return b }

// Desc sets the description of the separator.
func (b SepaBuilder) Desc(desc string) SepaBuilder { b.desc = &desc; return b }

// Priority sets the priority of the separator.
func (b SepaBuilder) Priority(p int32) SepaBuilder { b.priority = p; return b }

// Freq sets the frequency of the separator: 1 at every node, 2 every other
// node, -1 turns it off.
func (b SepaBuilder) Freq(f int32) SepaBuilder { b.freq = f; return b }

// MaxBoundDist sets the maximum relative distance from the current node's
// dual bound to primal bound compared to the best node's dual bound for
// applying the separator.
func (b SepaBuilder) MaxBoundDist(d float64) SepaBuilder { b.maxbounddist = d; return b }

// UsesSubscip sets whether the separator uses a secondary SCIP instance.
func (b SepaBuilder) UsesSubscip(v bool) SepaBuilder { b.usesubscip = v; return b }

// Delay sets whether the separator should be delayed.
func (b SepaBuilder) Delay(v bool) SepaBuilder { b.delay = v; return b }

// AddTo includes the separator in a model in the ProblemCreated stage.
func (b SepaBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the separator, returning an error on failure.
func (b SepaBuilder) TryAddTo(m Model) error {
	return m.TryIncludeSeparator(strOrEmpty(b.name), strOrEmpty(b.desc), b.priority, b.freq,
		b.maxbounddist, b.usesubscip, b.delay, b.sepa)
}

// NodeSelBuilder is a builder for easily creating node selectors.
type NodeSelBuilder struct {
	name            *string
	desc            *string
	stdPriority     int32
	memSavePriority int32
	nodesel         NodeSel
}

// NewNodesel creates a new NodeSelBuilder wrapping the given node selector.
//
// Defaults: empty name/desc, priorities 1000000 (higher than all default
// SCIP node selectors, so the custom one is used).
func NewNodesel(n NodeSel) NodeSelBuilder {
	return NodeSelBuilder{stdPriority: 1000000, memSavePriority: 1000000, nodesel: n}
}

// Name sets the name of the node selector.
func (b NodeSelBuilder) Name(name string) NodeSelBuilder { b.name = &name; return b }

// Desc sets the description of the node selector.
func (b NodeSelBuilder) Desc(desc string) NodeSelBuilder { b.desc = &desc; return b }

// StdPriority sets the standard priority of the node selector.
func (b NodeSelBuilder) StdPriority(p int32) NodeSelBuilder { b.stdPriority = p; return b }

// MemSavePriority sets the memory saving priority of the node selector.
func (b NodeSelBuilder) MemSavePriority(p int32) NodeSelBuilder { b.memSavePriority = p; return b }

// AddTo includes the node selector in a model in the ProblemCreated stage.
func (b NodeSelBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the node selector, returning an error on failure.
func (b NodeSelBuilder) TryAddTo(m Model) error {
	return m.TryIncludeNodesel(strOrEmpty(b.name), strOrEmpty(b.desc), b.stdPriority, b.memSavePriority, b.nodesel)
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
