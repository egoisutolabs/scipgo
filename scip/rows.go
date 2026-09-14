package scip

/*
#include "helpers.h"
*/
import "C"

import (
	"runtime"
	"sync"
)

// rowOwners tracks the rows the binding created and still holds a SCIP
// capture on, keyed by the row and valued by the owning raw SCIP instance.
// SCIPcreateEmptyRow* hand the caller one capture; SCIPaddRow* take their
// own, so the binding's capture must be released after a successful add —
// and rows created but never added must be released at FreeTransform or
// free, before the instance can tear the LP down. Rows obtained from
// queries (Constraint.Row, Col.Rows) are never in this map and are never
// released by the binding.
//
// Sub-SCIP copies: rows created through a copy's callback model belong to
// the copy's raw pointer, and the copy's plugin free callbacks call
// forgetCopy, which releases them there.
var rowOwners = struct {
	sync.Mutex
	nextInc uint64
	rows    map[*C.SCIP_ROW]rowOwner
}{rows: make(map[*C.SCIP_ROW]rowOwner)}

// rowOwner pairs the owning raw instance with the row's incarnation, a
// number from a monotonic counter unique to every row the binding creates.
// copyInc is the sub-SCIP incarnation the row was created in (0 for the
// main instance), so a teardown only releases rows of the copy it is
// actually tearing down, never stale entries from an older copy at the
// same address. added marks a row whose add succeeded while the binding's
// release failed: SCIP holds its own capture, so it must not be released
// as if the binding's were the last one.
type rowOwner struct {
	scip    *C.SCIP
	inc     uint64
	copyInc uint64
	added   bool
}

// rowsReleasedForTest counts binding-owned captures released, so tests can
// pin the ownership rules without dipping into C. Same-package tests reset
// it directly.
var rowsReleasedForTest int

// deadRows records the rows whose final capture the binding dropped
// deterministically — through Row.Release, or the teardown sweeps for rows
// that were never added — as a tombstone keyed by pointer and valued by the
// dead row's incarnation. A handle dies when a tombstone at its address is
// at least as new as the handle's incarnation, so a released row stays
// dead even after the address is reused for a fresh one (which simply gets
// a higher incarnation), and tombstones never need clearing. Rows the
// binding adds are never tombstoned: SCIP holds its own capture while it
// uses them, and they stay inspectable. Rows SCIP releases on its own (a
// cut removed from the LP) remain undetectable here; that is the open
// handle-liveness problem of #20.
var deadRows = struct {
	sync.Mutex
	m map[*C.SCIP_ROW]map[*C.SCIP]uint64
}{m: make(map[*C.SCIP_ROW]map[*C.SCIP]uint64)}

// poisonRow marks a row as gone for its owning instance: the binding
// dropped the final capture of that instance's allocation. Tombstones are
// per owner, because two live models can release rows at the same recycled
// address — one model's purge must not unprotect the other's handles.
func poisonRow(row *C.SCIP_ROW, owner *C.SCIP, inc uint64) {
	deadRows.Lock()
	owners := deadRows.m[row]
	if owners == nil {
		owners = make(map[*C.SCIP]uint64)
		deadRows.m[row] = owners
	}
	if owners[owner] < inc {
		owners[owner] = inc
	}
	deadRows.Unlock()
}

// purgeDeadRows drops the tombstones of one instance, once its handles are
// dead by a stronger rule: after a successful FreeTransform (the transform
// generation moved on) or when the instance itself is gone. Without this,
// long-lived processes would grow one map entry per released row address.
func purgeDeadRows(owner *C.SCIP) {
	deadRows.Lock()
	for row, owners := range deadRows.m {
		delete(owners, owner)
		if len(owners) == 0 {
			delete(deadRows.m, row)
		}
	}
	deadRows.Unlock()
}

