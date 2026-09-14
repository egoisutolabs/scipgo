package scip

/*
#include "helpers.h"
*/
import "C"

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
	"weak"
)

// Scip wraps a raw SCIP instance pointer, mirroring the Rust ScipPtr type.
// It is internal to the package; users interact with Model[Stage] instead.
type Scip struct {
	raw  *C.SCIP
	weak bool
	// owner is the strong instance a weak (callback) wrapper stands for, so
	// liveness is judged by instance identity, not by a pointer that is
	// never cleared and may be reused by a later SCIPcreate. It is held
	// weakly: a plugin that stores a callback handle must not root the model
	// through the plugin registry. copyInc identifies the sub-SCIP incarnation
	// a copy wrapper was minted in, so a later copy at the same address does
	// not revive it.
	owner   weak.Pointer[Scip]
	copyInc uint64
	// transGen counts FreeTransform calls; handles into the transformed
	// problem record it at creation and are dead once it moves on. probGen
	// counts problem replacements (CreateProb, ReadProb), which kill every
	// handle, original ones included.
	transGen uint64
	probGen  uint64
	// logSink is the strong reference to the currently installed log sink;
	// the global registry holds it only weakly, so a callback capturing its
	// Model cannot root the model through the registry and block its
	// finalizer.
	logSink *logSink
	// stopFlag is the Go-side half of Interrupt: set by a caller that wants
	// the solve stopped, relayed to concurrent workers and sub-SCIPs by the
	// interruptForwarder event handler (below). Like SCIP's own
	// user-interrupt flag it is cleared when the next solve starts.
	stopFlag atomic.Bool
	// solving counts solve calls in flight on this instance: 1 from the
	// moment Scip.solve/solveConcurrent enters C until it returns. It is the
	// Go-side answer to "is a solve running", which cannot be read from
	// SCIP's stage field cross-thread, and SCIPinterruptSolve is legal in
	// exactly the stages a solve call brackets.
	solving atomic.Int32
	// fwdIncluded records that interruptForwarder has been included, so it
	// is added at most once per instance.
	fwdIncluded bool
	// Variables added during solving, to be released after solving.
	varsAddedInSolving []*C.SCIP_VAR
	mu                 sync.Mutex
	freed              atomic.Bool
}

// instances maps every live strong instance by its raw pointer, so weak
// wrappers created inside callbacks can find the owner they belong to. The
// values are weak pointers: a strong reference here would keep every model
// reachable forever and its finalizer would never run.
var instances = struct {
	sync.Mutex
	m map[*C.SCIP]weak.Pointer[Scip]
}{m: make(map[*C.SCIP]weak.Pointer[Scip])}

func instanceOf(raw *C.SCIP) *Scip {
	wp, _ := instanceWeak(raw)
	return wp.Value()
}

func instanceWeak(raw *C.SCIP) (weak.Pointer[Scip], bool) {
	instances.Lock()
	defer instances.Unlock()
	wp, ok := instances.m[raw]
	return wp, ok
}

// root returns the strong instance behind s: itself, or a weak wrapper's
// owner, which is nil once that owner has been collected.
func (s *Scip) root() *Scip {
	if s != nil && s.weak {
		return s.owner.Value()
	}
	return s
}

// rootAlive resolves the strong instance behind s exactly once and reports
// whether it is alive: a weak wrapper is alive while its owner is and, for
// a sub-SCIP copy, while SCIP has not freed that copy (its plugins' free
// callbacks drop it from copyParents). Callers that go on to use the
// instance must hold the returned pointer, not resolve again.
func (s *Scip) rootAlive() (*Scip, bool) {
	r := s.root()
	if r == nil || r.raw == nil || r.freed.Load() {
		return r, false
	}
	if s.weak && s.raw != r.raw && copyIncarnation(s.raw) != s.copyInc {
		return r, false // the sub-SCIP this wrapper was minted in is gone
	}
	return r, true
}

// alive reports whether the instance behind s still exists.
func (s *Scip) alive() bool {
	_, ok := s.rootAlive()
	return ok
}

// gen is the generation a handle must carry to be valid: the problem
// generation for original-problem objects, the transform generation for
// everything else.
func (s *Scip) gen(orig bool) uint64 { return genOf(s.root(), orig) }

// genOf is gen on an already resolved root.
func genOf(r *Scip, orig bool) uint64 {
	if orig {
		return r.probGen
	}
	return r.transGen
}

// newProblem records that the problem was replaced: every handle is dead.
func (s *Scip) newProblem() {
	r := s.root()
	r.probGen++
	r.transGen++
}

// newScip creates a new SCIP instance and registers a finalizer that frees it
// when it becomes unreachable (mirroring the Rust Drop impl).
func newScip() (*Scip, error) {
	var scipPtr *C.SCIP
	if err := retcodeError(C.SCIPcreate(&scipPtr)); err != nil {
		return nil, err
	}
	s := &Scip{raw: scipPtr}
	forgetCopy(scipPtr)         // the address may have belonged to a freed sub-SCIP
	discardRowsOfOwner(scipPtr) // its stale row entries are dangling: drop, never release
	instances.Lock()
	instances.m[scipPtr] = weak.Make(s)
	instances.Unlock()
	runtime.SetFinalizer(s, (*Scip).release)
	return s, nil
}

// release finalizes the Scip, releasing all captured objects and freeing the
// SCIP instance (unless weak).
func (s *Scip) release() {
	defer func() { _ = recover() }() // never panic from a finalizer
	_ = s.free()
}

// free releases the instance; it keeps going past individual release
// failures so SCIPfree always runs, and returns the first error seen.
func (s *Scip) free() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	s.mu.Lock()
	if s.freed.Load() || s.weak {
		s.mu.Unlock()
		return nil
	}
	s.freed.Store(true)
	s.mu.Unlock()
	instances.Lock()
	delete(instances.m, s.raw)
	instances.Unlock()

	raw := s.raw
	var firstErr error
	check := func(rc C.SCIP_RETCODE) {
		if err := retcodeError(rc); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Release original variables and constraints if the instance reached the
	// problem stage (mirrors the Rust Drop impl).
	stage := C.SCIPgetStage(raw)
	switch stage {
	case C.SCIP_STAGE_PROBLEM,
		C.SCIP_STAGE_TRANSFORMED,
		C.SCIP_STAGE_INITPRESOLVE,
		C.SCIP_STAGE_PRESOLVING,
		C.SCIP_STAGE_EXITPRESOLVE,
		C.SCIP_STAGE_PRESOLVED,
		C.SCIP_STAGE_INITSOLVE,
		C.SCIP_STAGE_SOLVING,
		C.SCIP_STAGE_SOLVED,
		C.SCIP_STAGE_EXITSOLVE:
		nVars := C.SCIPgetNOrigVars(raw)
		vars := C.SCIPgetOrigVars(raw)
		for i := C.int(0); i < nVars; i++ {
			v := cVarAt(vars, int(i))
			check(C.SCIPreleaseVar(raw, &v))
		}

		for _, v := range s.varsAddedInSolving {
			check(C.SCIPreleaseVar(raw, &v))
		}

		nConss := C.SCIPgetNOrigConss(raw)
		conss := C.SCIPgetOrigConss(raw)
		for i := C.int(0); i < nConss; i++ {
			c := cConsAt(conss, int(i))
			check(C.SCIPreleaseCons(raw, &c))
		}
	}

	// The binding's still-held row captures go before SCIPfree tears down
	// the LP; a debug SCIP asserts if they outlive the instance.
	if err := releaseRowsOfOwner(s.raw); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.scipFree(raw); err != nil && firstErr == nil {
		firstErr = err
	}
	// The instance is gone: entries a refused release restored can never be
	// retried through it, and the rows are dangling with its memory — drop
	// them, and the tombstones, like the copy teardown does.
	discardRowsOfOwner(raw)
	purgeDeadRows(raw)
	// Drop panics stashed by plugin free callbacks: the raw pointer may be
	// reused by a later SCIPcreate, which would otherwise rethrow them.
	if ps := takePanics(s.raw); len(ps) > 0 && firstErr == nil {
		firstErr = fmt.Errorf("panic in plugin free callback: %v", ps[0])
	}
	deleteDatastore(s)
	// scipFree released the message handler, whose free callback flushed the
	// sink; drop the strong reference so a callback that captured the Model
	// does not keep it (and everything the callback closes over) alive.
	s.logSink = nil
	s.raw = nil
	return firstErr
}

