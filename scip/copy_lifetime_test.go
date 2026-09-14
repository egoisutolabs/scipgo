package scip

import (
	"errors"
	"testing"
)

// Simulate a reused address at the registry boundary: save a real copy's
// metadata, destroy that native instance, then leave the saved metadata under
// a fresh target's address. This is deterministic even when malloc chooses a
// different address, and exercises real SCIPcopyPlugins and SCIPreleaseRow.
func TestSameRootCopyRenewsStaleNativeIdentity(t *testing.T) {
	source := NewModel().HideOutput().IncludeDefaultPlugins()
	defer source.Free()
	old := NewModel().HideOutput()
	defer old.Free()
	if _, err := source.scip.copyPluginsTo(old.scip); err != nil {
		t.Fatal(err)
	}
	copyParents.Lock()
	previous := copyParents.m[old.scip.raw]
	copyParents.Unlock()
	if previous.inc == 0 || previous.marker == "" {
		t.Fatal("copy has no native identity marker")
	}
	old = mustRead(t, old, testFile("simple.mps")).Solve()
	stale := NewRow().Name("old_copy_row").AddTo(Model{scip: weakScip(old.scip.raw)})
	rowOwners.Lock()
	previousOwner := rowOwners.rows[stale.raw]
	rowOwners.Unlock()
	old.Free()

	target := NewModel().HideOutput()
	defer target.Free()
	// If an assertion fails before renewal, never let cleanup send the
	// deliberately stale pointer into SCIPreleaseRow.
	defer func() {
		rowOwners.Lock()
		if owner, ok := rowOwners.rows[stale.raw]; ok && owner.inc == previousOwner.inc {
			delete(rowOwners.rows, stale.raw)
		}
		rowOwners.Unlock()
	}()
	copyParents.Lock()
	copyParents.m[target.scip.raw] = previous
	copyParents.Unlock()
	previousOwner.scip = target.scip.raw
	rowOwners.Lock()
	rowOwners.rows[stale.raw] = previousOwner
	rowOwners.Unlock()
	escaped := weakScip(target.scip.raw).newRow(stale.raw)
	escaped.inc = stale.inc

	before := rowCount()
	if _, err := source.scip.copyPluginsTo(target.scip); err != nil {
		t.Fatal(err)
	}
	copyParents.Lock()
	current := copyParents.m[target.scip.raw]
	copyParents.Unlock()
	if current.root != previous.root || current.inc <= previous.inc || current.marker == previous.marker {
		t.Fatalf("same-root replacement did not renew identity: before=%+v after=%+v", previous, current)
	}
	rowOwners.Lock()
	_, staleStillOwned := rowOwners.rows[stale.raw]
	rowOwners.Unlock()
	if staleStillOwned || rowCount() != before {
		t.Fatal("stale row must be discarded during registration, without a native release")
	}
	expectErrorPanic(t, "escaped row after same-root replacement", RetcodeInvalidCall, func() { escaped.Inner() })
	if !hasCopyMarker(target.scip.raw, current.marker) {
		t.Fatal("new identity marker missing from native target")
	}

	target = mustRead(t, target, testFile("simple.mps")).Solve()
	callback := Model{scip: weakScip(target.scip.raw)}
	row := NewRow().Name("current_copy_row").AddTo(callback)
	// Another plugin registering in this same live native instance must not
	// renew its incarnation or drop the row just created by a sibling.
	if err := setCopyParent(target.scip.raw, source.scip.raw); err != nil {
		t.Fatal(err)
	}
	rowOwners.Lock()
	owner, present := rowOwners.rows[row.raw]
	rowOwners.Unlock()
	if !present || owner.copyInc != current.inc || copyIncarnation(target.scip.raw) != current.inc {
		t.Fatal("sibling registration changed the live copy's row ownership")
	}
	otherRoot := NewModel().HideOutput()
	defer otherRoot.Free()
	if err := setCopyParent(target.scip.raw, otherRoot.scip.raw); !errors.Is(err, RetcodeInvalidData) {
		t.Fatalf("reparenting a live native copy: %v, want InvalidData", err)
	}
	if copyIncarnation(target.scip.raw) != current.inc {
		t.Fatal("rejected reparenting changed the copy identity")
	}
	if row.Inner() != row.raw {
		t.Fatal("live copy row returned the wrong pointer")
	}

	before = rowCount()
	copyDies(target.scip.raw)
	if rowCount() != before+1 {
		t.Fatal("copy teardown did not release its current incarnation's row")
	}
	if err := row.deadRowErr("test"); err != nil {
		t.Fatal("teardown should purge the row tombstone; instance liveness must protect Inner")
	}
	expectErrorPanic(t, "escaped row after copy teardown", RetcodeInvalidCall, func() { row.Inner() })
}

func TestNativeCopyMarkerIsNotCopied(t *testing.T) {
	source := NewModel().HideOutput().IncludeDefaultPlugins()
	defer source.Free()
	child := NewModel().HideOutput()
	defer child.Free()
	if _, err := source.scip.copyPluginsTo(child.scip); err != nil {
		t.Fatal(err)
	}
	copyParents.Lock()
	childIdentity := copyParents.m[child.scip.raw]
	copyParents.Unlock()

	grandchild := NewModel().HideOutput()
	defer grandchild.Free()
	if _, err := child.scip.copyPluginsTo(grandchild.scip); err != nil {
		t.Fatal(err)
	}
	copyParents.Lock()
	grandchildIdentity := copyParents.m[grandchild.scip.raw]
	copyParents.Unlock()
	if grandchildIdentity.inc == 0 || grandchildIdentity.inc == childIdentity.inc || grandchildIdentity.root != source.scip.raw {
		t.Fatalf("nested copy has wrong identity: %+v", grandchildIdentity)
	}
	if hasCopyMarker(grandchild.scip.raw, childIdentity.marker) {
		t.Fatal("native marker was inherited from the source instance")
	}
	if !hasCopyMarker(grandchild.scip.raw, grandchildIdentity.marker) {
		t.Fatal("nested copy has no marker of its own")
	}
}

func TestCopyMarkerInstallFailurePreservesOwnership(t *testing.T) {
	source := NewModel().HideOutput()
	defer source.Free()
	target := mustRead(t, NewModel().HideOutput().IncludeDefaultPlugins(), testFile("simple.mps")).Solve()
	defer target.Free()
	row := NewRow().Name("still_owned").AddTo(target)
	// No marker may be installed in the Solved stage. The failed registration
	// must leave both the existing ownership entry and the native row alone.
	if err := setCopyParent(target.scip.raw, source.scip.raw); !errors.Is(err, RetcodeInvalidCall) {
		t.Fatalf("registration in Solved: %v, want InvalidCall", err)
	}
	if copyIncarnation(target.scip.raw) != 0 {
		t.Fatal("failed native registration published a copy incarnation")
	}
	rowOwners.Lock()
	_, present := rowOwners.rows[row.raw]
	rowOwners.Unlock()
	if !present || row.Name() != "still_owned" {
		t.Fatal("failed registration discarded a live row")
	}
	row.Release()
}
