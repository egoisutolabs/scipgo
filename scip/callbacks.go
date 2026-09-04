package scip

/*
#include "helpers.h"
*/
import "C"

import (
	"fmt"
	"os"
	"reflect"
	"sync"
)

// pluginRegistry keeps Go plugin implementations alive and addressable from
// C callback trampolines. Plugin implementations cannot be stored in C memory
// (cgo pointer rules), so each plugin gets a numeric id which is stored in the
// SCIP plugin data pointer instead.
type pluginRegistry struct {
	mu    sync.Mutex
	next  uintptr
	items map[uintptr]pluginEntry
}

// pluginEntry records which SCIP instance a plugin was included into, so a
// copy made into a sub-SCIP can be traced back to it.
type pluginEntry struct {
	item any
	scip *C.SCIP
}

var plugins = &pluginRegistry{items: make(map[uintptr]pluginEntry)}

// put registers a plugin and returns its id. C stores the id in the SCIP
// plugin-data pointer slot; on the Go side it must stay a uintptr, since the
// GC rejects a small integer found in a pointer-typed value.
func (r *pluginRegistry) put(item any, scip *C.SCIP) uintptr {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	r.items[r.next] = pluginEntry{item: item, scip: scip}
	return r.next
}

func (r *pluginRegistry) get(id uintptr) any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.items[id].item
}

func (r *pluginRegistry) owner(id uintptr) *C.SCIP {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.items[id].scip
}

// Copyable is implemented by plugins that may be copied into the sub-SCIPs
// SCIP creates for LNS heuristics and for SolveConcurrent workers. Copy
// returns the plugin to include in the copy: the receiver itself if it holds
// no per-instance state, or a fresh object. Plugins without Copy never run
// inside sub-SCIPs; for a Conshdlr that also marks every sub-SCIP copy as
// invalid, which disables SCIP's sub-MIP heuristics.
//
// A copy running in a SolveConcurrent worker is called from that worker's
// thread, concurrently with the other copies.
type Copyable interface {
	Copy() any
}

func isCopyable(plugin any) bool {
	_, ok := plugin.(Copyable)
	return ok
}

// copyParents maps a sub-SCIP that received a plugin copy to the Go-owned
// instance the original plugin belongs to, so panics and datastore lookups
// from inside the copy resolve to the instance the user holds.
var copyParents = struct {
	sync.Mutex
	m map[*C.SCIP]*C.SCIP
}{m: make(map[*C.SCIP]*C.SCIP)}

func rootScip(scip *C.SCIP) *C.SCIP {
	copyParents.Lock()
	defer copyParents.Unlock()
	if root, ok := copyParents.m[scip]; ok {
		return root
	}
	return scip
}

func setCopyParent(target, source *C.SCIP) {
	root := rootScip(source)
	copyParents.Lock()
	defer copyParents.Unlock()
	copyParents.m[target] = root
}

func forgetCopy(scip *C.SCIP) {
	copyParents.Lock()
	defer copyParents.Unlock()
	delete(copyParents.m, scip)
}

// pluginCopy resolves the Go plugin behind source plugin data, records target
// as a copy of its instance, and returns the object to include in target.
func pluginCopy[T any](target *C.SCIP, id uintptr) (T, bool) {
	var zero T
	setCopyParent(target, plugins.owner(id))
	item := plugins.get(id)
	c, ok := item.(Copyable)
	if !ok {
		stashPanic(target, "", fmt.Sprintf("scip: plugin %T is not Copyable", item))
		return zero, false
	}
	cp := c.Copy()
	t, ok := cp.(T)
	if !ok {
		stashPanic(target, "", fmt.Sprintf("scip: %T.Copy returned %T, want %v",
			item, cp, reflect.TypeOf((*T)(nil)).Elem()))
	}
	return t, ok
}

// errToC turns an include error back into the retcode for the C caller.
func errToC(err error) C.SCIP_RETCODE {
	if rc, ok := err.(Retcode); ok {
		return retcodeToC(rc)
	}
	return C.SCIP_ERROR
}

func (r *pluginRegistry) del(id uintptr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
}