// ------------------------------------------------------------- parameters

func (s *Scip) setStrParam(param, value string) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp, cv := cString(param), cString(value)
	defer func() { freeCString(cp); freeCString(cv) }()
	return retcodeError(C.SCIPsetStringParam(s.raw, cp, cv))
}

func (s *Scip) strParam(param string) (string, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	var value *C.char
	if err := retcodeError(C.SCIPgetStringParam(s.raw, cp, &value)); err != nil {
		return "", err
	}
	return goString(value), nil
}

func (s *Scip) setBoolParam(param string, value bool) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	var v C.uint
	if value {
		v = 1
	}
	return retcodeError(C.SCIPsetBoolParam(s.raw, cp, v))
}

func (s *Scip) boolParam(param string) (bool, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	var value C.uint
	if err := retcodeError(C.SCIPgetBoolParam(s.raw, cp, &value)); err != nil {
		return false, err
	}
	return value != 0, nil
}

func (s *Scip) setIntParam(param string, value int32) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	return retcodeError(C.SCIPsetIntParam(s.raw, cp, C.int(value)))
}

func (s *Scip) intParam(param string) (int32, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	var value C.int
	if err := retcodeError(C.SCIPgetIntParam(s.raw, cp, &value)); err != nil {
		return 0, err
	}
	return int32(value), nil
}

func (s *Scip) setLongintParam(param string, value int64) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	return retcodeError(C.SCIPsetLongintParam(s.raw, cp, C.longlong(value)))
}

func (s *Scip) longintParam(param string) (int64, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	var value C.longlong
	if err := retcodeError(C.SCIPgetLongintParam(s.raw, cp, &value)); err != nil {
		return 0, err
	}
	return int64(value), nil
}

func (s *Scip) setRealParam(param string, value float64) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	return retcodeError(C.SCIPsetRealParam(s.raw, cp, C.double(value)))
}

func (s *Scip) realParam(param string) (float64, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(param)
	defer freeCString(cp)
	var value C.double
	if err := retcodeError(C.SCIPgetRealParam(s.raw, cp, &value)); err != nil {
		return 0, err
	}
	return float64(value), nil
}

func (s *Scip) setPresolving(setting ParamSetting) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsetPresolving(s.raw, setting.toC(), 1))
}

func (s *Scip) setSeparating(setting ParamSetting) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsetSeparating(s.raw, setting.toC(), 1))
}

func (s *Scip) setHeuristics(setting ParamSetting) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsetHeuristics(s.raw, setting.toC(), 1))
}

// ------------------------------------------------------------- lifecycle

func (s *Scip) createProb(name string) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	err := retcodeError(C.SCIPcreateProbBasic(s.raw, cn))
	if err == nil {
		err = retcodeError(C.scipgo_watchProblem(s.raw)) // told when this problem is freed
	}
	return err
}

func (s *Scip) readProb(filename string) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cf := cString(filename)
	defer freeCString(cf)
	// A reader replaces the problem by calling SCIPcreateProb, possibly before
	// failing. The old problem's delorig hook fires inside that call, which
	// is the authoritative signal that its handles are dead; a read that
	// fails earlier (no file, wrong stage) never triggers it.
	rc := C.SCIPreadProb(s.raw, cf, nil)

	// SCIPreadProb creates the problem (and its variables/constraints)
	// before it can fail on, e.g., invalid data. Capture them here whenever
	// the problem stage was reached, so that free() releases a balanced
	// number of references (mirrors russcip issue #281).
	if C.SCIPgetStage(s.raw) == C.SCIP_STAGE_PROBLEM {
		if err := retcodeError(C.scipgo_watchProblem(s.raw)); err != nil && rc == C.SCIP_OKAY {
			return err
		}
		nVars := C.SCIPgetNVars(s.raw)
		vars := C.SCIPgetVars(s.raw)
		for i := C.int(0); i < nVars; i++ {
			C.SCIPcaptureVar(s.raw, cVarAt(vars, int(i)))
		}
		nConss := C.SCIPgetNConss(s.raw)
		conss := C.SCIPgetConss(s.raw)
		for i := C.int(0); i < nConss; i++ {
			mustOK(C.SCIPcaptureCons(s.raw, cConsAt(conss, int(i))))
		}
	}

	return retcodeError(rc)
}

func (s *Scip) setObjSense(sense ObjSense) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsetObjsense(s.raw, sense.toC()))
}

func (s *Scip) setObjIntegral() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsetObjIntegral(s.raw))
}

func (s *Scip) nVars() int {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return int(C.SCIPgetNVars(s.raw))
}

func (s *Scip) nConss() int {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return int(C.SCIPgetNConss(s.raw))
}

func (s *Scip) findCons(name string) *C.SCIP_CONS {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	return C.SCIPfindCons(s.raw, cn)
}

func (s *Scip) findHeur(name string) *C.SCIP_HEUR {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	return C.SCIPfindHeur(s.raw, cn)
}

func (s *Scip) findSepa(name string) *C.SCIP_SEPA {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	return C.SCIPfindSepa(s.raw, cn)
}

func (s *Scip) findPresol(name string) *C.SCIP_PRESOL {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	return C.SCIPfindPresol(s.raw, cn)
}

func (s *Scip) findNodesel(name string) *C.SCIP_NODESEL {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	return C.SCIPfindNodesel(s.raw, cn)
}

func (s *Scip) getTransformedCons(c Constraint) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var transformed *C.SCIP_CONS
	if err := retcodeError(C.SCIPgetTransformedCons(s.raw, c.raw, &transformed)); err != nil {
		return nil, err
	}
	return transformed, nil
}

