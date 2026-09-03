package scip

import (
	"sync/atomic"
	"testing"
)

type copyCountingRule struct {
	copies, execs *atomic.Int32
	sawData       *atomic.Bool
}

func (r copyCountingRule) Copy() any { r.copies.Add(1); return r }

func (r copyCountingRule) Execute(model Model, _ SCIPBranchRule, cands []BranchingCandidate) BranchingResult {
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
