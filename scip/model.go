package scip

/*
#include "helpers.h"
*/
import "C"

import "fmt"

// Infinity is SCIP's notion of positive infinity.
const Infinity = 1e+20

// NegInfinity is SCIP's notion of negative infinity.
const NegInfinity = -1e+20

// scipInvalid mirrors SCIP's SCIP_INVALID macro (1e99).
const scipInvalid = 1e99

// Model represents an optimization model backed by a SCIP instance.
//
// Unlike russcip's Model<State> typestate, a single Go type is used for all
// lifecycle stages; Model.Stage reports the current one. Every operation
// comes in two forms: a Try* method that returns an error (*Error for a SCIP
// failure, *CallbackPanic for a panic inside a plugin callback), and a plain
// method that panics with that same error value. Use Try* in services and the
// plain form in scripts and tests.
//
// Typical lifecycle:
//
//	model := scip.NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("problem")
//	x := model.AddVar(0, 1, 1, "x", scip.VarTypeBinary)
//	solved := model.Solve()
type Model struct {
	scip *Scip
}

// must panics with err if it is non-nil; every panicking method is this over
// its Try* sibling.
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// NewModel creates a new Model instance. It panics if the SCIP instance
// cannot be created.
func NewModel() Model {
	m, err := TryNewModel()
	must(err)
	return m
}

// TryNewModel tries to create a new Model instance.
func TryNewModel() (Model, error) {
	s, err := newScip()
	if err != nil {
		return Model{}, &Error{Op: "NewModel", Stage: StageInit, Retcode: err.(Retcode)}
	}
	return Model{scip: s}, nil
}

// DefaultModel creates a Model with default plugins included and a problem
// named "problem", mirroring Rust's Model::default().
func DefaultModel() Model {
	return NewModel().IncludeDefaultPlugins().CreateProb("problem")
}

// MinimalModel creates a Model with presolving, heuristics and separating
// turned off, useful for writing tests.
func MinimalModel() Model {
	return DefaultModel().
		SetPresolving(ParamSettingOff).
		SetHeuristics(ParamSettingOff).
		SetSeparating(ParamSettingOff)
}

// ------------------------------------------------------------- lifecycle

// TryIncludeDefaultPlugins includes all of SCIP's default plugins.
func (m Model) TryIncludeDefaultPlugins() (Model, error) {
	if err := m.guard("IncludeDefaultPlugins"); err != nil {
		return m, err
	}
	return m, m.wrap("IncludeDefaultPlugins", m.scip.includeDefaultPlugins(), "")
}

// IncludeDefaultPlugins includes all of SCIP's default plugins. It panics on
// failure.
func (m Model) IncludeDefaultPlugins() Model {
	m, err := m.TryIncludeDefaultPlugins()
	must(err)
	return m
}

// TryCreateProb creates a new problem with the given name.
func (m Model) TryCreateProb(name string) (Model, error) {
	if err := m.guard("CreateProb"); err != nil {
		return m, err
	}
	return m, m.wrap("CreateProb", m.scip.createProb(name), name)
}

// CreateProb creates a new problem with the given name. It panics on failure.
func (m Model) CreateProb(name string) Model {
	m, err := m.TryCreateProb(name)
	must(err)
	return m
}

// ReadProb reads a problem from the given file. On failure the model is
// left untouched but the zero Model is returned, matching russcip.
func (m Model) ReadProb(filename string) (Model, error) {
	if err := m.guard("ReadProb"); err != nil {
		return Model{}, err
	}
	if err := m.wrap("ReadProb", m.scip.readProb(filename), filename); err != nil {
		return Model{}, err
	}
	return m, nil
}

// TrySetObjSense sets the objective sense.
func (m Model) TrySetObjSense(sense ObjSense) (Model, error) {
	if err := m.guard("SetObjSense"); err != nil {
		return m, err
	}
	return m, m.wrap("SetObjSense", m.scip.setObjSense(sense), "")
}