func (s *Scip) status() Status {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	// Since SCIP 10, SCIPgetStatus dereferences scip->stat, which is not
	// allocated before a problem is created (the INIT stage).
	if C.SCIPgetStage(s.raw) == C.SCIP_STAGE_INIT {
		return StatusUnknown
	}
	return statusFromC(C.SCIPgetStatus(s.raw))
}

func (s *Scip) printVersion() {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	C.SCIPprintVersion(s.raw, nil)
}

func (s *Scip) write(path, ext string, symb bool) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	// SCIPwriteOrigProblem takes "genericnames", the inverse of symb.
	genericNames := C.uint(1)
	if symb {
		genericNames = 0
	}
	cp, ce := cString(path), cString(ext)
	defer func() { freeCString(cp); freeCString(ce) }()
	return retcodeError(C.SCIPwriteOrigProblem(s.raw, cp, ce, genericNames))
}

func (s *Scip) includeDefaultPlugins() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if err := retcodeError(C.SCIPincludeDefaultPlugins(s.raw)); err != nil {
		return err
	}
	// Included here so every conventionally built model can be interrupted
	// in every stage, including a concurrent solve re-started from a stage
	// where plugins can no longer be added.
	return s.includeInterruptForwarder()
}

// statisticsJSON returns the solving statistics in JSON format
// (SCIPprintStatisticsJson), capturing output through a temporary file.
func (s *Scip) statisticsJSON() (string, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	file := C.tmpfile()
	if file == nil {
		return "", RetcodeFileCreateError
	}
	defer C.fclose(file)

	if rc := C.SCIPprintStatisticsJson(s.raw, file); rc != C.SCIP_OKAY {
		return "", retcodeFromC(rc)
	}
	C.fflush(file)
	C.rewind(file)

	var buf []byte
	chunk := make([]byte, 4096)
	for {
		n := C.fread(unsafe.Pointer(&chunk[0]), 1, C.size_t(len(chunk)), file)
		if n == 0 {
			break
		}
		buf = append(buf, chunk[:n]...)
	}
	return string(buf), nil
}

func (s *Scip) writeStatisticsJSON(path string) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cp := cString(path)
	cw := cString("w")
	defer func() { freeCString(cp); freeCString(cw) }()
	file := C.fopen(cp, cw)
	if file == nil {
		return RetcodeFileCreateError
	}
	rc := C.SCIPprintStatisticsJson(s.raw, file)
	C.fclose(file)
	return retcodeError(rc)
}

// vars returns the problem (or original) variables keyed by their index.
func (s *Scip) vars(original bool) map[int]*C.SCIP_VAR {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var nVars int
	var scipVars **C.SCIP_VAR
	if original {
		nVars = int(C.SCIPgetNOrigVars(s.raw))
		scipVars = C.SCIPgetOrigVars(s.raw)
	} else {
		nVars = s.nVars()
		scipVars = C.SCIPgetVars(s.raw)
	}
	out := make(map[int]*C.SCIP_VAR, nVars)
	for i := 0; i < nVars; i++ {
		v := cVarAt(scipVars, i)
		out[int(C.SCIPvarGetIndex(v))] = v
	}
	return out
}

func (s *Scip) conss() []*C.SCIP_CONS {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	n := s.nConss()
	scipConss := C.SCIPgetConss(s.raw)
	out := make([]*C.SCIP_CONS, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, cConsAt(scipConss, i))
	}
	return out
}

// solve brackets the native solve call with the instance's solve counter;
// every other interrupt mechanism keys off that counter or the stop flag.
func (s *Scip) solve() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if r := s.root(); r != nil {
		r.solving.Add(1)
		defer r.solving.Add(-1)
	}
	return retcodeError(C.SCIPsolve(s.raw))
}

// clearInterrupt discards a leftover stop request, mirroring the reset SCIP's
// SCIPsolve does to its own user-interrupt flag. It runs before a solve is
// started — before the context watcher exists in the SolveContext path, so a
// cancellation observed from there on cannot be wiped by solve startup.
func (s *Scip) clearInterrupt() {
	if r := s.root(); r != nil {
		r.stopFlag.Store(false)
	}
}

// interrupt asks SCIP to stop the solve in progress on s's root at the next
// opportunity: between nodes, presolve rounds, LP iterations and pricing
// rounds, but only once the currently running plugin callback has returned.
// It may be called from any goroutine and is a no-op if the instance is gone
// or nothing is solving. A stop request left over from before a solve is
// discarded when the next one starts.
func (s *Scip) interrupt() {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	r := s.root()
	if r == nil || r.raw == nil || r.freed.Load() {
		return
	}
	r.stopFlag.Store(true)
	// SCIPinterruptSolve is SCIP's own asynchronous Ctrl-C path: a single
	// boolean write the solving thread polls, safe from any thread. It is
	// refused outside PROBLEM..FREETRANS, which is why it is issued only
	// while the counter brackets a solve call; SCIP's stage field itself
	// must not be read here, as the solving thread may be changing it.
	if r.solving.Load() > 0 {
		C.SCIPinterruptSolve(r.raw)
	}
}

// interruptPollInterval is how often interruptWhile re-issues the native
// interrupt while a solve runs.
const interruptPollInterval = 2 * time.Millisecond

// interruptWhile is the context-watcher half of SolveContext: it makes the
// stop request and keeps re-issuing the native interrupt until ch — closed
// when the solve call returns — says the solve is over. The re-issue is what
// makes a cancellation delivered during solve startup survive: SCIPsolve and
// SCIPsolveConcurrent reset SCIP's interrupt flag once on entry, and any
// issue after that reset sticks until the solving loop notices it. Only
// SCIPinterruptSolve is called (see interrupt); the loop ends within one
// interval of ch closing, before SolveContext returns to its caller.
func (s *Scip) interruptWhile(ch <-chan struct{}) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	r := s.root()
	if r == nil || r.raw == nil {
		return
	}
	r.stopFlag.Store(true)
	for {
		if !r.freed.Load() && r.solving.Load() > 0 {
			C.SCIPinterruptSolve(r.raw)
		}
		select {
		case <-ch:
			return
		case <-time.After(interruptPollInterval):
		}
	}
}

// interruptForwarderName is the SCIP name of the forwarder event handler.
const interruptForwarderName = "scipgo_interrupt"

// interruptForwarder relays a stop request into the SCIP instance a callback
// runs in. A sequential solve is stopped by SCIPinterruptSolve on the main
// instance directly, but the workers of a concurrent solve (and the sub-SCIPs
// of LNS heuristics) are separate SCIP instances that never see the main
// instance's flag, and the syncstore flag meant to stop them cannot be
// written from a Go thread while SCIP's OpenMP thread pool is busy: its lock
// deadlocks for threads the runtime did not create. A Copyable event handler
// copied into every worker sidesteps both: it runs on a solver thread, where
// interrupting the instance it executes in is always legal. The mask gives
// the same granularity as SCIP's own interrupt checks wherever SCIP emits
// events for them: PRESOLVEROUND between presolve rounds, NODEFOCUSED for
// every node — non-LP ones included, so node-completion events are not
// needed — and the LP events between the LP solves of a single node's
// cut-and-price loop. Node completion is deliberately not in the mask: a
// worker whose last node completes has finished, and a solve that finishes
// returns its result rather than a manufactured interrupt. Events SCIP does
// not emit — inside a worker's pricing loop, say — delay a forwarded stop
// until the next boundary.
type interruptForwarder struct{}