// pluginAs fetches the Go plugin behind a SCIP plugin data pointer and
// asserts its type, stashing a panic on mismatch.
func pluginAs[T any](scip *C.SCIP, id uintptr) (T, bool) {
	item := plugins.get(id)
	t, ok := item.(T)
	if !ok {
		stashPanic(scip, "", fmt.Sprintf("scip: plugin data is %T, want %v",
			item, reflect.TypeOf((*T)(nil)).Elem()))
	}
	return t, ok
}

// panicStash stores panics raised inside SCIP callbacks. Panics cannot unwind
// through C frames; instead they are recovered here, the callback reports an
// error retcode, and the top-level call (Solve, FreeTransform, AddSol)
// returns them as a *CallbackPanic.
type panicStore struct {
	mu   sync.Mutex
	vals map[*C.SCIP][]*CallbackPanic
}

var stashedPanics = &panicStore{vals: make(map[*C.SCIP][]*CallbackPanic)}

func stashPanic(scip *C.SCIP, plugin string, r any) {
	stashedPanics.mu.Lock()
	defer stashedPanics.mu.Unlock()
	if scip == nil {
		panic(r)
	}
	scip = rootScip(scip) // a sub-SCIP's panics surface on the model the user solves
	stashedPanics.vals[scip] = append(stashedPanics.vals[scip], &CallbackPanic{Plugin: plugin, Value: r})
}

// catchPanic is deferred at the top of every exported callback to recover and
// stash any panic before control returns into C. kind and id identify the
// plugin; the label is only built when a panic actually happened.
func catchPanic(scip *C.SCIP, kind string, id uintptr) {
	if r := recover(); r != nil {
		stashPanic(scip, pluginLabel(kind, id), r)
	}
}

func pluginLabel(kind string, id uintptr) string {
	if item := plugins.get(id); item != nil {
		return fmt.Sprintf("%s %T", kind, item)
	}
	return kind
}

// takePanics returns and clears any stashed panics for the given instance.
func takePanics(scip *C.SCIP) []*CallbackPanic {
	stashedPanics.mu.Lock()
	defer stashedPanics.mu.Unlock()
	v := stashedPanics.vals[scip]
	delete(stashedPanics.vals, scip)
	return v
}

// callbackError returns the panics stashed during the last top-level call as
// one *CallbackPanic (further ones in More), or nil.
func callbackError(scip *C.SCIP) error {
	ps := takePanics(scip)
	if len(ps) == 0 {
		return nil
	}
	first := ps[0]
	first.More = ps[1:]
	return first
}

// includeResult converts an include retcode, dropping the registry entry on
// failure so the plugin object is not pinned forever.
func includeResult(id uintptr, rc C.SCIP_RETCODE) error {
	err := retcodeError(rc)
	if err != nil {
		plugins.del(id)
	}
	return err
}

// weakScip wraps a raw SCIP pointer without taking ownership, as needed
// inside plugin callbacks.
func weakScip(scip *C.SCIP) *Scip { return &Scip{raw: scip, weak: true} }

// solvingModel wraps a raw SCIP pointer into a Model in the Solving stage, as
// passed to plugin callbacks.
func solvingModel(scip *C.SCIP) Model {
	return Model{scip: weakScip(scip)}
}

// ---------------------------------------------------------------- branchrule

//export GoBranchFree
func GoBranchFree(scip *C.SCIP, branchrule *C.SCIP_BRANCHRULE) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "branchrule", uintptr(C.scipgo_branchruleId(branchrule)))
	plugins.del(uintptr(C.scipgo_branchruleId(branchrule)))
	forgetCopy(scip)
	ret = C.SCIP_OKAY
	return
}