// SetObjSense sets the objective sense. It panics on failure.
func (m Model) SetObjSense(sense ObjSense) Model {
	m, err := m.TrySetObjSense(sense)
	must(err)
	return m
}

// TryMaximize sets the objective sense to maximize.
func (m Model) TryMaximize() (Model, error) { return m.TrySetObjSense(ObjSenseMaximize) }

// TryMinimize sets the objective sense to minimize.
func (m Model) TryMinimize() (Model, error) { return m.TrySetObjSense(ObjSenseMinimize) }

// Maximize sets the objective sense to maximize. It panics on failure.
func (m Model) Maximize() Model { return m.SetObjSense(ObjSenseMaximize) }

// Minimize sets the objective sense to minimize. It panics on failure.
func (m Model) Minimize() Model { return m.SetObjSense(ObjSenseMinimize) }

// TrySetObjIntegral informs SCIP that the objective value is always integral.
func (m Model) TrySetObjIntegral() (Model, error) {
	if err := m.guard("SetObjIntegral"); err != nil {
		return m, err
	}
	return m, m.wrap("SetObjIntegral", m.scip.setObjIntegral(), "")
}

// SetObjIntegral informs SCIP that the objective value is always integral. It
// panics on failure.
func (m Model) SetObjIntegral() Model {
	m, err := m.TrySetObjIntegral()
	must(err)
	return m
}

// TrySolve solves the model. A panic inside a plugin callback is returned as
// a *CallbackPanic; a SCIP failure as an *Error. The model stays usable in
// either case.
func (m Model) TrySolve() (Model, error) {
	if err := m.guard("Solve"); err != nil {
		return m, err
	}
	err := m.scip.solve()
	if cp := callbackError(m.scip.raw); cp != nil {
		return m, cp
	}
	return m, m.wrap("Solve", err, "")
}

// Solve solves the model. It panics on failure, including with the
// *CallbackPanic of a plugin callback that panicked.
func (m Model) Solve() Model {
	m, err := m.TrySolve()
	must(err)
	return m
}

// TrySolveConcurrent solves the model using SCIP's concurrent solvers,
// leveraging multiple CPU cores when SCIP was built with thread support. The
// number of threads can be controlled with the parallel/maxnthreads
// parameter. Errors are reported as for TrySolve.
func (m Model) TrySolveConcurrent() (Model, error) {
	if err := m.guard("SolveConcurrent"); err != nil {
		return m, err
	}
	err := m.scip.solveConcurrent()
	if cp := callbackError(m.scip.raw); cp != nil {
		return m, cp
	}
	return m, m.wrap("SolveConcurrent", err, "")
}

// SolveConcurrent solves the model using SCIP's concurrent solvers. Custom
// plugins reach the worker instances only if they implement Copyable, and
// their callbacks then run on the worker threads concurrently. It panics on
// failure.
func (m Model) SolveConcurrent() Model {
	m, err := m.TrySolveConcurrent()
	must(err)
	return m
}

// TryFree releases the SCIP instance now instead of when the garbage
// collector gets to it. Plugins that hold a Variable, Constraint or Model of
// this instance keep it reachable forever, so call Free to break that cycle.
// Every handle sharing the instance is invalid afterwards. Freeing twice is a
// no-op.
func (m Model) TryFree() error {
	if m.scip == nil {
		return nil
	}
	return m.wrap("Free", m.scip.free(), "")
}

// Free releases the SCIP instance now; see TryFree. It panics on failure.
func (m Model) Free() { must(m.TryFree()) }

// TryFreeTransform frees the transformed problem and returns the model to
// the problem stage where variables and constraints can be added again,
// useful for iterated solving.
func (m Model) TryFreeTransform() (Model, error) {
	if err := m.guard("FreeTransform"); err != nil {
		return m, err
	}
	err := m.scip.freeTransform()
	if cp := callbackError(m.scip.raw); cp != nil {
		return m, cp
	}
	return m, m.wrap("FreeTransform", err, "")
}

// FreeTransform frees the transformed problem; see TryFreeTransform. It
// panics on failure.
func (m Model) FreeTransform() Model {
	m, err := m.TryFreeTransform()
	must(err)
	return m
}

