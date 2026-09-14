package scip

import (
	"errors"
	"testing"
)

// The tests in this file pin the row-capture ownership rules of #25: the
// binding releases its create capture after a successful add, releases
// never-added rows at FreeTransform or free (and at a copy's death), and
// never releases rows obtained from queries.

func resetRowCount(t *testing.T) {
	t.Helper()
	rowOwners.Lock()
	rowsReleasedForTest = 0
	rowOwners.Unlock()
}

func rowCount() int {
	rowOwners.Lock()
	defer rowOwners.Unlock()
	return rowsReleasedForTest
}

func ownedRows() int {
	rowOwners.Lock()
	defer rowOwners.Unlock()
	return len(rowOwners.rows)
}

// countingSeparator creates one row per Execute and adds it as a cut.
type countingSeparator struct {
	added *int
}

func (s countingSeparator) ExecuteLP(model Model, _ SeparatorPlugin) SeparationResult {
	row := NewRow().Name("probe_cut").Local(false).AddTo(model)
	model.AddCut(row, false)
	*s.added++
	return SeparationResultSeparated
}

func TestRowsReleasedAfterAdd(t *testing.T) {
	resetRowCount(t)

	added := 0
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("p0201.mps"))
	defer model.Free()
	model.Add(NewSeparator(countingSeparator{added: &added}).Name("counting_sepa").Freq(1))
	model.Solve()

	if added == 0 {
		t.Fatal("separator never ran; the test proves nothing")
	}
	if got := rowCount(); got != added {
		t.Fatalf("released %d row captures for %d added cuts", got, added)
	}
	if got := ownedRows(); got != 0 {
		t.Fatalf("%d rows still owned after the solve", got)
	}
}

func TestUnaddedRowReleasedAtFreeTransform(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.mps"))
	defer model.Free()
	model.Solve()
	if _, err := NewRow().Name("never_added").TryAddTo(model); err != nil {
		t.Fatalf("create row: %v", err)
	}
	resetRowCount(t)
	model.FreeTransform()
	if got := rowCount(); got != 1 {
		t.Fatalf("FreeTransform released %d rows, want 1", got)
	}
}

func TestUnaddedRowReleasedAtFree(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.mps"))
	model.Solve()
	if _, err := NewRow().Name("never_added").TryAddTo(model); err != nil {
		t.Fatalf("create row: %v", err)
	}
	resetRowCount(t)
	model.Free()
	if got := rowCount(); got != 1 {
		t.Fatalf("Free released %d rows, want 1", got)
	}
}

func TestRowRelease(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.mps"))
	defer model.Free()
	model.Solve()
	row := NewRow().Name("released").AddTo(model)
	resetRowCount(t)
	row.Release()
	if err := row.TryRelease(); err == nil || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("double release = %v, want RetcodeInvalidCall", err)
	}
	if got := rowCount(); got != 1 {
		t.Fatalf("Release fired %d times, want 1", got)
	}
	// The handle is dead: reads panic with InvalidCall, writes return it.
	func() {
		defer func() {
			r := recover()
			e, ok := r.(*Error)
			if !ok || !errors.Is(e, RetcodeInvalidCall) {
				t.Fatalf("Name() on released row panicked with %v", r)
			}
		}()
		_ = row.Name()
	}()
	if err := row.TrySetCoeff(model.Vars()[0], 1); err == nil || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("TrySetCoeff on released row = %v, want RetcodeInvalidCall", err)
	}
}

// TestRetainedRowUsableAfterAdd pins the counterpart of the release rules:
// a row the binding added is never tombstoned — SCIP holds its own capture
// while it uses the row, and the handle stays inspectable, exactly as the
// docs promise. (A cut SCIP filters away is freed by the add and its handle
// is undetectably dead — the #20 handle-liveness gap, documented.)
func TestRetainedRowUsableAfterAdd(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("p0201.mps"))
	defer model.Free()
	model.Solve()
	row := NewRow().Name("retained_after_add").Local(true).AddTo(model)
	for _, v := range model.Vars() {
		row.SetCoeff(v, 1)
	}
	model.AddCut(row, true) // forceCut: SCIP retains it
	if err := row.TrySetCoeff(model.Vars()[0], 2); err != nil {
		t.Fatalf("TrySetCoeff after a retained add: %v", err)
	}
	_ = row.Age() // reads still work
	if row.Inner() != row.raw {
		t.Fatal("retained row must expose its live native pointer")
	}
}

