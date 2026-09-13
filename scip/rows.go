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
	rows map[*C.SCIP_ROW]*C.SCIP
}{rows: make(map[*C.SCIP_ROW]*C.SCIP)}

// rowsReleasedForTest counts binding-owned captures released, so tests can
// pin the ownership rules without dipping into C. Same-package tests reset
// it directly.
var rowsReleasedForTest int

// deadRows records rows whose final capture the binding dropped itself —
// through Row.Release, or an add SCIP did not retain — keyed by pointer and
// owning instance, so every handle minted before the drop fails its
// liveness check instead of dereferencing freed memory. ownRow deletes an
// entry when the address is reused for a fresh row. Rows SCIP releases on
// its own (a cut removed from the LP) remain undetectable here; that is
// the open handle-liveness problem of #20.
var deadRows = struct {
	sync.Mutex
	m map[*C.SCIP_ROW]*C.SCIP
}{m: make(map[*C.SCIP_ROW]*C.SCIP)}

// poisonRow marks a row as gone: the binding dropped its final capture.
func poisonRow(row *C.SCIP_ROW, owner *C.SCIP) {
	deadRows.Lock()
	deadRows.m[row] = owner
	deadRows.Unlock()
}

// unpoisonRow clears the dead mark, for the address reused by a fresh row.
func unpoisonRow(row *C.SCIP_ROW) {
	deadRows.Lock()
	delete(deadRows.m, row)
	deadRows.Unlock()
}

// deadRowErr reports the error a method on a released row returns.
func (r Row) deadRowErr(op string) error {
	deadRows.Lock()
	owner := deadRows.m[r.raw]
	deadRows.Unlock()
	if owner == nil || r.scip == nil || owner != r.scip.raw {
		return nil
	}
	return (&Error{Op: op, Retcode: RetcodeInvalidCall, Detail: "row was released"})
}

// ownRow records the binding's capture on a freshly created row.
func ownRow(s *Scip, row *C.SCIP_ROW) {
	rowOwners.Lock()
	rowOwners.rows[row] = s.raw
	rowOwners.Unlock()
	unpoisonRow(row) // the address may have belonged to a released row
}

// releaseOwnedRow drops the binding's capture on row if it belongs to the
// instance s, and reports whether it did. A row the binding did not create
// (a query result), or one belonging to another instance, is left alone.
// The release happens outside the registry lock; if SCIP refuses it, the
// ownership entry is restored so a later teardown can retry, and the error
// is returned alongside released=false.
func releaseOwnedRowErr(s *Scip, row *C.SCIP_ROW) (bool, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	rowOwners.Lock()
	owner, ok := rowOwners.rows[row]
	if !ok || owner != s.raw {
		rowOwners.Unlock()
		return false, nil
	}
	delete(rowOwners.rows, row)
	rowOwners.Unlock()

	r := row
	if err := retcodeError(C.SCIPreleaseRow(s.raw, &r)); err != nil {
		rowOwners.Lock()
		rowOwners.rows[row] = owner // still captured; let teardown retry
		rowOwners.Unlock()
		return false, err
	}
	poisonRow(row, owner) // the handle must not outlive the final capture
	rowOwners.Lock()
	rowsReleasedForTest++
	rowOwners.Unlock()
	return true, nil
}

// releaseOwnedRow is releaseOwnedRowErr for the post-add paths, which are
// best-effort: the add itself took a capture, so a failed release only
// defers the row's reclamation to teardown.
func releaseOwnedRow(s *Scip, row *C.SCIP_ROW) bool {
	ok, _ := releaseOwnedRowErr(s, row)
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
		if owner == raw {
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
	rowOwners.Lock()
	var owned []*C.SCIP_ROW
	for row, owner := range rowOwners.rows {
		if owner == raw {
			owned = append(owned, row)
			delete(rowOwners.rows, row)
		}
	}
	rowOwners.Unlock()

	var firstErr error
	var failed []*C.SCIP_ROW
	for _, row := range owned {
		r := row
		if err := retcodeError(C.SCIPreleaseRow(raw, &r)); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			failed = append(failed, row)
		}
	}
	if len(failed) > 0 {
		// Put the refused rows back so a later teardown can retry them.
		rowOwners.Lock()
		for _, row := range failed {
			rowOwners.rows[row] = raw
		}
		rowOwners.Unlock()
	}
	rowOwners.Lock()
	rowsReleasedForTest += len(owned) - len(failed)
	rowOwners.Unlock()
	return firstErr
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
	if !ok {
		return (&Error{Op: "Row.Release", Retcode: RetcodeInvalidData,
			Detail: "row was not created by the binding, or is already released"})
	}
	if owner != r.scip.raw {
		return (&Error{Op: "Row.Release", Retcode: RetcodeInvalidData,
			Detail: "row belongs to another model"})
	}
	released, err := releaseOwnedRowErr(r.scip, r.raw)
	if err != nil {
		return err
	}
	if !released {
		return (&Error{Op: "Row.Release", Retcode: RetcodeInvalidData,
			Detail: "row was not created by the binding, or is already released"})
	}
	return nil
}

// Release releases the binding's capture on the row; see TryRelease. It
// panics on failure.
func (r Row) Release() { must(r.TryRelease()) }
