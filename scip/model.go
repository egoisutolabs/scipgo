package scip

/*
#include "helpers.h"
*/
import "C"

import "fmt"

// FindHeur finds a primal heuristic by its name (e.g. "completesol"), giving
// access to its runtime statistics.
func (m Model) FindHeur(name string) (Heur, bool) { return findHeurOf(m.scip, name) }

func findHeurOf(s *Scip, name string) (Heur, bool) {
	raw := s.findHeur(name)
	if raw == nil {
		return Heur{}, false
	}
	return Heur{raw: raw, scip: s}, true
}

// Model represents an optimization model backed by a SCIP instance.
//
// Unlike russcip's Model<State> typestate, a single Go type is used for all
// lifecycle stages: Go does not permit methods on concrete instantiations of
// generic types. Calling a method in the wrong stage yields a SCIP retcode
// error (usually a panic, mirroring russcip's expect()).
//
// Typical lifecycle:
//
//	model := scip.NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("problem")
//	x := model.AddVar(0, 1, 1, "x", scip.VarTypeBinary)
//	solved := model.Solve()
type Model struct {
	scip *Scip
}

// NewModel creates a new Model instance. It panics if the SCIP instance
// cannot be created.
func NewModel() Model {
	m, err := TryNewModel()
	if err != nil {
		panic("Failed to create SCIP instance")
	}
	return m
}