func (interruptForwarder) GetEventMask() EventMask {
	return EventMaskNodeFocused | EventMaskPresolveRound | EventMaskLpEvent
}

func (interruptForwarder) Execute(model Model, h EventhdlrPlugin, event Event) {
	defer runtime.KeepAlive(model.scip.root()) // pin the owner for the flag read
	s := model.scip
	r := s.root()
	if r == nil || !r.stopFlag.Load() {
		return
	}
	// s.raw is the instance this callback executes in — the main instance
	// for a sequential solve, one worker or sub-SCIP copy otherwise — and
	// cannot be freed before the callback returns, so unlike the owner it
	// needs no pinning.
	C.SCIPinterruptSolve(s.raw)
}

// Copyable; see interruptForwarder.
func (interruptForwarder) Copy() any { return interruptForwarder{} }

// includeInterruptForwarder adds the forwarder once. An instance may also
// have received it through a plugin copy (SCIP copying a model's plugins
// into a sub-SCIP or another model copies Copyable ones wholesale), which
// does not go through here, so existence is checked by name — verifying
// through the Go plugin registry that the handler found under the name is
// really this forwarder, since the name is user-visible and reserved: an
// application squatting on it gets the duplicate-include error from SCIP
// rather than silently losing the ability to stop concurrent solves.
func (s *Scip) includeInterruptForwarder() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if s.fwdIncluded {
		return nil
	}
	// SCIPincludeEventhdlr is legal in Init and Problem only; a solve
	// started from a later stage keeps whatever forwarder was included
	// before, if any.
	if st := Stage(int(C.SCIPgetStage(s.raw))); st > StageProblem {
		return nil
	}
	cn := cString(interruptForwarderName)
	defer func() { freeCString(cn) }()
	if h := C.SCIPfindEventhdlr(s.raw, cn); h != nil {
		if _, ours := plugins.get(uintptr(C.scipgo_eventhdlrId(h))).(interruptForwarder); ours {
			s.fwdIncluded = true
			return nil
		}
	}
	if err := s.includeEventhdlr(interruptForwarderName, "relays scip.Interrupt into concurrent workers", interruptForwarder{}); err != nil {
		return err
	}
	s.fwdIncluded = true
	return nil
}

// SCIP's task processing interface is one process-wide thread pool: an
// instance's first concurrent solve creates it (inside SCIPsyncstoreInit) and
// SCIPfree of that instance destroys it, neither checking whether another
// instance created or destroyed it in between. Two instances that both ran a
// concurrent solve therefore crash on the second free. tpiPool tracks whether
// the pool exists so create/destroy stay balanced, and serialises concurrent
// solves and the frees of concurrent-solved instances, which one global pool
// requires anyway.
var tpiPool struct {
	sem  chan struct{} // take to use the pool; a channel, not a Mutex, so a wait for it can be abandoned without a stranded goroutine
	live bool
}

func init() { tpiPool.sem = make(chan struct{}, 1) }

// holdsTPI reports whether this instance ran a concurrent solve, i.e. whether
// its SCIPfree will destroy the thread pool.
func (s *Scip) holdsTPI() bool {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPsyncstoreIsInitialized(C.SCIPgetSyncstore(s.raw)) != 0
}

func tpiInit(nthreads int32) error {
	if err := retcodeError(C.SCIPtpiInit(C.int(nthreads), C.int(math.MaxInt32), 0)); err != nil {
		return err
	}
	tpiPool.live = true
	return nil
}

// tpiPoolAcquire takes the pool, giving the wait up when wait closes first.
// A send on the semaphore is cancellable, so an abandoned wait leaves no
// goroutine blocked behind whoever holds the pool.
func tpiPoolAcquire(wait <-chan struct{}) bool {
	if wait == nil {
		tpiPool.sem <- struct{}{}
		return true
	}
	select {
	case tpiPool.sem <- struct{}{}:
		return true
	case <-wait:
		return false
	}
}

// tpiPoolRelease gives the pool back.
func tpiPoolRelease() { <-tpiPool.sem }

// solveConcurrent runs SCIPsolveConcurrent under the process-wide pool lock.
// wait, when non-nil, lets the lock wait be abandoned — for a context that
// is done waiting for another model's concurrent solve — in which case no
// lock is held, no solve is run and abandoned reports true.
func (s *Scip) solveConcurrent(wait <-chan struct{}) (abandoned bool, err error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	// The workers are separate SCIP instances; without the forwarder a
	// concurrent solve could not be stopped before it runs to completion.
	if r := s.root(); r != nil {
		if err := r.includeInterruptForwarder(); err != nil {
			return false, err
		}
	}
	if !tpiPoolAcquire(wait) {
		return true, nil
	}
	defer tpiPoolRelease()
	if r := s.root(); r != nil {
		r.solving.Add(1)
		defer r.solving.Add(-1)
	}
	if s.holdsTPI() {
		// Re-solve: SCIP reuses its solvers and expects the pool to exist.
		if !tpiPool.live {
			n, _ := s.intParam("parallel/maxnthreads") // an upper bound on the solver count
			if err := tpiInit(max(n, 1)); err != nil {
				return false, err
			}
		}
	} else if tpiPool.live {
		// SCIP is about to create a pool; drop the one another instance left.
		if err := retcodeError(C.SCIPtpiExit()); err != nil {
			return false, err
		}
		tpiPool.live = false
	}
	err = retcodeError(C.SCIPsolveConcurrent(s.raw))
	tpiPool.live = tpiPool.live || s.holdsTPI()
	return false, err
}

// scipFree calls SCIPfree, giving it a thread pool to destroy if this
// instance expects one and another instance already destroyed it.
func (s *Scip) scipFree(raw *C.SCIP) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if !s.holdsTPI() {
		return retcodeError(C.SCIPfree(&raw))
	}
	tpiPoolAcquire(nil)
	defer tpiPoolRelease()
	if !tpiPool.live {
		if err := tpiInit(1); err != nil {
			return err // SCIPfree would crash in SCIPtpiExit; leaking beats crashing
		}
	}
	rc := C.SCIPfree(&raw)
	tpiPool.live = false
	return retcodeError(rc)
}

func (s *Scip) nSols() int {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return int(C.SCIPgetNSols(s.raw))
}

func (s *Scip) bestSol() *C.SCIP_SOL {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if s.nSols() == 0 {
		return nil
	}
	return C.SCIPgetBestSol(s.raw)
}

func (s *Scip) getSols() []*C.SCIP_SOL {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	n := s.nSols()
	if n == 0 {
		return nil
	}
	return cSlice(C.SCIPgetSols(s.raw), n)
}

func (s *Scip) objVal() float64 {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return float64(C.SCIPgetPrimalbound(s.raw))
}

func (s *Scip) bestBound() float64 {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return float64(C.SCIPgetDualbound(s.raw))
}

func (s *Scip) nNodes() int {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return int(C.SCIPgetNNodes(s.raw))
}