// deadRowErr reports the error a method on a released row returns. Query
// handles (incarnation 0) are never killed by tombstones: the binding did
// not mint them and cannot know which allocation of the address they saw.
func (r Row) deadRowErr(op string) error {
	if r.inc == 0 || r.raw == nil {
		return nil
	}
	deadRows.Lock()
	owners := deadRows.m[r.raw]
	tomb := uint64(0)
	if owners != nil {
		tomb = owners[r.scip.raw]
	}
	deadRows.Unlock()
	if tomb >= r.inc {
		return Model{scip: r.scip}.invalid(op, RetcodeInvalidCall, "row was released")
	}
	return nil
}

// ownRow records the binding's capture on a freshly created row and hands
// back the row's incarnation.
func ownRow(s *Scip, row *C.SCIP_ROW) uint64 {
	cInc := copyIncarnation(s.raw)
	rowOwners.Lock()
	rowOwners.nextInc++
	inc := rowOwners.nextInc
	rowOwners.rows[row] = rowOwner{scip: s.raw, inc: inc, copyInc: cInc}
	rowOwners.Unlock()
	return inc
}

// releaseOwnedRow drops the binding's capture on row if it belongs to the
// instance s, and reports whether it did. A row the binding did not create
// (a query result), or one belonging to another instance, is left alone.
// The release happens outside the registry lock; if SCIP refuses it, the
// ownership entry is restored so a later teardown can retry, and the error
// is returned alongside released=false.
func releaseOwnedRowErr(s *Scip, row *C.SCIP_ROW, afterAdd bool) (bool, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	rowOwners.Lock()
	owner, ok := rowOwners.rows[row]
	if !ok || owner.scip != s.raw {
		rowOwners.Unlock()
		return false, nil
	}
	delete(rowOwners.rows, row)
	rowOwners.Unlock()

	// No tombstone here: the add paths share this helper, and a row SCIP
	// retained is alive in the separation storage, the LP or the cut pool —
	// inspectable, exactly as documented. Only the callers that know the
	// capture was the last one poison the handle.
	r := row
	if err := retcodeError(C.SCIPreleaseRow(s.raw, &r)); err != nil {
		owner.added = owner.added || afterAdd // SCIP holds a capture of an added row
		rowOwners.Lock()
		rowOwners.rows[row] = owner // still captured; let teardown retry
		rowOwners.Unlock()
		return false, err
	}
	rowOwners.Lock()
	rowsReleasedForTest++
	rowOwners.Unlock()
	return true, nil
}

// releaseOwnedRow is releaseOwnedRowErr for the post-add paths, which are
// best-effort and must not tombstone: the add itself took a capture, so
// the row stays alive and inspectable while SCIP uses it.
func releaseOwnedRow(s *Scip, row *C.SCIP_ROW) bool {
	ok, _ := releaseOwnedRowErr(s, row, true)
	return ok
}

// discardRowsOfOwner drops the registry entries for an instance without
// releasing anything — for a raw address that once belonged to a sub-SCIP
// which missed its free callbacks: its rows are dangling with the dead
// copy, and must never be passed to SCIPreleaseRow through the fresh
// instance now living at that address.
func discardRowsOfOwner(raw *C.SCIP) {
	rowOwners.Lock()
	for row, owner := range rowOwners.rows {
		if owner.scip == raw {
			delete(rowOwners.rows, row)
		}
	}
	rowOwners.Unlock()
}

// releaseRowsOfOwner drops the binding's captures on every row still held
// by one instance — the rows created and never added — and returns the
// first release error, if any. Rows whose release SCIP refuses are put
// back, so a later teardown can retry them.
func releaseRowsOfOwner(raw *C.SCIP) error {
	return releaseRowsOfOwnerInc(raw, copyIncarnation(raw))
}