// ------------------------------------------------------------- plugins

// FindHeur finds a primal heuristic by its name (e.g. "completesol"), giving
// access to its runtime statistics.
func (m Model) FindHeur(name string) (HeurPlugin, bool) { return findHeurOf(m.scip, name) }

func findHeurOf(s *Scip, name string) (HeurPlugin, bool) {
	raw := s.findHeur(name)
	if raw == nil {
		return HeurPlugin{}, false
	}
	return HeurPlugin{raw: raw, scip: s}, true
}

// Heurs returns all primal heuristics included in the model.
func (m Model) Heurs() []HeurPlugin {
	n := int(C.SCIPgetNHeurs(m.scip.raw))
	arr := C.SCIPgetHeurs(m.scip.raw)
	out := make([]HeurPlugin, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, HeurPlugin{raw: cAt(arr, i), scip: m.scip})
	}
	return out
}

// Separators returns all separators included in the model.
func (m Model) Separators() []SeparatorPlugin {
	n := int(C.SCIPgetNSepas(m.scip.raw))
	arr := C.SCIPgetSepas(m.scip.raw)
	out := make([]SeparatorPlugin, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, SeparatorPlugin{raw: cAt(arr, i)})
	}
	return out
}

// FindSeparator finds a separator by name.
func (m Model) FindSeparator(name string) (SeparatorPlugin, bool) {
	raw := m.scip.findSepa(name)
	if raw == nil {
		return SeparatorPlugin{}, false
	}
	return SeparatorPlugin{raw: raw}, true
}

// Presolvers returns all presolvers included in the model.
func (m Model) Presolvers() []PresolverPlugin {
	n := int(C.SCIPgetNPresols(m.scip.raw))
	arr := C.SCIPgetPresols(m.scip.raw)
	out := make([]PresolverPlugin, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, PresolverPlugin{raw: cAt(arr, i)})
	}
	return out
}

// FindPresolver finds a presolver by name.
func (m Model) FindPresolver(name string) (PresolverPlugin, bool) {
	raw := m.scip.findPresol(name)
	if raw == nil {
		return PresolverPlugin{}, false
	}
	return PresolverPlugin{raw: raw}, true
}

// FindNodesel finds an included node selector by its name (e.g. "bfs"),
// giving access to its priorities and statistics.
func (m Model) FindNodesel(name string) (NodeselPlugin, bool) {
	raw := m.scip.findNodesel(name)
	if raw == nil {
		return NodeselPlugin{}, false
	}
	return NodeselPlugin{raw: raw}, true
}

// TrySetHeurPriority sets the priority of a primal heuristic.
func (m Model) TrySetHeurPriority(h HeurPlugin, priority int32) error {
	if err := m.guard("SetHeurPriority"); err != nil {
		return err
	}
	return m.call("SetHeurPriority", C.SCIPsetHeurPriority(m.scip.raw, h.raw, C.int(priority)))
}

// SetHeurPriority sets the priority of a primal heuristic. It panics on failure.
func (m Model) SetHeurPriority(h HeurPlugin, priority int32) { must(m.TrySetHeurPriority(h, priority)) }

// TrySetSepaPriority sets the priority of a separator.
func (m Model) TrySetSepaPriority(s SeparatorPlugin, priority int32) error {
	if err := m.guard("SetSepaPriority"); err != nil {
		return err
	}
	return m.call("SetSepaPriority", C.SCIPsetSepaPriority(m.scip.raw, s.raw, C.int(priority)))
}

// SetSepaPriority sets the priority of a separator. It panics on failure.
func (m Model) SetSepaPriority(s SeparatorPlugin, priority int32) {
	must(m.TrySetSepaPriority(s, priority))
}

// TrySetPresolPriority sets the priority of a presolver.
func (m Model) TrySetPresolPriority(p PresolverPlugin, priority int32) error {
	if err := m.guard("SetPresolPriority"); err != nil {
		return err
	}
	return m.call("SetPresolPriority", C.SCIPsetPresolPriority(m.scip.raw, p.raw, C.int(priority)))
}