//export GoBranchCopy
func GoBranchCopy(scip *C.SCIP, branchrule *C.SCIP_BRANCHRULE) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "branchrule", uintptr(C.scipgo_branchruleId(branchrule)))
	rule, ok := pluginCopy[BranchRule](scip, uintptr(C.scipgo_branchruleId(branchrule)))
	if !ok {
		return
	}
	if err := weakScip(scip).includeBranchRule(
		goString(C.SCIPbranchruleGetName(branchrule)), goString(C.SCIPbranchruleGetDesc(branchrule)),
		int32(C.SCIPbranchruleGetPriority(branchrule)), int32(C.SCIPbranchruleGetMaxdepth(branchrule)),
		float64(C.SCIPbranchruleGetMaxbounddist(branchrule)), rule); err != nil {
		return errToC(err)
	}
	ret = C.SCIP_OKAY
	return
}

//export GoBranchExecLp
func GoBranchExecLp(scip *C.SCIP, branchrule *C.SCIP_BRANCHRULE, allowaddcons C.uint, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "branchrule", uintptr(C.scipgo_branchruleId(branchrule)))
	rule, ok := pluginAs[BranchRule](scip, uintptr(C.scipgo_branchruleId(branchrule)))
	if !ok {
		return
	}

	cands := lpBranchingCands(scip)
	model := solvingModel(scip)
	br := BranchRulePlugin{raw: branchrule}
	res := rule.Execute(model, br, cands)

	if res.Kind == BranchingResultBranchOn {
		mustBranchVarVal(scip, res.Candidate.VarProbID, res.Candidate.LpSolVal)
	}
	if res.Kind == BranchingResultCustomBranching && C.SCIPgetNChildren(scip) <= 0 {
		stashPanic(scip, "", "custom branching rule must create at least one child node")
		return
	}

	*result = branchResultToC(res)
	ret = C.SCIP_OKAY
	return
}

// ---------------------------------------------------------------- eventhdlr

//export GoEventhdlrFree
func GoEventhdlrFree(scip *C.SCIP, eventhdlr *C.SCIP_EVENTHDLR) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "eventhdlr", uintptr(C.scipgo_eventhdlrId(eventhdlr)))
	plugins.del(uintptr(C.scipgo_eventhdlrId(eventhdlr)))
	forgetCopy(scip)
	ret = C.SCIP_OKAY
	return
}

//export GoEventhdlrCopy
func GoEventhdlrCopy(scip *C.SCIP, eventhdlr *C.SCIP_EVENTHDLR) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "eventhdlr", uintptr(C.scipgo_eventhdlrId(eventhdlr)))
	eh, ok := pluginCopy[Eventhdlr](scip, uintptr(C.scipgo_eventhdlrId(eventhdlr)))
	if !ok {
		return
	}
	// SCIP keeps no getter for an event handler's description.
	if err := weakScip(scip).includeEventhdlr(goString(C.SCIPeventhdlrGetName(eventhdlr)), "", eh); err != nil {
		return errToC(err)
	}
	ret = C.SCIP_OKAY
	return
}

//export GoEventhdlrInit
func GoEventhdlrInit(scip *C.SCIP, eventhdlr *C.SCIP_EVENTHDLR) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "eventhdlr", uintptr(C.scipgo_eventhdlrId(eventhdlr)))
	hdlr, ok := pluginAs[Eventhdlr](scip, uintptr(C.scipgo_eventhdlrId(eventhdlr)))
	if !ok {
		return
	}
	ret = C.SCIPcatchEvent(scip, C.SCIP_EVENTTYPE(hdlr.GetEventMask()), eventhdlr, nil, nil)
	return
}

//export GoEventhdlrExec
func GoEventhdlrExec(scip *C.SCIP, eventhdlr *C.SCIP_EVENTHDLR, event *C.SCIP_EVENT, eventdata *C.SCIP_EVENTDATA) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "eventhdlr", uintptr(C.scipgo_eventhdlrId(eventhdlr)))
	hdlr, ok := pluginAs[Eventhdlr](scip, uintptr(C.scipgo_eventhdlrId(eventhdlr)))
	if !ok {
		return
	}
	s := weakScip(scip)
	model := Model{scip: s}
	sh := EventhdlrPlugin{raw: eventhdlr}
	ev := Event{raw: event, scip: s}
	hdlr.Execute(model, sh, ev)
	ret = C.SCIP_OKAY
	return
}

// ------------------------------------------------------------------ nodesel

