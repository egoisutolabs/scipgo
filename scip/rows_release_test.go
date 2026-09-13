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
	if err := row.TryRelease(); err == nil || !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("double release = %v, want RetcodeInvalidData", err)
	}
	if got := rowCount(); got != 1 {
		t.Fatalf("Release fired %d times, want 1", got)
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
