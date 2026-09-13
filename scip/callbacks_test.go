package scip

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

type copyCountingRule struct {
	copies, execs *atomic.Int32
	sawData       *atomic.Bool
}

func (r copyCountingRule) Copy() any { r.copies.Add(1); return r }

func (r copyCountingRule) Execute(model Model, _ BranchRulePlugin, cands []BranchingCandidate) BranchingResult {
	r.execs.Add(1)
	if v, ok := GetData[int](model); ok && v == 42 {
		r.sawData.Store(true)
	}
	return BranchOn(cands[0])
}

type copyableConshdlr struct{ countingConshdlr }

func (c *copyableConshdlr) Copy() any { return c }

func TestCopyablePlugins(t *testing.T) {
	rule := copyCountingRule{new(atomic.Int32), new(atomic.Int32), new(atomic.Bool)}
	source := NewModel().HideOutput().IncludeDefaultPlugins()
	source.IncludeBranchRule("copied", "", 1000000, -1, 1, rule)
	source.IncludeConshdlr("copiedcons", "", -1, -1, &copyableConshdlr{})
	source.IncludeConshdlr("plaincons", "", -1, -1, &countingConshdlr{}) // not Copyable
	SetData(source, 42)

	target := NewModel().HideOutput()
	valid, err := source.scip.copyPluginsTo(target.scip)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("copy reported valid although a non-Copyable conshdlr was skipped")
	}
	if rule.copies.Load() != 1 {
		t.Fatalf("Copy called %d times, want 1", rule.copies.Load())
	}

	// The copied rule runs in the target and sees the source's datastore.
	target = mustRead(t, target, testFile("gen-ip054.mps")) // readers were copied too
	target, _ = target.SetLongintParam("limits/nodes", 30)
	target.Solve()
	if rule.execs.Load() == 0 {
		t.Fatal("copied branch rule never executed in target")
	}
	if !rule.sawData.Load() {
		t.Fatal("copied rule did not see source datastore")
	}
	target.Free()
	source.Free()
}

func TestSolveConcurrentCopiesPlugins(t *testing.T) {
	rule := copyCountingRule{new(atomic.Int32), new(atomic.Int32), new(atomic.Bool)}
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
	model.IncludeBranchRule("copied", "", 1000000, -1, 1, rule)
	model, _ = model.SetIntParam("parallel/maxnthreads", 2)
	model, _ = model.SetLongintParam("limits/nodes", 30)
	model.SolveConcurrent()
	t.Logf("copies=%d execs=%d", rule.copies.Load(), rule.execs.Load())
	if rule.copies.Load() == 0 {
		t.Fatal("plugin was not copied into the concurrent workers")
	}
}

// ------------------------------------------------- cached callback wrappers

// identityHdlr records which *Scip wrapper every callback received.
type identityHdlr struct {
	seen   map[*Scip]int
	events int
}

func (h *identityHdlr) GetEventMask() EventMask { return EventMaskNodeFocused }

func (h *identityHdlr) Execute(model Model, _ EventhdlrPlugin, _ Event) {
	if h.seen == nil {
		h.seen = make(map[*Scip]int)
	}
	h.seen[model.scip]++
	h.events++
}

// Every callback for one plugin must receive the same cached wrapper, across
// solves too: the registry entry survives FreeTransform, so a re-solve must
// not mint a new one.
func TestCallbackWrapperCachedPerPlugin(t *testing.T) {
	h := &identityHdlr{}
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
	model.IncludeEventhdlr("identity", "", h)
	model, err := model.SetLongintParam("limits/nodes", 30)
	if err != nil {
		t.Fatal(err)
	}
	model.Solve()
	model = model.FreeTransform()
	model.Solve()

	if h.events == 0 {
		t.Fatal("handler never ran")
	}
	if len(h.seen) != 1 {
		t.Fatalf("%d distinct wrappers handed to callbacks, want 1", len(h.seen))
	}
}