//export GoNodeselFree
func GoNodeselFree(scip *C.SCIP, nodesel *C.SCIP_NODESEL) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "nodesel", uintptr(C.scipgo_nodeselId(nodesel)))
	plugins.del(uintptr(C.scipgo_nodeselId(nodesel)))
	forgetCopy(scip)
	ret = C.SCIP_OKAY
	return
}

//export GoNodeselCopy
func GoNodeselCopy(scip *C.SCIP, nodesel *C.SCIP_NODESEL) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "nodesel", uintptr(C.scipgo_nodeselId(nodesel)))
	ns, ok := pluginCopy[NodeSel](scip, uintptr(C.scipgo_nodeselId(nodesel)))
	if !ok {
		return
	}
	if err := weakScip(scip).includeNodesel(
		goString(C.SCIPnodeselGetName(nodesel)), goString(C.SCIPnodeselGetDesc(nodesel)),
		int32(C.SCIPnodeselGetStdPriority(nodesel)), int32(C.SCIPnodeselGetMemsavePriority(nodesel)), ns); err != nil {
		return errToC(err)
	}
	ret = C.SCIP_OKAY
	return
}

//export GoNodeselSelect
func GoNodeselSelect(scip *C.SCIP, nodesel *C.SCIP_NODESEL, selnode **C.SCIP_NODE) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "nodesel", uintptr(C.scipgo_nodeselId(nodesel)))
	sel, ok := pluginAs[NodeSel](scip, uintptr(C.scipgo_nodeselId(nodesel)))
	if !ok {
		return
	}
	model := solvingModel(scip)
	node := sel.Select(model)
	if node != nil {
		*selnode = node.raw
	} else {
		*selnode = nil
	}
	ret = C.SCIP_OKAY
	return
}

//export GoNodeselComp
func GoNodeselComp(scip *C.SCIP, nodesel *C.SCIP_NODESEL, node1, node2 *C.SCIP_NODE) (ret C.int) {
	defer catchPanic(scip, "nodesel", uintptr(C.scipgo_nodeselId(nodesel)))
	sel, ok := pluginAs[NodeSel](scip, uintptr(C.scipgo_nodeselId(nodesel)))
	if !ok {
		return 0
	}
	s := weakScip(scip)
	ret = C.int(sel.Comp(Node{raw: node1, scip: s}, Node{raw: node2, scip: s}))
	return
}

// ------------------------------------------------------------------- pricer

func callPricer(scip *C.SCIP, pricer *C.SCIP_PRICER, lowerbound *C.double, stopearly *C.uint, result *C.SCIP_RESULT, farkas bool) C.SCIP_RETCODE {
	p, ok := pluginAs[Pricer](scip, uintptr(C.scipgo_pricerId(pricer)))
	if !ok {
		return C.SCIP_ERROR
	}

	nVarsBefore := C.SCIPgetNVars(scip)

	model := solvingModel(scip)
	res := p.GenerateColumns(model, PricerPlugin{raw: pricer}, farkas)

	if !farkas {
		if res.LowerBound != nil {
			*lowerbound = C.double(*res.LowerBound)
		}
		if res.State == PricerResultStateStopEarly {
			*stopearly = 1
		}
	}

	if farkas && res.State == PricerResultStateStopEarly {
		stashPanic(scip, "pricer", "farkas pricing should never stop early as LP would remain infeasible")
		return C.SCIP_ERROR
	}

	if res.State == PricerResultStateFoundColumns {
		if nVarsBefore >= C.SCIPgetNVars(scip) {
			stashPanic(scip, "pricer", "pricer reported FoundColumns but added no variables")
			return C.SCIP_ERROR
		}
	}

	*result = pricerStateToC(res.State)
	return C.SCIP_OKAY
}

//export GoPricerFree
func GoPricerFree(scip *C.SCIP, pricer *C.SCIP_PRICER) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "pricer", uintptr(C.scipgo_pricerId(pricer)))
	plugins.del(uintptr(C.scipgo_pricerId(pricer)))
	forgetCopy(scip)
	ret = C.SCIP_OKAY
	return
}