// TryNewModel tries to create a new Model instance.
func TryNewModel() (Model, error) {
	s, err := newScip()
	if err != nil {
		return Model{}, err
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

// IncludeDefaultPlugins includes all default plugins in the SCIP instance
// and returns the model in the plugins-included stage.
func (m Model) IncludeDefaultPlugins() Model {
	if err := m.scip.includeDefaultPlugins(); err != nil {
		panic("Failed to include default plugins")
	}
	return m
}

// CreateProb creates a new problem in the SCIP instance with the given name.
// It panics if the problem cannot be created in the current state.
func (m Model) CreateProb(name string) Model {
	if err := m.scip.createProb(name); err != nil {
		panic("Failed to create problem in state PluginsIncluded")
	}
	return m
}

// ReadProb reads a problem from the given file.
func (m Model) ReadProb(filename string) (Model, error) {
	if err := m.scip.readProb(filename); err != nil {
		return Model{}, err
	}
	return m, nil
}

// SetObjSense sets the objective sense of the model to the given value.
func (m Model) SetObjSense(sense ObjSense) Model {
	if err := m.scip.setObjSense(sense); err != nil {
		panic("Failed to set objective sense in state ProblemCreated")
	}
	return m
}

// Maximize sets the objective sense of the model to maximize.
func (m Model) Maximize() Model { return m.SetObjSense(ObjSenseMaximize) }

// Minimize sets the objective sense of the model to minimize.
func (m Model) Minimize() Model { return m.SetObjSense(ObjSenseMinimize) }

// SetObjIntegral informs SCIP that the objective value is always integral.
func (m Model) SetObjIntegral() Model {
	if err := m.scip.setObjIntegral(); err != nil {
		panic("Failed to set the objective value as integral")
	}
	return m
}

// TrySolve tries to solve the model, returning an error on failure.
// Panics that occurred inside plugin callbacks during solving are re-raised
// here.
func (m Model) TrySolve() (Model, error) {
	err := m.scip.solve()
	rethrowPanics(m.scip.raw)
	if err != nil {
		return Model{}, err
	}
	return m, nil
}

// Solve solves the model. It panics if the problem cannot be solved.
func (m Model) Solve() Model {
	solved, err := m.TrySolve()
	if err != nil {
		panic("Failed to solve problem in state ProblemCreated")
	}
	return solved
}

// TrySolveConcurrent tries to solve the model using SCIP's concurrent
// solvers, leveraging multiple CPU cores when SCIP was built with thread
// support. The number of threads can be controlled with the
// parallel/maxnthreads parameter.
func (m Model) TrySolveConcurrent() (Model, error) {
	err := m.scip.solveConcurrent()
	rethrowPanics(m.scip.raw)
	if err != nil {
		return Model{}, err
	}
	return m, nil
}

// SolveConcurrent solves the model using SCIP's concurrent solvers. Custom
// plugins reach the worker instances only if they implement Copyable, and
// their callbacks then run on the worker threads concurrently.
func (m Model) SolveConcurrent() Model {
	solved, err := m.TrySolveConcurrent()
	if err != nil {
		panic("Failed to solve problem concurrently in state ProblemCreated")
	}
	return solved
}

// Free releases the SCIP instance now instead of when the garbage collector
// gets to it. Plugins that hold a Variable, Constraint or Model of this
// instance keep it reachable forever, so call Free to break that cycle.
// Every handle sharing the instance is invalid afterwards. Freeing twice is
// a no-op.
func (m Model) Free() {
	if err := m.scip.free(); err != nil {
		panic(fmt.Sprintf("scip: failed to free SCIP instance: %v", err))
	}
}

// FreeTransform frees the transformed problem and returns the model to the
// problem-created stage where variables and constraints can be added again,
// useful for iterated solving.
func (m Model) FreeTransform() Model {
	err := m.scip.freeTransform()
	rethrowPanics(m.scip.raw)
	if err != nil {
		panic(fmt.Sprintf("SCIP returned unexpected retcode %v", err))
	}
	return m
}

// IncludeBranchRule includes a new branch rule in the model.
// It panics if the inclusion fails (e.g. a rule with the same name exists).
func (m Model) IncludeBranchRule(name, desc string, priority, maxdepth int32, maxbounddist float64, rule BranchRule) {
	if err := m.scip.includeBranchRule(name, desc, priority, maxdepth, maxbounddist, rule); err != nil {
		panic("Failed to include branch rule at state ProblemCreated")
	}
}

// IncludeNodesel includes a new node selector in the model.
func (m Model) IncludeNodesel(name, desc string, stdPriority, memSavePriority int32, nodesel NodeSel) {
	if err := m.scip.includeNodesel(name, desc, stdPriority, memSavePriority, nodesel); err != nil {
		panic("Failed to include node selector at state ProblemCreated")
	}
}

// IncludeHeur includes a new primal heuristic in the model.
func (m Model) IncludeHeur(name, desc string, priority int32, dispchar byte, freq, freqofs, maxdepth int32, timing HeurTiming, usessubscip bool, heur Heuristic) {
	if err := m.scip.includeHeur(name, desc, priority, dispchar, freq, freqofs, maxdepth, timing, usessubscip, heur); err != nil {
		panic("Failed to include heuristic at state ProblemCreated")
	}
}

// IncludeSeparator includes a new separator in the model.
func (m Model) IncludeSeparator(name, desc string, priority, freq int32, maxbounddist float64, usesubscip, delay bool, sep Separator) {
	if err := m.scip.includeSeparator(name, desc, priority, freq, maxbounddist, usesubscip, delay, sep); err != nil {
		panic("Failed to include separator at state ProblemCreated")
	}
}

// IncludeEventhdlr includes a new event handler in the model.
func (m Model) IncludeEventhdlr(name, desc string, eventhdlr Eventhdlr) {
	if err := m.scip.includeEventhdlr(name, desc, eventhdlr); err != nil {
		panic("Failed to include event handler at state ProblemCreated")
	}
}

// IncludePricer includes a new pricer in the SCIP data structure.
func (m Model) IncludePricer(name, desc string, priority int32, delay bool, pricer Pricer) {
	if err := m.scip.includePricer(name, desc, priority, delay, pricer); err != nil {
		panic("Failed to include pricer at state ProblemCreated")
	}
}

// IncludeConshdlr includes a custom constraint handler in the SCIP data
// structure. Besides Check and Enforce, the handler may implement
// ConshdlrEnfoPS, ConshdlrSepa and ConshdlrProp; those callbacks are
// registered only when present.
func (m Model) IncludeConshdlr(name, desc string, enfopriority, checkpriority int32, conshdlr Conshdlr) {
	if err := m.scip.includeConshdlr(name, desc, enfopriority, checkpriority, defaultConshdlrOpts, conshdlr); err != nil {
		panic("Failed to include constraint handler at state ProblemCreated")
	}
}

// Add adds builders (variables, constraints, rows, plugins) to the model,
// discarding any return value. Use the builders' AddTo methods when the
// created object is needed.
func (m Model) Add(items ...any) {
	for _, it := range items {
		switch b := it.(type) {
		case VarBuilder:
			b.AddTo(m)
		case ConsBuilder:
			b.AddTo(m)
		case RowBuilder:
			b.AddTo(m)
		case BranchRuleBuilder:
			b.AddTo(m)
		case PricerBuilder:
			b.AddTo(m)
		case EventHdlrBuilder:
			b.AddTo(m)
		case HeurBuilder:
			b.AddTo(m)
		case SepaBuilder:
			b.AddTo(m)
		case NodeSelBuilder:
			b.AddTo(m)
		default:
			panic(fmt.Sprintf("scip: cannot add value of type %T to model", it))
		}
	}
}

// Inner returns a pointer to the SCIP instance, for use with raw C API calls.
func (m Model) Inner() *C.SCIP { return m.scip.raw }

// ScipPtr is an alias for Inner.
func (m Model) ScipPtr() *C.SCIP { return m.Inner() }

// FindNodesel finds an included node selector by its name (e.g. "bfs"),
// giving access to its priorities and statistics.
func (m Model) FindNodesel(name string) (SCIPNodesel, bool) {
	raw := m.scip.findNodesel(name)
	if raw == nil {
		return SCIPNodesel{}, false
	}
	return SCIPNodesel{raw: raw}, true
}

// Status returns the status of the optimization model.
func (m Model) Status() Status { return m.scip.status() }

// PrintVersion prints the version of SCIP used by the optimization model.
func (m Model) PrintVersion() { m.scip.printVersion() }

// SetDisplayVerbosity sets the display/verblevel parameter to the provided value.
func (m Model) SetDisplayVerbosity(level int32) Model {
	if err := m.scip.setIntParam("display/verblevel", level); err != nil {
		panic(fmt.Sprintf("Failed to set display/verblevel to %d", level))
	}
	return m
}

// ShowOutput shows the output of the optimization model by setting the
// display/verblevel parameter to its default value 4.
func (m Model) ShowOutput() Model { return m.SetDisplayVerbosity(4) }

// HideOutput hides the output of the optimization model by setting the
// display/verblevel parameter to 0.
func (m Model) HideOutput() Model { return m.SetDisplayVerbosity(0) }

// SetTimeLimit sets the time limit for the optimization model, in seconds.
func (m Model) SetTimeLimit(timeLimit float64) Model {
	if err := m.scip.setRealParam("limits/time", timeLimit); err != nil {
		panic("Failed to set time limit")
	}
	return m
}

// SetMemoryLimit sets the memory limit for the optimization model, in MB.
func (m Model) SetMemoryLimit(memoryLimit float64) Model {
	if err := m.scip.setRealParam("limits/memory", memoryLimit); err != nil {
		panic("Failed to set memory limit")
	}
	return m
}

// SetStrParam sets a SCIP string parameter.
func (m Model) SetStrParam(param, value string) (Model, error) {
	if err := m.scip.setStrParam(param, value); err != nil {
		return m, err
	}
	return m, nil
}

// SetBoolParam sets a SCIP boolean parameter.
func (m Model) SetBoolParam(param string, value bool) (Model, error) {
	if err := m.scip.setBoolParam(param, value); err != nil {
		return m, err
	}
	return m, nil
}

// SetIntParam sets a SCIP integer parameter.
func (m Model) SetIntParam(param string, value int32) (Model, error) {
	if err := m.scip.setIntParam(param, value); err != nil {
		return m, err
	}
	return m, nil
}

// SetLongintParam sets a SCIP long integer parameter.
func (m Model) SetLongintParam(param string, value int64) (Model, error) {
	if err := m.scip.setLongintParam(param, value); err != nil {
		return m, err
	}
	return m, nil
}

// SetRealParam sets a SCIP real parameter.
func (m Model) SetRealParam(param string, value float64) (Model, error) {
	if err := m.scip.setRealParam(param, value); err != nil {
		return m, err
	}
	return m, nil
}

// StrParam returns the value of a SCIP string parameter.
func (m Model) StrParam(param string) string {
	v, err := m.scip.strParam(param)
	if err != nil {
		panic("Failed to get string parameter")
	}
	return v
}

// BoolParam returns the value of a SCIP boolean parameter.
func (m Model) BoolParam(param string) bool {
	v, err := m.scip.boolParam(param)
	if err != nil {
		panic("Failed to get boolean parameter")
	}
	return v
}

// IntParam returns the value of a SCIP integer parameter.
func (m Model) IntParam(param string) int32 {
	v, err := m.scip.intParam(param)
	if err != nil {
		panic("Failed to get integer parameter")
	}
	return v
}

// LongintParam returns the value of a SCIP long integer parameter.
func (m Model) LongintParam(param string) int64 {
	v, err := m.scip.longintParam(param)
	if err != nil {
		panic("Failed to get long integer parameter")
	}
	return v
}

// RealParam returns the value of a SCIP real parameter.
func (m Model) RealParam(param string) float64 {
	v, err := m.scip.realParam(param)
	if err != nil {
		panic("Failed to get real parameter")
	}
	return v
}

// SetPresolving sets the presolving parameter of the SCIP instance.
func (m Model) SetPresolving(presolving ParamSetting) Model {
	if err := m.scip.setPresolving(presolving); err != nil {
		panic("Failed to set presolving with valid value")
	}
	return m
}

// SetSeparating sets the separating parameter of the SCIP instance.
func (m Model) SetSeparating(separating ParamSetting) Model {
	if err := m.scip.setSeparating(separating); err != nil {
		panic("Failed to set separating with valid value")
	}
	return m
}

// SetHeuristics sets the heuristics parameter of the SCIP instance.
func (m Model) SetHeuristics(heuristics ParamSetting) Model {
	if err := m.scip.setHeuristics(heuristics); err != nil {
		panic("Failed to set heuristics with valid value")
	}
	return m
}

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