func (s *Scip) solvingTime() float64 {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return float64(C.SCIPgetSolvingTime(s.raw))
}

func (s *Scip) nLPIterations() int {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return int(C.SCIPgetNLPIterations(s.raw))
}

// ------------------------------------------------------------- variables

func (s *Scip) createVar(lb, ub, obj float64, name string, varType VarType) (*C.SCIP_VAR, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	var varPtr *C.SCIP_VAR
	if err := retcodeError(C.SCIPcreateVarBasic(s.raw, &varPtr, cn,
		C.double(lb), C.double(ub), C.double(obj), varType.toC())); err != nil {
		return nil, err
	}
	if err := retcodeError(C.SCIPaddVar(s.raw, varPtr)); err != nil {
		return nil, err
	}
	return varPtr, nil
}

func (s *Scip) createVarSolving(lb, ub, obj float64, name string, varType VarType) (*C.SCIP_VAR, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	var varPtr *C.SCIP_VAR
	if err := retcodeError(C.SCIPcreateVarBasic(s.raw, &varPtr, cn,
		C.double(lb), C.double(ub), C.double(obj), varType.toC())); err != nil {
		return nil, err
	}
	if err := retcodeError(C.SCIPaddVar(s.raw, varPtr)); err != nil {
		return nil, err
	}
	var transVar *C.SCIP_VAR
	if err := retcodeError(C.SCIPgetTransformedVar(s.raw, varPtr, &transVar)); err != nil {
		return nil, err
	}
	mustOK(C.SCIPreleaseVar(s.raw, &varPtr))
	return transVar, nil
}

func (s *Scip) createPricedVar(lb, ub, obj float64, name string, varType VarType) (*C.SCIP_VAR, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	var varPtr *C.SCIP_VAR
	if err := retcodeError(C.SCIPcreateVarBasic(s.raw, &varPtr, cn,
		C.double(lb), C.double(ub), C.double(obj), varType.toC())); err != nil {
		return nil, err
	}
	// 1.0 is used as a default score for now
	if err := retcodeError(C.SCIPaddPricedVar(s.raw, varPtr, 1.0)); err != nil {
		return nil, err
	}
	var transVar *C.SCIP_VAR
	if err := retcodeError(C.SCIPgetTransformedVar(s.raw, varPtr, &transVar)); err != nil {
		return nil, err
	}
	mustOK(C.SCIPreleaseVar(s.raw, &varPtr))
	return transVar, nil
}

func varFromID(scip *C.SCIP, varProbID int) *C.SCIP_VAR {
	nVars := int(C.SCIPgetNVars(scip))
	if varProbID >= nVars || varProbID < 0 {
		return nil
	}
	return cVarAt(C.SCIPgetVars(scip), varProbID)
}

// ------------------------------------------------------------ constraints

func (s *Scip) createCons(node *Node, vars []Variable, coefs []float64, lhs, rhs float64, name string, local bool) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if len(vars) != len(coefs) {
		return nil, fmt.Errorf("number of variables (%d) and coefficients (%d) differ", len(vars), len(coefs))
	}
	cn := cString(name)
	defer freeCString(cn)
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicLinear(s.raw, &cons, cn,
		0, nil, nil, C.double(lhs), C.double(rhs))); err != nil {
		return nil, err
	}
	for i := range vars {
		if err := retcodeError(C.SCIPaddCoefLinear(s.raw, cons, vars[i].raw, C.double(coefs[i]))); err != nil {
			return nil, err
		}
	}
	if local {
		if node != nil {
			if err := retcodeError(C.SCIPaddConsNode(s.raw, node.raw, cons, nil)); err != nil {
				return nil, err
			}
		} else {
			if err := retcodeError(C.SCIPaddConsLocal(s.raw, cons, nil)); err != nil {
				return nil, err
			}
		}
	} else {
		if err := retcodeError(C.SCIPaddCons(s.raw, cons)); err != nil {
			return nil, err
		}
	}

	if C.SCIPgetStage(s.raw) == C.SCIP_STAGE_SOLVING {
		// SCIP holds its own reference from SCIPaddCons*; drop ours but keep
		// the pointer, since SCIPreleaseCons clears the variable it is given.
		kept := cons
		mustOK(C.SCIPreleaseCons(s.raw, &cons))
		return kept, nil
	}
	return cons, nil
}

func (s *Scip) createConsSetPart(vars []Variable, name string) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicSetpart(s.raw, &cons, cn, 0, nil)); err != nil {
		return nil, err
	}
	for _, v := range vars {
		if err := retcodeError(C.SCIPaddCoefSetppc(s.raw, cons, v.raw)); err != nil {
			return nil, err
		}
	}
	return cons, retcodeError(C.SCIPaddCons(s.raw, cons))
}

func (s *Scip) createConsSetCover(vars []Variable, name string) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicSetcover(s.raw, &cons, cn, 0, nil)); err != nil {
		return nil, err
	}
	for _, v := range vars {
		if err := retcodeError(C.SCIPaddCoefSetppc(s.raw, cons, v.raw)); err != nil {
			return nil, err
		}
	}
	return cons, retcodeError(C.SCIPaddCons(s.raw, cons))
}

func (s *Scip) createConsSetPack(vars []Variable, name string) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicSetpack(s.raw, &cons, cn, 0, nil)); err != nil {
		return nil, err
	}
	for _, v := range vars {
		if err := retcodeError(C.SCIPaddCoefSetppc(s.raw, cons, v.raw)); err != nil {
			return nil, err
		}
	}
	return cons, retcodeError(C.SCIPaddCons(s.raw, cons))
}

func (s *Scip) createConsQuadratic(linVars []Variable, linCoefs []float64,
	quadVars1, quadVars2 []Variable, quadCoefs []float64, lhs, rhs float64, name string) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if len(linVars) != len(linCoefs) {
		return nil, fmt.Errorf("linear variables (%d) and coefficients (%d) differ", len(linVars), len(linCoefs))
	}
	if len(quadVars1) != len(quadVars2) || len(quadVars1) != len(quadCoefs) {
		return nil, fmt.Errorf("quadratic term arrays have mismatched lengths")
	}
	cn := cString(name)
	defer freeCString(cn)
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicQuadraticNonlinear(s.raw, &cons, cn,
		C.int(len(linVars)), cVarSlice(linVars), cDoubleSlice(linCoefs),
		C.int(len(quadVars1)), cVarSlice(quadVars1), cVarSlice(quadVars2), cDoubleSlice(quadCoefs),
		C.double(lhs), C.double(rhs))); err != nil {
		return nil, err
	}
	return cons, retcodeError(C.SCIPaddCons(s.raw, cons))
}