// SetPresolPriority sets the priority of a presolver. It panics on failure.
func (m Model) SetPresolPriority(p PresolverPlugin, priority int32) {
	must(m.TrySetPresolPriority(p, priority))
}

// TryIncludeBranchRule includes a new branch rule in the model.
func (m Model) TryIncludeBranchRule(name, desc string, priority, maxdepth int32, maxbounddist float64, rule BranchRule) error {
	if err := m.guard("IncludeBranchRule"); err != nil {
		return err
	}
	return m.wrap("IncludeBranchRule", m.scip.includeBranchRule(name, desc, priority, maxdepth, maxbounddist, rule), name)
}

// IncludeBranchRule includes a new branch rule in the model. It panics on
// failure, e.g. when a rule with the same name exists.
func (m Model) IncludeBranchRule(name, desc string, priority, maxdepth int32, maxbounddist float64, rule BranchRule) {
	must(m.TryIncludeBranchRule(name, desc, priority, maxdepth, maxbounddist, rule))
}

// TryIncludeNodesel includes a new node selector in the model.
func (m Model) TryIncludeNodesel(name, desc string, stdPriority, memSavePriority int32, nodesel NodeSel) error {
	if err := m.guard("IncludeNodesel"); err != nil {
		return err
	}
	return m.wrap("IncludeNodesel", m.scip.includeNodesel(name, desc, stdPriority, memSavePriority, nodesel), name)
}

// IncludeNodesel includes a new node selector in the model. It panics on failure.
func (m Model) IncludeNodesel(name, desc string, stdPriority, memSavePriority int32, nodesel NodeSel) {
	must(m.TryIncludeNodesel(name, desc, stdPriority, memSavePriority, nodesel))
}

// TryIncludeHeur includes a new primal heuristic in the model.
func (m Model) TryIncludeHeur(name, desc string, priority int32, dispchar byte, freq, freqofs, maxdepth int32, timing HeurTiming, usessubscip bool, heur Heuristic) error {
	if err := m.guard("IncludeHeur"); err != nil {
		return err
	}
	return m.wrap("IncludeHeur", m.scip.includeHeur(name, desc, priority, dispchar, freq, freqofs, maxdepth, timing, usessubscip, heur), name)
}

// IncludeHeur includes a new primal heuristic in the model. It panics on failure.
func (m Model) IncludeHeur(name, desc string, priority int32, dispchar byte, freq, freqofs, maxdepth int32, timing HeurTiming, usessubscip bool, heur Heuristic) {
	must(m.TryIncludeHeur(name, desc, priority, dispchar, freq, freqofs, maxdepth, timing, usessubscip, heur))
}

// TryIncludeSeparator includes a new separator in the model.
func (m Model) TryIncludeSeparator(name, desc string, priority, freq int32, maxbounddist float64, usesubscip, delay bool, sep Separator) error {
	if err := m.guard("IncludeSeparator"); err != nil {
		return err
	}
	return m.wrap("IncludeSeparator", m.scip.includeSeparator(name, desc, priority, freq, maxbounddist, usesubscip, delay, sep), name)
}

// IncludeSeparator includes a new separator in the model. It panics on failure.
func (m Model) IncludeSeparator(name, desc string, priority, freq int32, maxbounddist float64, usesubscip, delay bool, sep Separator) {
	must(m.TryIncludeSeparator(name, desc, priority, freq, maxbounddist, usesubscip, delay, sep))
}

// TryIncludeEventhdlr includes a new event handler in the model.
func (m Model) TryIncludeEventhdlr(name, desc string, eventhdlr Eventhdlr) error {
	if err := m.guard("IncludeEventhdlr"); err != nil {
		return err
	}
	return m.wrap("IncludeEventhdlr", m.scip.includeEventhdlr(name, desc, eventhdlr), name)
}

