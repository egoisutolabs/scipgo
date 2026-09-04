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

// alive reports whether the instance behind s still exists. A weak wrapper
// is alive while its owner is and, for a sub-SCIP copy, while SCIP has not
// freed that copy (its plugins' free callbacks drop it from copyParents).
func (s *Scip) alive() bool {
	r := s.root()
	if r == nil || r.raw == nil || r.freed.Load() {
		return false
	}
	if s.weak && s.raw != r.raw && copyIncarnation(s.raw) != s.copyInc {
		return false // the sub-SCIP this wrapper was minted in is gone
	}
	return true
}

// gen is the generation a handle must carry to be valid: the problem
// generation for original-problem objects, the transform generation for
// everything else.
func (s *Scip) gen(orig bool) uint64 {
	if orig {
		return s.root().probGen
	}
	return s.root().transGen
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
	forgetCopy(scipPtr) // the address may have belonged to a freed sub-SCIP
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

	if err := s.scipFree(raw); err != nil && firstErr == nil {
		firstErr = err
	}
	// Drop panics stashed by plugin free callbacks: the raw pointer may be
	// reused by a later SCIPcreate, which would otherwise rethrow them.
	if ps := takePanics(s.raw); len(ps) > 0 && firstErr == nil {
		firstErr = fmt.Errorf("panic in plugin free callback: %v", ps[0])
	}
	deleteDatastore(s)
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
	return retcodeError(C.SCIPincludeDefaultPlugins(s.raw))
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

func (s *Scip) solve() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	return retcodeError(C.SCIPsolve(s.raw))
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
	sync.Mutex
	live bool
}

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

func (s *Scip) solveConcurrent() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	tpiPool.Lock()
	defer tpiPool.Unlock()
	if s.holdsTPI() {
		// Re-solve: SCIP reuses its solvers and expects the pool to exist.
		if !tpiPool.live {
			n, _ := s.intParam("parallel/maxnthreads") // an upper bound on the solver count
			if err := tpiInit(max(n, 1)); err != nil {
				return err
			}
		}
	} else if tpiPool.live {
		// SCIP is about to create a pool; drop the one another instance left.
		if err := retcodeError(C.SCIPtpiExit()); err != nil {
			return err
		}
		tpiPool.live = false
	}
	err := retcodeError(C.SCIPsolveConcurrent(s.raw))
	tpiPool.live = tpiPool.live || s.holdsTPI()
	return err
}

// scipFree calls SCIPfree, giving it a thread pool to destroy if this
// instance expects one and another instance already destroyed it.
func (s *Scip) scipFree(raw *C.SCIP) error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if !s.holdsTPI() {
		return retcodeError(C.SCIPfree(&raw))
	}
	tpiPool.Lock()
	defer tpiPool.Unlock()
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
	scipSols := C.SCIPgetSols(s.raw)
	out := make([]*C.SCIP_SOL, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, cSolAt(scipSols, i))
	}
	return out
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

func (s *Scip) createSol(original bool) (*C.SCIP_SOL, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var sol *C.SCIP_SOL
	var rc C.SCIP_RETCODE
	if original {
		rc = C.SCIPcreateOrigSol(s.raw, &sol, nil)
	} else {
		rc = C.SCIPcreateSol(s.raw, &sol, nil)
	}
	if err := retcodeError(rc); err != nil {
		return nil, err
	}
	if sol == nil {
		return nil, RetcodeError
	}
	return sol, nil
}

func (s *Scip) createPartialSol() (*C.SCIP_SOL, error) {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	var sol *C.SCIP_SOL
	if err := retcodeError(C.SCIPcreatePartialSol(s.raw, &sol, nil)); err != nil {
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
	return infeasible != 0, nil
}

func (s *Scip) freeTransform() error {
	defer runtime.KeepAlive(s.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	err := retcodeError(C.SCIPfreeTransform(s.raw))
	if err == nil {
		s.root().transGen++ // every transformed handle is now dead
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
	var nlpcands C.int
	var nfracimplvars C.int
	C.SCIPgetLPBranchCands(scip, &lpcands, &lpcandssol, nil, &nlpcands, nil, &nfracimplvars)

	cands := make([]BranchingCandidate, 0, int(nlpcands))
	for i := 0; i < int(nlpcands); i++ {
		varPtr := cVarAt(lpcands, i)
		lpSolVal := float64(cAt(lpcandssol, i))
		cands = append(cands, BranchingCandidate{
			VarProbID: int(C.SCIPvarGetProbindex(varPtr)),
			LpSolVal:  lpSolVal,
			Frac:      lpSolVal - math.Trunc(lpSolVal),
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

func (s *Scip) includeNodesel(name, desc string, stdPriority, memSavePriority int32, nodesel NodeSel) error {
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