// createConsNonlinear adds lhs <= expr + sum(linCoefs*linVars) <= rhs.
func (s *Scip) createConsNonlinear(expr Expr, linVars []Variable, linCoefs []float64, lhs, rhs float64, name string) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if len(linVars) != len(linCoefs) {
		return nil, fmt.Errorf("linear variables (%d) and coefficients (%d) differ", len(linVars), len(linCoefs))
	}
	raw, err := expr.build(s)
	if err != nil {
		return nil, err
	}
	defer C.SCIPreleaseExpr(s.raw, &raw) // the constraint captures its own reference
	cn := cString(name)
	defer freeCString(cn)
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicNonlinear(s.raw, &cons, cn, raw, C.double(lhs), C.double(rhs))); err != nil {
		return nil, err
	}
	for i := range linVars {
		if err := retcodeError(C.SCIPaddLinearVarNonlinear(s.raw, cons, linVars[i].raw, C.double(linCoefs[i]))); err != nil {
			return nil, err
		}
	}
	return cons, retcodeError(C.SCIPaddCons(s.raw, cons))
}

func (s *Scip) createConsCardinality(vars []Variable, cardinality int, name string) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn := cString(name)
	defer freeCString(cn)
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicCardinality(s.raw, &cons, cn, 0, nil, 0, nil, nil)); err != nil {
		return nil, err
	}
	for i, v := range vars {
		if err := retcodeError(C.SCIPaddVarCardinality(s.raw, cons, v.raw, nil, C.double(float64(i)))); err != nil {
			return nil, err
		}
	}
	if err := retcodeError(C.SCIPchgCardvalCardinality(s.raw, cons, C.int(cardinality))); err != nil {
		return nil, err
	}
	return cons, retcodeError(C.SCIPaddCons(s.raw, cons))
}

func (s *Scip) createConsIndicator(binVar Variable, vars []Variable, coefs []float64, rhs float64, name string) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if len(vars) != len(coefs) {
		return nil, fmt.Errorf("variables (%d) and coefficients (%d) differ", len(vars), len(coefs))
	}
	cn := cString(name)
	defer freeCString(cn)
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicIndicator(s.raw, &cons, cn, binVar.raw,
		C.int(len(vars)), cVarSlice(vars), cDoubleSlice(coefs), C.double(rhs))); err != nil {
		return nil, err
	}
	return cons, retcodeError(C.SCIPaddCons(s.raw, cons))
}

func (s *Scip) createConsSOS1(vars []Variable, weights []float64, name string) (*C.SCIP_CONS, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if len(vars) == 0 {
		return nil, RetcodeParameterWrongVal
	}
	if weights != nil && len(vars) != len(weights) {
		return nil, RetcodeParameterWrongVal
	}
	cn := cString(name)
	defer freeCString(cn)
	if weights == nil {
		weights = make([]float64, len(vars)) // default weights
	}
	var cons *C.SCIP_CONS
	if err := retcodeError(C.SCIPcreateConsBasicSOS1(s.raw, &cons, cn,
		C.int(len(vars)), cVarSlice(vars), cDoubleSlice(weights))); err != nil {
		return nil, err
	}
	return cons, retcodeError(C.SCIPaddCons(s.raw, cons))
}

func (s *Scip) nodeGetNAddedConss(n Node) int {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return int(C.SCIPnodeGetNAddedConss(n.raw))
}

func (s *Scip) addConsCoef(cons Constraint, v Variable, coef float64) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	consTransformed := C.SCIPconsIsTransformed(cons.raw) == 1
	varTransformed := C.SCIPvarIsTransformed(v.raw) == 1

	consPtr := cons.raw
	if !consTransformed && varTransformed {
		ptr, err := s.getTransformedCons(cons)
		if err != nil {
			return err
		}
		if ptr == nil {
			return fmt.Errorf("no transformed constraint was found for the passed original constraint; " +
				"to prevent this you could disable presolving or mark the constraint to be not removable")
		}
		consPtr = ptr
	}

	varPtr := v.raw
	if consTransformed && !varTransformed {
		var transVar *C.SCIP_VAR
		if err := retcodeError(C.SCIPgetTransformedVar(s.raw, v.raw, &transVar)); err != nil {
			return err
		}
		varPtr = transVar
	}

	return retcodeError(C.SCIPaddCoefLinear(s.raw, consPtr, varPtr, C.double(coef)))
}

func (s *Scip) addConsCoefSetppc(cons Constraint, v Variable) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPaddCoefSetppc(s.raw, cons.raw, v.raw))
}

func (s *Scip) setConsModifiable(cons Constraint, modifiable bool) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsetConsModifiable(s.raw, cons.raw, cBool(modifiable)))
}

func (s *Scip) consIsModifiable(cons Constraint) bool {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPconsIsModifiable(cons.raw) == C.TRUE
}

func (s *Scip) setConsRemovable(cons Constraint, removable bool) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsetConsRemovable(s.raw, cons.raw, cBool(removable)))
}

func (s *Scip) consIsRemovable(cons Constraint) bool {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPconsIsRemovable(cons.raw) == C.TRUE
}

func (s *Scip) setConsSeparated(cons Constraint, separate bool) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsetConsSeparated(s.raw, cons.raw, cBool(separate)))
}

func (s *Scip) consIsSeparated(cons Constraint) bool {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPconsIsSeparated(cons.raw) == C.TRUE
}

// ------------------------------------------------------------- solutions

func (s *Scip) createSol(original bool, heur *C.SCIP_HEUR) (*C.SCIP_SOL, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var sol *C.SCIP_SOL
	var rc C.SCIP_RETCODE
	if original {
		rc = C.SCIPcreateOrigSol(s.raw, &sol, heur)
	} else {
		rc = C.SCIPcreateSol(s.raw, &sol, heur)
	}
	if err := retcodeError(rc); err != nil {
		return nil, err
	}
	if sol == nil {
		return nil, RetcodeError
	}
	return sol, nil
}

func (s *Scip) createPartialSol(heur *C.SCIP_HEUR) (*C.SCIP_SOL, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var sol *C.SCIP_SOL
	if err := retcodeError(C.SCIPcreatePartialSol(s.raw, &sol, heur)); err != nil {
		return nil, err
	}
	if sol == nil {
		return nil, RetcodeError
	}
	return sol, nil
}

// addSol adds a solution to the model, consuming it. Returns whether the
// solution was successfully stored.
func (s *Scip) addSol(sol *Solution) (bool, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if sol.raw == nil {
		return false, fmt.Errorf("solution is nil")
	}
	var feasible C.uint
	// Partial solutions can't be checked/tried (they have UNKNOWN entries);
	// add them directly for the completesol heuristic to complete.
	if C.SCIPsolIsPartial(sol.raw) == 1 {
		raw := sol.raw
		sol.raw = nil // SCIPaddSolFree owns it from here, even if it fails
		return s.feasibleOr(C.SCIPaddSolFree(s.raw, &raw, &feasible), raw, feasible)
	}
	if C.SCIPsolIsOriginal(sol.raw) == 1 {
		if err := retcodeError(C.SCIPcheckSolOrig(s.raw, sol.raw, &feasible, 0, 1)); err != nil {
			// The check failed (typically a panicking constraint handler);
			// we still own the solution, so consume it as promised.
			raw := sol.raw
			sol.raw = nil
			C.SCIPfreeSol(s.raw, &raw)
			return false, err
		}
		if feasible == 1 {
			raw := sol.raw
			sol.raw = nil
			return s.feasibleOr(C.SCIPaddSolFree(s.raw, &raw, &feasible), raw, feasible)
		} else {
			// Not added: we own the solution, so free it to avoid a leak.
			raw := sol.raw
			mustOK(C.SCIPfreeSol(s.raw, &raw))
			sol.raw = nil
		}
		return feasible != 0, nil
	}
	// SCIPtrySolFree takes ownership and frees the solution whether or not it
	// is stored.
	raw := sol.raw
	sol.raw = nil
	return s.feasibleOr(C.SCIPtrySolFree(s.raw, &raw, 0, 1, 1, 1, 1, &feasible), raw, feasible)
}