// Two models must never share a cached wrapper.
func TestCallbackWrapperPerModel(t *testing.T) {
	h1, h2 := &identityHdlr{}, &identityHdlr{}
	for _, h := range []*identityHdlr{h1, h2} {
		model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
		model.IncludeEventhdlr("identity", "", h)
		model, err := model.SetLongintParam("limits/nodes", 30)
		if err != nil {
			t.Fatal(err)
		}
		model.Solve()
		if len(h.seen) != 1 {
			t.Fatalf("%d distinct wrappers, want 1", len(h.seen))
		}
	}
	var p1, p2 *Scip
	for p := range h1.seen {
		p1 = p
	}
	for p := range h2.seen {
		p2 = p
	}
	if p1 == p2 {
		t.Fatal("two models shared one callback wrapper")
	}
}

// copyIdentityHdlr is copied into concurrent workers; each copy checks that
// it always sees the same wrapper and reports the wrapper it saw so copies
// can be told apart.
type copyIdentityHdlr struct {
	wrappers *copyWrapperSet
	first    *Scip
}

type copyWrapperSet struct {
	mu       sync.Mutex
	seen     map[*Scip]bool
	mismatch atomic.Bool
}

func (h *copyIdentityHdlr) GetEventMask() EventMask { return EventMaskNodeFocused }

func (h *copyIdentityHdlr) Execute(model Model, _ EventhdlrPlugin, _ Event) {
	if h.first == nil {
		h.first = model.scip
		h.wrappers.mu.Lock()
		h.wrappers.seen[model.scip] = true
		h.wrappers.mu.Unlock()
		return
	}
	if h.first != model.scip {
		h.wrappers.mismatch.Store(true)
	}
}

func (h *copyIdentityHdlr) Copy() any { return &copyIdentityHdlr{wrappers: h.wrappers} }

// Each concurrent worker is its own instance with its own registry entry, so
// each copy of a Copyable handler must get its own stable wrapper — never
// the main instance's, never another worker's.
func TestCallbackWrapperPerConcurrentWorker(t *testing.T) {
	set := &copyWrapperSet{seen: make(map[*Scip]bool)}
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
	model.IncludeEventhdlr("copyidentity", "", &copyIdentityHdlr{wrappers: set})
	model, err := model.SetIntParam("parallel/maxnthreads", 2)
	if err != nil {
		t.Fatal(err)
	}
	model, err = model.SetLongintParam("limits/nodes", 30)
	if err != nil {
		t.Fatal(err)
	}
	model.SolveConcurrent()

	set.mu.Lock()
	wrapped := len(set.seen)
	set.mu.Unlock()
	if wrapped == 0 {
		t.Fatal("no worker callbacks ran")
	}
	if set.mismatch.Load() {
		t.Fatal("one copy saw more than one wrapper")
	}
}

// allocCountingHdlr tracks the Go-heap allocations between its callbacks.
type allocCountingHdlr struct {
	events  int
	samples int
	prev    uint64
	minD    uint64
}

func (h *allocCountingHdlr) GetEventMask() EventMask { return EventMaskNodeFocused }

func (h *allocCountingHdlr) Execute(model Model, _ EventhdlrPlugin, _ Event) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	if h.events > 0 {
		// The minimum is tracked with a separate sample count: a real zero
		// must not be treated as "no sample yet" and overwritten by a later,
		// noisier interval.
		if d := ms.Mallocs - h.prev; h.samples == 0 || d < h.minD {
			h.minD = d
		}
		h.samples++
	}
	h.prev = ms.Mallocs
	h.events++
}