//export GoPricerCopy
func GoPricerCopy(scip *C.SCIP, pricer *C.SCIP_PRICER, valid *C.uint) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "pricer", uintptr(C.scipgo_pricerId(pricer)))
	p, ok := pluginCopy[Pricer](scip, uintptr(C.scipgo_pricerId(pricer)))
	if !ok {
		return
	}
	if err := weakScip(scip).includePricer(
		goString(C.SCIPpricerGetName(pricer)), goString(C.SCIPpricerGetDesc(pricer)),
		int32(C.SCIPpricerGetPriority(pricer)), C.SCIPpricerIsDelayed(pricer) != 0, p); err != nil {
		return errToC(err)
	}
	*valid = 1
	ret = C.SCIP_OKAY
	return
}

//export GoPricerRedcost
func GoPricerRedcost(scip *C.SCIP, pricer *C.SCIP_PRICER, lowerbound *C.double, stopearly *C.uint, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "pricer", uintptr(C.scipgo_pricerId(pricer)))
	return callPricer(scip, pricer, lowerbound, stopearly, result, false)
}

//export GoPricerFarkas
func GoPricerFarkas(scip *C.SCIP, pricer *C.SCIP_PRICER, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "pricer", uintptr(C.scipgo_pricerId(pricer)))
	return callPricer(scip, pricer, nil, nil, result, true)
}

// ---------------------------------------------------------------- heuristic

//export GoHeurFree
func GoHeurFree(scip *C.SCIP, heur *C.SCIP_HEUR) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "heuristic", uintptr(C.scipgo_heurId(heur)))
	plugins.del(uintptr(C.scipgo_heurId(heur)))
	forgetCopy(scip)
	ret = C.SCIP_OKAY
	return
}

//export GoHeurCopy
func GoHeurCopy(scip *C.SCIP, heur *C.SCIP_HEUR) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "heuristic", uintptr(C.scipgo_heurId(heur)))
	h, ok := pluginCopy[Heuristic](scip, uintptr(C.scipgo_heurId(heur)))
	if !ok {
		return
	}
	if err := weakScip(scip).includeHeur(
		goString(C.SCIPheurGetName(heur)), goString(C.SCIPheurGetDesc(heur)),
		int32(C.SCIPheurGetPriority(heur)), byte(C.SCIPheurGetDispchar(heur)),
		int32(C.SCIPheurGetFreq(heur)), int32(C.SCIPheurGetFreqofs(heur)), int32(C.SCIPheurGetMaxdepth(heur)),
		HeurTiming(C.SCIPheurGetTimingmask(heur)), C.SCIPheurUsesSubscip(heur) != 0, h); err != nil {
		return errToC(err)
	}
	ret = C.SCIP_OKAY
	return
}

//export GoHeurExec
func GoHeurExec(scip *C.SCIP, heur *C.SCIP_HEUR, heurtiming C.SCIP_HEURTIMING, nodeinfeasible C.uint, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "heuristic", uintptr(C.scipgo_heurId(heur)))
	h, ok := pluginAs[Heuristic](scip, uintptr(C.scipgo_heurId(heur)))
	if !ok {
		return
	}

	currentNSols := C.SCIPgetNSols(scip)
	model := solvingModel(scip)
	heurRes := h.Execute(model, heurTimingFromC(uint32(heurtiming)), nodeinfeasible != 0)
	if heurRes == HeurResultFoundSol {
		newNSols := C.SCIPgetNSols(scip)
		if newNSols <= currentNSols {
			fmt.Fprintf(os.Stderr, "Heuristic %s returned result %v, but no solutions were added\n",
				goString(C.SCIPheurGetName(heur)), heurRes)
			return C.SCIP_ERROR
		}
	}

	*result = heurResultToC(heurRes)
	ret = C.SCIP_OKAY
	return
}

// ---------------------------------------------------------------- separator

//export GoSepaFree
func GoSepaFree(scip *C.SCIP, sepa *C.SCIP_SEPA) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "separator", uintptr(C.scipgo_sepaId(sepa)))
	plugins.del(uintptr(C.scipgo_sepaId(sepa)))
	forgetCopy(scip)
	ret = C.SCIP_OKAY
	return
}