// feasibleOr turns a retcode plus SCIP's stored flag into addSol's result.
// The SCIP*Free variants clear raw once they have freed the solution; if they
// fail earlier (e.g. a panicking constraint handler during the check) raw is
// still ours, so free it rather than leak it.
func (s *Scip) feasibleOr(rc C.SCIP_RETCODE, raw *C.SCIP_SOL, feasible C.uint) (bool, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if err := retcodeError(rc); err != nil {
		if raw != nil {
			C.SCIPfreeSol(s.raw, &raw)
		}
		return false, err
	}
	return feasible != 0, nil
}

// ------------------------------------------------------------- LP / rows

func (s *Scip) isLPConstructed() bool {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPisLPConstructed(s.raw) != 0
}

func (s *Scip) constructLP() (bool, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var cutoff C.uint
	if err := retcodeError(C.SCIPconstructLP(s.raw, &cutoff)); err != nil {
		return false, err
	}
	return cutoff != 0, nil
}

func (s *Scip) lpStatus() LPStatus {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return lpStatusFromC(C.SCIPgetLPSolstat(s.raw))
}

func (s *Scip) lpObjVal() float64 {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return float64(C.SCIPgetLPObjval(s.raw))
}

func (s *Scip) createEmptyRow(rb *RowBuilder) (*C.SCIP_ROW, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	name := "r"
	if rb.name != nil {
		name = *rb.name
	}
	modifiable := boolOr(rb.modifiable, false)
	removable := boolOr(rb.removable, true)
	local := boolOr(rb.local, true)

	cn := cString(name)
	defer freeCString(cn)

	var rowPtr *C.SCIP_ROW
	var rc C.SCIP_RETCODE
	switch {
	case rb.source != nil && rb.source.separator != nil:
		rc = C.SCIPcreateEmptyRowSepa(s.raw, &rowPtr, rb.source.separator.raw, cn,
			C.double(rb.lhs), C.double(rb.rhs), cBool(local), cBool(modifiable), cBool(removable))
	case rb.source != nil && rb.source.constraintHandler != nil:
		rc = C.SCIPcreateEmptyRowConshdlr(s.raw, &rowPtr, rb.source.constraintHandler.raw, cn,
			C.double(rb.lhs), C.double(rb.rhs), cBool(local), cBool(modifiable), cBool(removable))
	case rb.source != nil && rb.source.constraint != nil:
		rc = C.SCIPcreateEmptyRowCons(s.raw, &rowPtr, rb.source.constraint.raw, cn,
			C.double(rb.lhs), C.double(rb.rhs), cBool(local), cBool(modifiable), cBool(removable))
	default:
		rc = C.SCIPcreateEmptyRowUnspec(s.raw, &rowPtr, cn,
			C.double(rb.lhs), C.double(rb.rhs), cBool(local), cBool(modifiable), cBool(removable))
	}
	if err := retcodeError(rc); err != nil {
		return nil, err
	}
	return rowPtr, nil
}

func (s *Scip) addRow(row Row, forceCut bool) (bool, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var infeasible C.uint
	if err := retcodeError(C.SCIPaddRow(s.raw, row.raw, cBool(forceCut), &infeasible)); err != nil {
		return false, err
	}
	// SCIPaddRow took its own capture; drop the binding's, mirroring
	// create/fill/add/release in SCIP's own separators.
	releaseOwnedRow(s, row.raw)
	return infeasible != 0, nil
}

func (s *Scip) freeTransform() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	// Rows created and never added are the binding's to release before the
	// transformed problem — and the LP holding its captures — goes away.
	relErr := releaseRowsOfOwner(s.raw)
	err := retcodeError(C.SCIPfreeTransform(s.raw))
	if err == nil {
		s.root().transGen++ // every transformed handle is now dead
		// ... and stronger than any tombstone: purge them so the map does
		// not grow one entry per released row address over the process
		// lifetime.
		purgeDeadRows(s.raw)
		err = relErr // a row that could not be released is still outstanding
	}
	return err
}

// ------------------------------------------------------------- tree nodes

func (s *Scip) focusNode() *C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPgetFocusNode(s.raw)
}

func (s *Scip) createChild() (*C.SCIP_NODE, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var nodePtr *C.SCIP_NODE
	if err := retcodeError(C.SCIPcreateChild(s.raw, &nodePtr, 0, C.SCIPgetLocalTransEstimate(s.raw))); err != nil {
		return nil, err
	}
	return nodePtr, nil
}

func (s *Scip) bestNode() *C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPgetBestNode(s.raw)
}
func (s *Scip) bestBoundNode() *C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPgetBestboundNode(s.raw)
}
func (s *Scip) bestLeaf() *C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPgetBestLeaf(s.raw)
}
func (s *Scip) bestChild() *C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPgetBestChild(s.raw)
}
func (s *Scip) bestSibling() *C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPgetBestSibling(s.raw)
}
func (s *Scip) prioChild() *C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPgetPrioChild(s.raw)
}
func (s *Scip) prioSibling() *C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return C.SCIPgetPrioSibling(s.raw)
}

func (s *Scip) nodeSlice(nodesPtr **C.SCIP_NODE, n C.int) []*C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if n <= 0 {
		return nil
	}
	out := make([]*C.SCIP_NODE, 0, int(n))
	for i := 0; i < int(n); i++ {
		out = append(out, cNodeAt(nodesPtr, i))
	}
	return out
}

func (s *Scip) leaves() []*C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var nodesPtr **C.SCIP_NODE
	var n C.int
	mustOK(C.SCIPgetLeaves(s.raw, &nodesPtr, &n))
	return s.nodeSlice(nodesPtr, n)
}

func (s *Scip) children() []*C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var nodesPtr **C.SCIP_NODE
	var n C.int
	mustOK(C.SCIPgetChildren(s.raw, &nodesPtr, &n))
	return s.nodeSlice(nodesPtr, n)
}

func (s *Scip) siblings() []*C.SCIP_NODE {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var nodesPtr **C.SCIP_NODE
	var n C.int
	mustOK(C.SCIPgetSiblings(s.raw, &nodesPtr, &n))
	return s.nodeSlice(nodesPtr, n)
}

// ------------------------------------------------------------- branching