// A callback must not allocate on the Go heap: the wrapper is cached in the
// registry entry, not minted per call. The binding's cost is the minimum
// allocation delta between consecutive callbacks, because the process-wide
// counter also picks up unrelated background allocation — finalizers, test
// goroutines — which is additive and cannot hide a real per-callback cost.
// Heuristics are off so LNS sub-SCIPs do not copy plugins mid-solve.
func TestEventhdlrCallbacksDoNotAllocate(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
	model = model.SetHeuristics(ParamSettingOff)
	model, err := model.SetLongintParam("limits/nodes", 50)
	if err != nil {
		t.Fatal(err)
	}
	h := &allocCountingHdlr{}
	model.IncludeEventhdlr("alloccount", "", h)
	model.Solve()

	if h.samples < 1 {
		t.Fatalf("only %d callbacks, need 2 to compare", h.events)
	}
	if h.minD != 0 {
		t.Fatalf("best-case callback allocated %d heap objects, want 0", h.minD)
	}
}

// BenchmarkEventhdlrExec measures the binding's cost per event callback; the
// zero-allocation claim itself is asserted by TestEventhdlrCallbacksDoNotAllocate.
func BenchmarkEventhdlrExec(b *testing.B) {
	model := NewModel().HideOutput().IncludeDefaultPlugins()
	model, err := model.ReadProb(testFile("gen-ip054.mps"))
	if err != nil {
		b.Fatal(err)
	}
	model = model.SetHeuristics(ParamSettingOff)
	model, err = model.SetLongintParam("limits/nodes", 200)
	if err != nil {
		b.Fatal(err)
	}
	h := &countingHdlr{}
	model.IncludeEventhdlr("benchcount", "", h)

	model.Solve() // warm up
	model = model.FreeTransform()
	warm := h.events

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.Solve()
		model = model.FreeTransform()
	}
	b.StopTimer()
	if h.events == warm {
		b.Fatal("handler never ran")
	}
	b.ReportMetric(float64(h.events-warm)/float64(b.N), "callbacks/op")
}

// countingHdlr counts events without allocating.
type countingHdlr struct{ events int }

func (h *countingHdlr) GetEventMask() EventMask { return EventMaskNodeFocused }
func (h *countingHdlr) Execute(model Model, _ EventhdlrPlugin, _ Event) {
	h.events++
}

// stageProbeHdlr calls a guard-checked Model method (Status) from its
// callback; with sibling plugin copies churning the copy incarnation, a
// wrapper with a stale incarnation reports a freed model here.
type stageProbeHdlr struct{ calls atomic.Int32 }

func (h *stageProbeHdlr) GetEventMask() EventMask { return EventMaskNodeFocused }

func (h *stageProbeHdlr) Execute(model Model, _ EventhdlrPlugin, _ Event) {
	model.Status() // guard-checked: a stale incarnation wrapper reports a freed model here
	h.calls.Add(1)
}

func (h *stageProbeHdlr) Copy() any { return h }

// A sub-SCIP receiving several Copyable plugins must keep one incarnation
// for all of them: each Go*Copy used to re-register the target, so a wrapper
// cached at the first plugin's include went stale the moment the second
// plugin was copied, and that plugin's callbacks saw a freed model.
func TestCopiedPluginsShareOneIncarnation(t *testing.T) {
	first, second := &stageProbeHdlr{}, &stageProbeHdlr{}
	source := NewModel().HideOutput().IncludeDefaultPlugins()
	source.IncludeEventhdlr("first", "", first)
	source.IncludeEventhdlr("second", "", second)

	target := NewModel().HideOutput()
	if _, err := source.scip.copyPluginsTo(target.scip); err != nil {
		t.Fatal(err)
	}
	target = mustRead(t, target, testFile("gen-ip054.mps"))
	target, err := target.SetLongintParam("limits/nodes", 30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.TrySolve(); err != nil {
		t.Fatalf("solve with copied handlers: %v", err)
	}
	if first.calls.Load() == 0 || second.calls.Load() == 0 {
		t.Fatalf("copied handlers ran %d and %d times, want both > 0",
			first.calls.Load(), second.calls.Load())
	}
}