//export GoSepaCopy
func GoSepaCopy(scip *C.SCIP, sepa *C.SCIP_SEPA) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "separator", uintptr(C.scipgo_sepaId(sepa)))
	sp, ok := pluginCopy[Separator](scip, uintptr(C.scipgo_sepaId(sepa)))
	if !ok {
		return
	}
	if err := weakScip(scip).includeSeparator(
		goString(C.SCIPsepaGetName(sepa)), goString(C.SCIPsepaGetDesc(sepa)),
		int32(C.SCIPsepaGetPriority(sepa)), int32(C.SCIPsepaGetFreq(sepa)),
		float64(C.SCIPsepaGetMaxbounddist(sepa)), C.SCIPsepaUsesSubscip(sepa) != 0,
		C.SCIPsepaIsDelayed(sepa) != 0, sp); err != nil {
		return errToC(err)
	}
	ret = C.SCIP_OKAY
	return
}

//export GoSepaExecLp
func GoSepaExecLp(scip *C.SCIP, sepa *C.SCIP_SEPA, result *C.SCIP_RESULT, allowlocal C.uint, depth C.int) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "separator", uintptr(C.scipgo_sepaId(sepa)))
	s, ok := pluginAs[Separator](scip, uintptr(C.scipgo_sepaId(sepa)))
	if !ok {
		return
	}

	model := solvingModel(scip)
	sepRes := s.ExecuteLP(model, SeparatorPlugin{raw: sepa})
	*result = separationResultToC(sepRes)
	ret = C.SCIP_OKAY
	return
}

//export GoSepaExecSol
func GoSepaExecSol(scip *C.SCIP, sepa *C.SCIP_SEPA, sol *C.SCIP_SOL, result *C.SCIP_RESULT, allowlocal C.uint, depth C.int) (ret C.SCIP_RETCODE) {
	return C.SCIP_OKAY
}

// ---------------------------------------------------------------- conshdlr

//export GoConsFree
func GoConsFree(scip *C.SCIP, conshdlr *C.SCIP_CONSHDLR) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "conshdlr", uintptr(C.scipgo_conshdlrId(conshdlr)))
	plugins.del(uintptr(C.scipgo_conshdlrId(conshdlr)))
	forgetCopy(scip)
	ret = C.SCIP_OKAY
	return
}

//export GoConsCopy
func GoConsCopy(scip *C.SCIP, conshdlr *C.SCIP_CONSHDLR, valid *C.uint) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "conshdlr", uintptr(C.scipgo_conshdlrId(conshdlr)))
	c, ok := pluginCopy[Conshdlr](scip, uintptr(C.scipgo_conshdlrId(conshdlr)))
	if !ok {
		return
	}
	opts := conshdlrOpts{
		sepaFreq:     int32(C.SCIPconshdlrGetSepaFreq(conshdlr)),
		sepaPriority: int32(C.SCIPconshdlrGetSepaPriority(conshdlr)),
		delaySepa:    C.SCIPconshdlrIsSeparationDelayed(conshdlr) != 0,
		propFreq:     int32(C.SCIPconshdlrGetPropFreq(conshdlr)),
		delayProp:    C.SCIPconshdlrIsPropagationDelayed(conshdlr) != 0,
		propTiming:   uint32(C.SCIPconshdlrGetPropTiming(conshdlr)),
	}
	if err := weakScip(scip).includeConshdlr(
		goString(C.SCIPconshdlrGetName(conshdlr)), goString(C.SCIPconshdlrGetDesc(conshdlr)),
		int32(C.SCIPconshdlrGetEnfoPriority(conshdlr)), int32(C.SCIPconshdlrGetCheckPriority(conshdlr)),
		opts, c); err != nil {
		return errToC(err)
	}
	*valid = 1
	ret = C.SCIP_OKAY
	return
}