// IncludeEventhdlr includes a new event handler in the model. It panics on failure.
func (m Model) IncludeEventhdlr(name, desc string, eventhdlr Eventhdlr) {
	must(m.TryIncludeEventhdlr(name, desc, eventhdlr))
}

// TryIncludePricer includes a new pricer in the model and activates it.
func (m Model) TryIncludePricer(name, desc string, priority int32, delay bool, pricer Pricer) error {
	if err := m.guard("IncludePricer"); err != nil {
		return err
	}
	return m.wrap("IncludePricer", m.scip.includePricer(name, desc, priority, delay, pricer), name)
}

// IncludePricer includes a new pricer in the model and activates it. It
// panics on failure.
func (m Model) IncludePricer(name, desc string, priority int32, delay bool, pricer Pricer) {
	must(m.TryIncludePricer(name, desc, priority, delay, pricer))
}

// TryIncludeConshdlr includes a custom constraint handler in the model.
// Besides Check and Enforce, the handler may implement ConshdlrEnfoPS,
// ConshdlrSepa and ConshdlrProp; those callbacks are registered only when
// present.
func (m Model) TryIncludeConshdlr(name, desc string, enfopriority, checkpriority int32, conshdlr Conshdlr) error {
	if err := m.guard("IncludeConshdlr"); err != nil {
		return err
	}
	return m.wrap("IncludeConshdlr", m.scip.includeConshdlr(name, desc, enfopriority, checkpriority, defaultConshdlrOpts, conshdlr), name)
}

// IncludeConshdlr includes a custom constraint handler in the model; see
// TryIncludeConshdlr. It panics on failure.
func (m Model) IncludeConshdlr(name, desc string, enfopriority, checkpriority int32, conshdlr Conshdlr) {
	must(m.TryIncludeConshdlr(name, desc, enfopriority, checkpriority, conshdlr))
}