// releaseRowsOfOwnerInc is releaseRowsOfOwner with the incarnation the
// caller knows is current — copyDies must pass the one it read before
// forgetting the copy, since afterwards copyIncarnation reports 0.
func releaseRowsOfOwnerInc(raw *C.SCIP, curInc uint64) error {
	rowOwners.Lock()
	var rows []*C.SCIP_ROW
	var owners []rowOwner
	for row, owner := range rowOwners.rows {
		if owner.scip != raw {
			continue
		}
		if owner.copyInc != curInc {
			continue // stale entry of an older copy: drop it below
		}
		rows = append(rows, row)
		owners = append(owners, owner)
		delete(rowOwners.rows, row)
	}
	rowOwners.Unlock()

	var firstErr error
	var failed []*C.SCIP_ROW
	var failedOwners []rowOwner
	for i, row := range rows {
		r := row
		if err := retcodeError(C.SCIPreleaseRow(raw, &r)); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			failed = append(failed, row)
			failedOwners = append(failedOwners, owners[i])
		} else if !owners[i].added {
			// Never added: the binding's capture was the last one, so the
			// row is gone and its handle must die — even if
			// SCIPfreeTransform is about to fail. An added row SCIP still
			// holds stays inspectable; only the binding's capture went.
			poisonRow(row, raw, owners[i].inc)
		}
	}
	if len(failed) > 0 {
		// Put the refused rows back so a later teardown can retry them.
		rowOwners.Lock()
		for i, row := range failed {
			rowOwners.rows[row] = failedOwners[i]
		}
		rowOwners.Unlock()
	}
	// Entries of older incarnations at this address are stale data: the
	// copies they belonged to are gone. Drop them so they can never be
	// released through whoever lives at the address now.
	discardStaleRows(raw, curInc)
	rowOwners.Lock()
	rowsReleasedForTest += len(rows) - len(failed)
	rowOwners.Unlock()
	return firstErr
}

// discardStaleRows drops the ownership entries of an address whose copy
// incarnation is not the current one.
func discardStaleRows(raw *C.SCIP, curInc uint64) {
	rowOwners.Lock()
	for row, owner := range rowOwners.rows {
		if owner.scip == raw && owner.copyInc != curInc {
			delete(rowOwners.rows, row)
		}
	}
	rowOwners.Unlock()
}

// TryRelease releases the binding's capture on a row it created, freeing it
// immediately when SCIP holds no other capture — for a plugin that built a
// row and then decided not to add it. Rows added through Model.AddCut,
// Prober.AddRow or Diver.AddRow are released by those methods; rows the
// binding did not create (results of Constraint.Row, Col.Rows) are not the
// binding's to release, and neither is a row that was already released, so
// both fail with RetcodeInvalidData. After a successful release the Row
// value is no longer usable.
func (r Row) TryRelease() error {
	defer runtime.KeepAlive(r.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	m := Model{scip: r.scip}
	if err := m.checkHandle("Row.Release", "Row", r.raw != nil, r.scip, r.gen, false); err != nil {
		return err
	}
	if err := r.deadRowErr("Row.Release"); err != nil {
		return err
	}
	rowOwners.Lock()
	owner, ok := rowOwners.rows[r.raw]
	rowOwners.Unlock()
	notOurs := func() error {
		return Model{scip: r.scip}.invalid("Row.Release", RetcodeInvalidData,
			"row was not created by the binding, or is already released")
	}
	if !ok {
		return notOurs()
	}
	if owner.scip != r.scip.raw {
		return Model{scip: r.scip}.invalid("Row.Release", RetcodeInvalidData, "row belongs to another model")
	}
	if owner.inc != r.inc {
		return notOurs()
	}
	if owner.added {
		return Model{scip: r.scip}.invalid("Row.Release", RetcodeInvalidData,
			"row was added; SCIP holds its own capture while it uses it")
	}
	released, err := releaseOwnedRowErr(r.scip, r.raw, false)
	if err != nil {
		return Model{scip: r.scip}.wrap("Row.Release", err, "")
	}
	if !released {
		return notOurs()
	}
	// The binding's capture was the last one (an added row is no longer
	// owned): the row is freed and every handle to it must die, including
	// across a later reuse of the address.
	poisonRow(r.raw, r.scip.raw, r.inc)
	return nil
}

// Release releases the binding's capture on the row; see TryRelease. It
// panics on failure.
func (r Row) Release() { must(r.TryRelease()) }