//export GoConsEnfops
func GoConsEnfops(scip *C.SCIP, conshdlr *C.SCIP_CONSHDLR, conss **C.SCIP_CONS, nconss C.int, nusefulconss C.int, solinfeasible C.uint, objinfeasible C.uint, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "conshdlr", uintptr(C.scipgo_conshdlrId(conshdlr)))
	c, ok := pluginAs[ConshdlrEnfoPS](scip, uintptr(C.scipgo_conshdlrId(conshdlr)))
	if !ok {
		return
	}
	*result = conshdlrResultToC(c.EnforcePseudo(solvingModel(scip), ConshdlrPlugin{raw: conshdlr},
		solinfeasible != 0, objinfeasible != 0))
	ret = C.SCIP_OKAY
	return
}

//export GoConsSepalp
func GoConsSepalp(scip *C.SCIP, conshdlr *C.SCIP_CONSHDLR, conss **C.SCIP_CONS, nconss C.int, nusefulconss C.int, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "conshdlr", uintptr(C.scipgo_conshdlrId(conshdlr)))
	c, ok := pluginAs[ConshdlrSepa](scip, uintptr(C.scipgo_conshdlrId(conshdlr)))
	if !ok {
		return
	}
	*result = separationResultToC(c.SeparateLP(solvingModel(scip), ConshdlrPlugin{raw: conshdlr}))
	ret = C.SCIP_OKAY
	return
}

//export GoConsProp
func GoConsProp(scip *C.SCIP, conshdlr *C.SCIP_CONSHDLR, conss **C.SCIP_CONS, nconss C.int, nusefulconss C.int, nmarkedconss C.int, proptiming C.uint, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "conshdlr", uintptr(C.scipgo_conshdlrId(conshdlr)))
	c, ok := pluginAs[ConshdlrProp](scip, uintptr(C.scipgo_conshdlrId(conshdlr)))
	if !ok {
		return
	}
	*result = propResultToC(c.Propagate(solvingModel(scip), ConshdlrPlugin{raw: conshdlr}))
	ret = C.SCIP_OKAY
	return
}

//export GoConsEnfolp
func GoConsEnfolp(scip *C.SCIP, conshdlr *C.SCIP_CONSHDLR, conss **C.SCIP_CONS, nconss C.int, nusefulconss C.int, solinfeasible C.uint, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "conshdlr", uintptr(C.scipgo_conshdlrId(conshdlr)))
	c, ok := pluginAs[Conshdlr](scip, uintptr(C.scipgo_conshdlrId(conshdlr)))
	if !ok {
		return
	}
	model := solvingModel(scip)
	*result = conshdlrResultToC(c.Enforce(model, ConshdlrPlugin{raw: conshdlr}))
	ret = C.SCIP_OKAY
	return
}

//export GoConsCheck
func GoConsCheck(scip *C.SCIP, conshdlr *C.SCIP_CONSHDLR, conss **C.SCIP_CONS, nconss C.int, sol *C.SCIP_SOL, checkintegrality C.uint, checklprows C.uint, printreason C.uint, completely C.uint, result *C.SCIP_RESULT) (ret C.SCIP_RETCODE) {
	ret = C.SCIP_ERROR
	defer catchPanic(scip, "conshdlr", uintptr(C.scipgo_conshdlrId(conshdlr)))
	c, ok := pluginAs[Conshdlr](scip, uintptr(C.scipgo_conshdlrId(conshdlr)))
	if !ok {
		return
	}
	s := weakScip(scip)
	model := Model{scip: s}
	solution := Solution{raw: sol, scip: s}

	feasible := c.Check(model, ConshdlrPlugin{raw: conshdlr}, solution)
	if feasible {
		*result = C.SCIP_FEASIBLE
	} else {
		*result = C.SCIP_INFEASIBLE
	}
	ret = C.SCIP_OKAY
	return
}

//export GoConsLock
func GoConsLock(scipPtr *C.SCIP, conshdlr *C.SCIP_CONSHDLR, cons *C.SCIP_CONS, locktype C.SCIP_LOCKTYPE, nlockspos C.int, nlocksneg C.int) (ret C.SCIP_RETCODE) {
	return C.SCIP_OKAY
}