// TryAdd adds builders (variables, constraints, rows, plugins) to the model,
// discarding any return value, and stops at the first failure. Use the
// builders' TryAddTo methods when the created object is needed.
func (m Model) TryAdd(items ...any) error {
	for _, it := range items {
		var err error
		switch b := it.(type) {
		case VarBuilder:
			_, err = b.TryAddTo(m)
		case ConsBuilder:
			_, err = b.TryAddTo(m)
		case RowBuilder:
			_, err = b.TryAddTo(m)
		case BranchRuleBuilder:
			err = b.TryAddTo(m)
		case PricerBuilder:
			err = b.TryAddTo(m)
		case EventHdlrBuilder:
			err = b.TryAddTo(m)
		case HeurBuilder:
			err = b.TryAddTo(m)
		case SepaBuilder:
			err = b.TryAddTo(m)
		case NodeSelBuilder:
			err = b.TryAddTo(m)
		default:
			err = m.invalid("Add", RetcodeInvalidData, fmt.Sprintf("cannot add a value of type %T", it))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Add adds builders to the model; see TryAdd. It panics on failure.
func (m Model) Add(items ...any) { must(m.TryAdd(items...)) }

// ------------------------------------------------------------- generic API

// Inner returns a pointer to the SCIP instance, for use with raw C API calls.
func (m Model) Inner() *C.SCIP { return m.scip.raw }

// ScipPtr is an alias for Inner.
func (m Model) ScipPtr() *C.SCIP { return m.Inner() }

// Status returns the status of the optimization model.
func (m Model) Status() Status { return m.scip.status() }

// PrintVersion prints the version of SCIP used by the optimization model.
func (m Model) PrintVersion() { m.scip.printVersion() }

// ------------------------------------------------------------- parameters

// TrySetDisplayVerbosity sets the display/verblevel parameter.
func (m Model) TrySetDisplayVerbosity(level int32) (Model, error) {
	return m.SetIntParam("display/verblevel", level)
}

// SetDisplayVerbosity sets the display/verblevel parameter. It panics on failure.
func (m Model) SetDisplayVerbosity(level int32) Model {
	m, err := m.TrySetDisplayVerbosity(level)
	must(err)
	return m
}

// ShowOutput shows the output of the optimization model by setting the
// display/verblevel parameter to its default value 4.
func (m Model) ShowOutput() Model { return m.SetDisplayVerbosity(4) }

// HideOutput hides the output of the optimization model by setting the
// display/verblevel parameter to 0.
func (m Model) HideOutput() Model { return m.SetDisplayVerbosity(0) }

// TrySetTimeLimit sets the time limit for the optimization model, in seconds.
func (m Model) TrySetTimeLimit(timeLimit float64) (Model, error) {
	return m.SetRealParam("limits/time", timeLimit)
}

// SetTimeLimit sets the time limit in seconds. It panics on failure.
func (m Model) SetTimeLimit(timeLimit float64) Model {
	m, err := m.TrySetTimeLimit(timeLimit)
	must(err)
	return m
}

// TrySetMemoryLimit sets the memory limit for the optimization model, in MB.
func (m Model) TrySetMemoryLimit(memoryLimit float64) (Model, error) {
	return m.SetRealParam("limits/memory", memoryLimit)
}

// SetMemoryLimit sets the memory limit in MB. It panics on failure.
func (m Model) SetMemoryLimit(memoryLimit float64) Model {
	m, err := m.TrySetMemoryLimit(memoryLimit)
	must(err)
	return m
}

// SetStrParam sets a SCIP string parameter.
func (m Model) SetStrParam(param, value string) (Model, error) {
	if err := m.guard("SetStrParam"); err != nil {
		return m, err
	}
	return m, m.wrap("SetStrParam", m.scip.setStrParam(param, value), param)
}

// SetBoolParam sets a SCIP boolean parameter.
func (m Model) SetBoolParam(param string, value bool) (Model, error) {
	if err := m.guard("SetBoolParam"); err != nil {
		return m, err
	}
	return m, m.wrap("SetBoolParam", m.scip.setBoolParam(param, value), param)
}

// SetIntParam sets a SCIP integer parameter.
func (m Model) SetIntParam(param string, value int32) (Model, error) {
	if err := m.guard("SetIntParam"); err != nil {
		return m, err
	}
	return m, m.wrap("SetIntParam", m.scip.setIntParam(param, value), param)
}

// SetLongintParam sets a SCIP long integer parameter.
func (m Model) SetLongintParam(param string, value int64) (Model, error) {
	if err := m.guard("SetLongintParam"); err != nil {
		return m, err
	}
	return m, m.wrap("SetLongintParam", m.scip.setLongintParam(param, value), param)
}

// SetRealParam sets a SCIP real parameter.
func (m Model) SetRealParam(param string, value float64) (Model, error) {
	if err := m.guard("SetRealParam"); err != nil {
		return m, err
	}
	return m, m.wrap("SetRealParam", m.scip.setRealParam(param, value), param)
}

// TryStrParam returns the value of a SCIP string parameter.
func (m Model) TryStrParam(param string) (string, error) {
	if err := m.guard("StrParam"); err != nil {
		return "", err
	}
	v, err := m.scip.strParam(param)
	return v, m.wrap("StrParam", err, param)
}

// StrParam returns the value of a SCIP string parameter. It panics on failure.
func (m Model) StrParam(param string) string {
	v, err := m.TryStrParam(param)
	must(err)
	return v
}

// TryBoolParam returns the value of a SCIP boolean parameter.
func (m Model) TryBoolParam(param string) (bool, error) {
	if err := m.guard("BoolParam"); err != nil {
		return false, err
	}
	v, err := m.scip.boolParam(param)
	return v, m.wrap("BoolParam", err, param)
}

// BoolParam returns the value of a SCIP boolean parameter. It panics on failure.
func (m Model) BoolParam(param string) bool {
	v, err := m.TryBoolParam(param)
	must(err)
	return v
}

// TryIntParam returns the value of a SCIP integer parameter.
func (m Model) TryIntParam(param string) (int32, error) {
	if err := m.guard("IntParam"); err != nil {
		return 0, err
	}
	v, err := m.scip.intParam(param)
	return v, m.wrap("IntParam", err, param)
}

// IntParam returns the value of a SCIP integer parameter. It panics on failure.
func (m Model) IntParam(param string) int32 {
	v, err := m.TryIntParam(param)
	must(err)
	return v
}

// TryLongintParam returns the value of a SCIP long integer parameter.
func (m Model) TryLongintParam(param string) (int64, error) {
	if err := m.guard("LongintParam"); err != nil {
		return 0, err
	}
	v, err := m.scip.longintParam(param)
	return v, m.wrap("LongintParam", err, param)
}

// LongintParam returns the value of a SCIP long integer parameter. It panics
// on failure.
func (m Model) LongintParam(param string) int64 {
	v, err := m.TryLongintParam(param)
	must(err)
	return v
}

// TryRealParam returns the value of a SCIP real parameter.
func (m Model) TryRealParam(param string) (float64, error) {
	if err := m.guard("RealParam"); err != nil {
		return 0, err
	}
	v, err := m.scip.realParam(param)
	return v, m.wrap("RealParam", err, param)
}

// RealParam returns the value of a SCIP real parameter. It panics on failure.
func (m Model) RealParam(param string) float64 {
	v, err := m.TryRealParam(param)
	must(err)
	return v
}

// TrySetPresolving sets the presolving emphasis.
func (m Model) TrySetPresolving(presolving ParamSetting) (Model, error) {
	if err := m.guard("SetPresolving"); err != nil {
		return m, err
	}
	return m, m.wrap("SetPresolving", m.scip.setPresolving(presolving), "")
}

// SetPresolving sets the presolving emphasis. It panics on failure.
func (m Model) SetPresolving(presolving ParamSetting) Model {
	m, err := m.TrySetPresolving(presolving)
	must(err)
	return m
}

// TrySetSeparating sets the separating emphasis.
func (m Model) TrySetSeparating(separating ParamSetting) (Model, error) {
	if err := m.guard("SetSeparating"); err != nil {
		return m, err
	}
	return m, m.wrap("SetSeparating", m.scip.setSeparating(separating), "")
}

// SetSeparating sets the separating emphasis. It panics on failure.
func (m Model) SetSeparating(separating ParamSetting) Model {
	m, err := m.TrySetSeparating(separating)
	must(err)
	return m
}

// TrySetHeuristics sets the heuristics emphasis.
func (m Model) TrySetHeuristics(heuristics ParamSetting) (Model, error) {
	if err := m.guard("SetHeuristics"); err != nil {
		return m, err
	}
	return m, m.wrap("SetHeuristics", m.scip.setHeuristics(heuristics), "")
}

// SetHeuristics sets the heuristics emphasis. It panics on failure.
func (m Model) SetHeuristics(heuristics ParamSetting) Model {
	m, err := m.TrySetHeuristics(heuristics)
	must(err)
	return m
}

// ------------------------------------------------------------- numerics

// Eq checks equality using tolerance.
func (m Model) Eq(a, b float64) bool { return C.SCIPisEQ(m.scip.raw, C.double(a), C.double(b)) != 0 }

// Lt checks if a is less than b using tolerance.
func (m Model) Lt(a, b float64) bool { return C.SCIPisLT(m.scip.raw, C.double(a), C.double(b)) != 0 }

// Le checks if a is less than or equal to b using tolerance.
func (m Model) Le(a, b float64) bool { return C.SCIPisLE(m.scip.raw, C.double(a), C.double(b)) != 0 }

// Gt checks if a is greater than b using tolerance.
func (m Model) Gt(a, b float64) bool { return C.SCIPisGT(m.scip.raw, C.double(a), C.double(b)) != 0 }

// Ge checks if a is greater than or equal to b using tolerance.
func (m Model) Ge(a, b float64) bool { return C.SCIPisGE(m.scip.raw, C.double(a), C.double(b)) != 0 }

// Eps returns SCIP's epsilon value.
func (m Model) Eps() float64 { return float64(C.SCIPepsilon(m.scip.raw)) }
