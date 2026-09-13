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

// EventhdlrBuilder is a builder for easily creating event handlers.
type EventhdlrBuilder struct {
	name      *string
	desc      *string
	eventhdlr Eventhdlr
}

// NewEventhdlr creates a new EventhdlrBuilder wrapping the given handler.
func NewEventhdlr(e Eventhdlr) EventhdlrBuilder { return EventhdlrBuilder{eventhdlr: e} }

// Name sets the name of the event handler.
func (b EventhdlrBuilder) Name(name string) EventhdlrBuilder { b.name = &name; return b }

// Desc sets the description of the event handler.
func (b EventhdlrBuilder) Desc(desc string) EventhdlrBuilder { b.desc = &desc; return b }

// AddTo includes the event handler in a model in the ProblemCreated stage.
func (b EventhdlrBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the event handler, returning an error on failure.
func (b EventhdlrBuilder) TryAddTo(m Model) error {
	return m.TryIncludeEventhdlr(strOrEmpty(b.name), strOrEmpty(b.desc), b.eventhdlr)
}

// HeuristicBuilder is a builder for easily creating primal heuristics.
type HeuristicBuilder struct {
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

// NewHeuristic creates a new HeuristicBuilder wrapping the given heuristic.
//
// Defaults: empty name/desc, dispchar '?', timing BEFORE_NODE, priority
// 100000, freq 1, freqofs 0, maxdepth -1, usessubscip false.
func NewHeuristic(h Heuristic) HeuristicBuilder {
	return HeuristicBuilder{priority: 100000, freq: 1, freqofs: 0, maxdepth: -1, heur: h}
}

// Name sets the name of the heuristic.
func (b HeuristicBuilder) Name(name string) HeuristicBuilder { b.name = &name; return b }

// Desc sets the description of the heuristic.
func (b HeuristicBuilder) Desc(desc string) HeuristicBuilder { b.desc = &desc; return b }

// Priority sets the priority of the heuristic.
func (b HeuristicBuilder) Priority(p int32) HeuristicBuilder { b.priority = p; return b }

// DispChar sets the display character of the heuristic.
func (b HeuristicBuilder) DispChar(c byte) HeuristicBuilder { b.dispchar = &c; return b }

// Freq sets the frequency for calling the heuristic.
func (b HeuristicBuilder) Freq(f int32) HeuristicBuilder { b.freq = f; return b }

// FreqOfs sets the frequency offset for calling the heuristic.
func (b HeuristicBuilder) FreqOfs(f int32) HeuristicBuilder { b.freqofs = f; return b }

// MaxDepth sets the maximum depth up to which the heuristic is used.
func (b HeuristicBuilder) MaxDepth(d int32) HeuristicBuilder { b.maxdepth = d; return b }

// Timing sets the timing mask of the heuristic.
func (b HeuristicBuilder) Timing(t HeurTiming) HeuristicBuilder { b.timing = &t; return b }

// UsesSubscip sets whether the heuristic should use a secondary SCIP instance.
func (b HeuristicBuilder) UsesSubscip(v bool) HeuristicBuilder { b.usessubscip = v; return b }

// AddTo includes the heuristic in a model in the ProblemCreated stage.
func (b HeuristicBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the heuristic, returning an error on failure.
func (b HeuristicBuilder) TryAddTo(m Model) error {
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

// SeparatorBuilder is a builder for easily creating separators.
type SeparatorBuilder struct {
	name         *string
	desc         *string
	priority     int32
	freq         int32
	maxbounddist float64
	usesubscip   bool
	delay        bool
	sepa         Separator
}

// NewSeparator creates a new SeparatorBuilder wrapping the given separator.
//
// Defaults: empty name/desc, priority 100000, freq 1, maxbounddist 1.0,
// usesubscip false, delay false.
func NewSeparator(s Separator) SeparatorBuilder {
	return SeparatorBuilder{priority: 100000, freq: 1, maxbounddist: 1.0, sepa: s}
}

// Name sets the name of the separator.
func (b SeparatorBuilder) Name(name string) SeparatorBuilder { b.name = &name; return b }

// Desc sets the description of the separator.
func (b SeparatorBuilder) Desc(desc string) SeparatorBuilder { b.desc = &desc; return b }

// Priority sets the priority of the separator.
func (b SeparatorBuilder) Priority(p int32) SeparatorBuilder { b.priority = p; return b }

// Freq sets the frequency of the separator: 1 at every node, 2 every other
// node, -1 turns it off.
func (b SeparatorBuilder) Freq(f int32) SeparatorBuilder { b.freq = f; return b }

// MaxBoundDist sets the maximum relative distance from the current node's
// dual bound to primal bound compared to the best node's dual bound for
// applying the separator.
func (b SeparatorBuilder) MaxBoundDist(d float64) SeparatorBuilder { b.maxbounddist = d; return b }

// UsesSubscip sets whether the separator uses a secondary SCIP instance.
func (b SeparatorBuilder) UsesSubscip(v bool) SeparatorBuilder { b.usesubscip = v; return b }

// Delay sets whether the separator should be delayed.
func (b SeparatorBuilder) Delay(v bool) SeparatorBuilder { b.delay = v; return b }

// AddTo includes the separator in a model in the ProblemCreated stage.
func (b SeparatorBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the separator, returning an error on failure.
func (b SeparatorBuilder) TryAddTo(m Model) error {
	return m.TryIncludeSeparator(strOrEmpty(b.name), strOrEmpty(b.desc), b.priority, b.freq,
		b.maxbounddist, b.usesubscip, b.delay, b.sepa)
}

// NodeselBuilder is a builder for easily creating node selectors.
type NodeselBuilder struct {
	name            *string
	desc            *string
	stdPriority     int32
	memSavePriority int32
	nodesel         Nodesel
}

// NewNodesel creates a new NodeselBuilder wrapping the given node selector.
//
// Defaults: empty name/desc, priorities 1000000 (higher than all default
// SCIP node selectors, so the custom one is used).
func NewNodesel(n Nodesel) NodeselBuilder {
	return NodeselBuilder{stdPriority: 1000000, memSavePriority: 1000000, nodesel: n}
}

// Name sets the name of the node selector.
func (b NodeselBuilder) Name(name string) NodeselBuilder { b.name = &name; return b }

// Desc sets the description of the node selector.
func (b NodeselBuilder) Desc(desc string) NodeselBuilder { b.desc = &desc; return b }

// StdPriority sets the standard priority of the node selector.
func (b NodeselBuilder) StdPriority(p int32) NodeselBuilder { b.stdPriority = p; return b }

// MemSavePriority sets the memory saving priority of the node selector.
func (b NodeselBuilder) MemSavePriority(p int32) NodeselBuilder { b.memSavePriority = p; return b }

// AddTo includes the node selector in a model in the ProblemCreated stage.
func (b NodeselBuilder) AddTo(m Model) { must(b.TryAddTo(m)) }

// TryAddTo includes the node selector, returning an error on failure.
func (b NodeselBuilder) TryAddTo(m Model) error {
	return m.TryIncludeNodesel(strOrEmpty(b.name), strOrEmpty(b.desc), b.stdPriority, b.memSavePriority, b.nodesel)
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