func TestRowInnerChecksLifetimeAfterTombstonePurge(t *testing.T) {
	var zero Row
	expectErrorPanic(t, "zero row", RetcodeInvalidData, func() { zero.Inner() })

	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.mps")).Solve()
	defer model.Free()
	released := NewRow().Name("released_before_transform").AddTo(model)
	if released.Inner() != released.raw {
		t.Fatal("live row returned the wrong pointer")
	}
	released.Release()
	expectErrorPanic(t, "released row", RetcodeInvalidCall, func() { released.Inner() })

	unadded := NewRow().Name("swept_at_transform").AddTo(model)
	// Query wrappers carry no allocation incarnation, so only the full
	// generation/instance check can protect their Inner calls after teardown.
	query := model.scip.newRow(unadded.raw)
	model.FreeTransform()
	for name, row := range map[string]Row{"released": released, "unadded": unadded, "query": query} {
		if err := row.deadRowErr("test"); err != nil {
			t.Fatalf("%s: tombstone was not purged: %v", name, err)
		}
		expectErrorPanic(t, name+" after FreeTransform", RetcodeInvalidCall, func() { row.Inner() })
	}
	model.Solve()
	expectErrorPanic(t, "released row after re-solve", RetcodeInvalidCall, func() { released.Inner() })

	freed := NewRow().Name("swept_at_free").AddTo(model)
	model.Free()
	if err := freed.deadRowErr("test"); err != nil {
		t.Fatal("instance free should purge the row tombstone")
	}
	expectErrorPanic(t, "row after Free", RetcodeInvalidCall, func() { freed.Inner() })
}

// TestReleasedRowStaysDeadAcrossReuse checks the incarnation rule: after a
// release, a fresh row — possibly allocated at the recycled address — is
// usable, while the released handle keeps reporting InvalidCall.
func TestReleasedRowStaysDeadAcrossReuse(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.mps"))
	defer model.Free()
	model.Solve()
	a := NewRow().Name("released_a").AddTo(model)
	a.Release()
	if err := a.TrySetCoeff(model.Vars()[0], 1); err == nil || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("released handle = %v, want RetcodeInvalidCall", err)
	}
	b := NewRow().Name("fresh_b").AddTo(model) // may reuse a's address
	if err := b.TrySetCoeff(model.Vars()[0], 1); err != nil {
		t.Fatalf("fresh row at a recycled address: %v", err)
	}
	if err := a.TrySetCoeff(model.Vars()[0], 1); err == nil || !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("released handle after reuse = %v, want RetcodeInvalidCall", err)
	}
}

func TestQueryRowsNotReleasable(t *testing.T) {
	resetRowCount(t)
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.mps"))
	defer model.Free()
	model.Solve()
	// Rows of the LP reached through a query: the binding did not create
	// them (SCIP's own cuts, or rows it kept), and must not release them.
	rows := 0
	for _, v := range model.Vars() {
		col, ok := v.Col()
		if !ok {
			continue
		}
		for _, r := range col.Rows() {
			rows++
			if err := r.TryRelease(); err == nil || !errors.Is(err, RetcodeInvalidData) {
				t.Fatalf("release of query row = %v, want RetcodeInvalidData", err)
			}
		}
	}
	if rows == 0 {
		t.Skip("no LP rows reachable in this model")
	}
	if got := rowCount(); got != 0 {
		t.Fatalf("query row release fired %d times", got)
	}
}

// TestFilteredCutNotReadAfterRelease pins the P1 from review: a cut SCIP
// does not retain (forceCut=false, non-efficacious) is freed when the
// binding drops its capture, so TryAddCut must not read the row — its
// name, for the error detail — after the add.
func TestFilteredCutNotReadAfterRelease(t *testing.T) {
	model := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("p0201.mps"))
	defer model.Free()
	model.Solve()
	row := NewRow().Name("filtered_empty_cut").Local(true).AddTo(model)
	resetRowCount(t)
	model.AddCut(row, false)
	if got := rowCount(); got != 1 {
		t.Fatalf("binding capture released %d times, want 1", got)
	}
	if got := ownedRows(); got != 0 {
		t.Fatalf("%d rows still owned", got)
	}
}