func lpBranchingCands(scip *C.SCIP) []BranchingCandidate {
	var lpcands **C.SCIP_VAR
	var lpcandssol *C.double
	var lpcandsfrac *C.double
	var nlpcands C.int
	var nfracimplvars C.int
	C.SCIPgetLPBranchCands(scip, &lpcands, &lpcandssol, &lpcandsfrac, &nlpcands, nil, &nfracimplvars)

	cands := make([]BranchingCandidate, 0, int(nlpcands))
	for i := 0; i < int(nlpcands); i++ {
		varPtr := cVarAt(lpcands, i)
		cands = append(cands, BranchingCandidate{
			VarProbID: int(C.SCIPvarGetProbindex(varPtr)),
			LpSolVal:  float64(cAt(lpcandssol, i)),
			// SCIP's fractionality is in [0, 1) for negative values too; a
			// plain lpSolVal - trunc(lpSolVal) would be negative there.
			Frac: float64(cAt(lpcandsfrac, i)),
		})
	}
	return cands
}

func branchVarVal(scip *C.SCIP, varProbID int, val float64) error {
	v := varFromID(scip, varProbID)
	if v == nil {
		return RetcodeError
	}
	return retcodeError(C.SCIPbranchVarVal(scip, v, C.double(val), nil, nil, nil))
}

// mustBranchVarVal is branchVarVal that panics on failure (used inside
// branchrule callbacks, mirroring the Rust .unwrap()).
func mustBranchVarVal(scip *C.SCIP, varProbID int, val float64) {
	if err := branchVarVal(scip, varProbID, val); err != nil {
		panic(err)
	}
}

// ------------------------------------------------------------- plugins

func (s *Scip) includeBranchRule(name, desc string, priority, maxdepth int32, maxbounddist float64, rule BranchRule) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn, cd := cString(name), cString(desc)
	defer func() { freeCString(cn); freeCString(cd) }()
	data := plugins.put(rule, s.raw)
	return includeResult(data, C.scipgo_includeBranchrule(s.raw, cn, cd,
		C.int(priority), C.int(maxdepth), C.double(maxbounddist), cInt(isCopyable(rule)), C.uintptr_t(data)))
}

func (s *Scip) includeEventhdlr(name, desc string, eventhdlr Eventhdlr) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn, cd := cString(name), cString(desc)
	defer func() { freeCString(cn); freeCString(cd) }()
	data := plugins.put(eventhdlr, s.raw)
	return includeResult(data, C.scipgo_includeEventhdlr(s.raw, cn, cd, cInt(isCopyable(eventhdlr)), C.uintptr_t(data)))
}

func (s *Scip) includeNodesel(name, desc string, stdPriority, memSavePriority int32, nodesel Nodesel) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn, cd := cString(name), cString(desc)
	defer func() { freeCString(cn); freeCString(cd) }()
	data := plugins.put(nodesel, s.raw)
	return includeResult(data, C.scipgo_includeNodesel(s.raw, cn, cd,
		C.int(stdPriority), C.int(memSavePriority), cInt(isCopyable(nodesel)), C.uintptr_t(data)))
}

func (s *Scip) includePricer(name, desc string, priority int32, delay bool, pricer Pricer) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn, cd := cString(name), cString(desc)
	defer func() { freeCString(cn); freeCString(cd) }()
	data := plugins.put(pricer, s.raw)
	return includeResult(data, C.scipgo_includePricer(s.raw, cn, cd, C.int(priority), cInt(delay), cInt(isCopyable(pricer)), C.uintptr_t(data)))
}

func (s *Scip) includeHeur(name, desc string, priority int32, dispchar byte, freq, freqofs, maxdepth int32, timing HeurTiming, usessubscip bool, heur Heuristic) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn, cd := cString(name), cString(desc)
	defer func() { freeCString(cn); freeCString(cd) }()
	data := plugins.put(heur, s.raw)
	return includeResult(data, C.scipgo_includeHeur(s.raw, cn, cd, C.char(dispchar),
		C.int(priority), C.int(freq), C.int(freqofs), C.int(maxdepth),
		C.uint(timing), cInt(usessubscip), cInt(isCopyable(heur)), C.uintptr_t(data)))
}

func (s *Scip) includeSeparator(name, desc string, priority, freq int32, maxbounddist float64, usesubscip, delay bool, sep Separator) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn, cd := cString(name), cString(desc)
	defer func() { freeCString(cn); freeCString(cd) }()
	data := plugins.put(sep, s.raw)
	return includeResult(data, C.scipgo_includeSepa(s.raw, cn, cd, C.int(priority), C.int(freq),
		C.double(maxbounddist), cInt(usesubscip), cInt(delay), cInt(isCopyable(sep)), C.uintptr_t(data)))
}

// conshdlrOpts are the separation/propagation settings of a constraint
// handler; they only matter when the Conshdlr implements ConshdlrSepa or
// ConshdlrProp.
type conshdlrOpts struct {
	sepaFreq, sepaPriority int32
	delaySepa              bool
	propFreq               int32
	delayProp              bool
	propTiming             uint32
}

// ponytail: every node, before the LP (PySCIPOpt's defaults); add setters when someone needs root-only separation.
var defaultConshdlrOpts = conshdlrOpts{sepaFreq: 1, propFreq: 1, propTiming: C.SCIP_PROPTIMING_BEFORELP}

func (s *Scip) includeConshdlr(name, desc string, enfopriority, checkpriority int32, o conshdlrOpts, conshdlr Conshdlr) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	cn, cd := cString(name), cString(desc)
	defer func() { freeCString(cn); freeCString(cd) }()
	data := plugins.put(conshdlr, s.raw)
	_, hasEnfops := conshdlr.(ConshdlrEnfoPS)
	_, hasSepa := conshdlr.(ConshdlrSepa)
	_, hasProp := conshdlr.(ConshdlrProp)
	return includeResult(data, C.scipgo_includeConshdlr(s.raw, cn, cd,
		C.int(enfopriority), C.int(checkpriority), cInt(isCopyable(conshdlr)), cInt(hasEnfops),
		cInt(hasSepa), C.int(o.sepaFreq), C.int(o.sepaPriority), cInt(o.delaySepa),
		cInt(hasProp), C.int(o.propFreq), cInt(o.delayProp), C.uint(o.propTiming), C.uintptr_t(data)))
}

// copyPluginsTo copies every plugin of s into target (what SCIP does when it
// creates a sub-SCIP). valid reports whether all constraint handlers copied.
func (s *Scip) copyPluginsTo(target *Scip) (valid bool, err error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var v C.uint
	err = retcodeError(C.scipgo_copyPlugins(s.raw, target.raw, &v))
	return v != 0, err
}

// ------------------------------------------------------------- helpers

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// cVarSlice returns a C array of SCIP_VAR* for passing to SCIP functions.
// The backing Go slice contains only C pointers, which is valid cgo usage.
// Returns nil for an empty input.
func cVarSlice(vars []Variable) **C.SCIP_VAR {
	if len(vars) == 0 {
		return nil
	}
	out := make([]*C.SCIP_VAR, len(vars))
	for i, v := range vars {
		out[i] = v.raw
	}
	return &out[0]
}

func cDoubleSlice(vals []float64) *C.double {
	if len(vals) == 0 {
		return nil
	}
	out := make([]C.double, len(vals))
	for i, v := range vals {
		out[i] = C.double(v)
	}
	return &out[0]
}
