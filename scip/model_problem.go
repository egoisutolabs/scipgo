package scip

/*
#include "helpers.h"
*/
import "C"

// Vars returns all variables in the optimization model.
func (m Model) Vars() []Variable { return scipVars(m.scip, false) }

// OrigVars returns all original variables in the optimization model.
func (m Model) OrigVars() []Variable { return scipVars(m.scip, true) }

// Var returns the variable with the given ID, if it exists.
func (m Model) Var(varID VarId) (Variable, bool) { return varByID(m.scip, varID) }

// NVars returns the number of variables in the optimization model.
func (m Model) NVars() int { return m.scip.nVars() }

// NConss returns the number of constraints in the optimization model.
func (m Model) NConss() int { return m.scip.nConss() }

// FindCons finds a constraint by name.
func (m Model) FindCons(name string) (Constraint, bool) { return findConsOf(m.scip, name) }

// Conss returns all constraints in the optimization model.
func (m Model) Conss() []Constraint { return scipConss(m.scip) }

// ConsIsModifiable returns the modifiable flag of the given constraint.
func (m Model) ConsIsModifiable(c Constraint) bool { return m.scip.consIsModifiable(c) }

// ConsIsRemovable returns the removable flag of the given constraint.
func (m Model) ConsIsRemovable(c Constraint) bool { return m.scip.consIsRemovable(c) }

// ConsIsSeparated returns whether the constraint should be separated during
// LP processing.
func (m Model) ConsIsSeparated(c Constraint) bool { return m.scip.consIsSeparated(c) }

// Write writes the problem to a file using SCIP's writer.
//
// path is the file path (without extension), ext the file extension (e.g.
// "lp", "mps") and symb selects whether to use symbolic names given by the
// user for variables and constraints.
func (m Model) Write(path, ext string, symb bool) error {
	return m.scip.write(path, ext, symb)
}

// AddVar adds a new variable to the model with the given bounds, objective
// coefficient, name, and type. During solving, the transformed variable is
// returned (mirroring russcip's Model<Solving>::add_var).
func (m Model) AddVar(lb, ub, obj float64, name string, varType VarType) Variable {
	var varPtr *C.SCIP_VAR
	var err error
	if C.SCIPgetStage(m.scip.raw) == C.SCIP_STAGE_SOLVING {
		varPtr, err = m.scip.createVarSolving(lb, ub, obj, name, varType)
		if err != nil {
			panic("Failed to create variable in state Solving")
		}
	} else {
		varPtr, err = m.scip.createVar(lb, ub, obj, name, varType)
		if err != nil {
			panic("Failed to create variable in state ProblemCreated")
		}
	}
	return Variable{raw: varPtr, scip: m.scip}
}

// AddPricedVar adds a new priced variable to the SCIP data structure.
func (m Model) AddPricedVar(lb, ub, obj float64, name string, varType VarType) Variable {
	varPtr, err := m.scip.createPricedVar(lb, ub, obj, name, varType)
	if err != nil {
		panic("Failed to create variable in state Solving")
	}
	return Variable{raw: varPtr, scip: m.scip}
}

// AddCons adds a new linear constraint to the model with the given variables,
// coefficients, sides, and name.
func (m Model) AddCons(vars []Variable, coefs []float64, lhs, rhs float64, name string) Constraint {
	cons, err := m.scip.createCons(nil, vars, coefs, lhs, rhs, name, false)
	if err != nil {
		panic("Failed to create constraint")
	}
	return Constraint{raw: cons, scip: m.scip}
}

// AddConsLocal locally adds a constraint (built with NewCons) to the current
// node and its subnodes.
func (m Model) AddConsLocal(cons ConsBuilder) Constraint {
	return m.addLocalCons(nil, cons)
}

// AddConsNode locally adds a constraint (built with NewCons) to the given
// node and its children.
func (m Model) AddConsNode(node *Node, cons ConsBuilder) Constraint {
	return m.addLocalCons(node, cons)
}

func (m Model) addLocalCons(node *Node, cons ConsBuilder) Constraint {
	consPtr, err := m.scip.createCons(node, cons.vars(), cons.vals(), cons.lhs, cons.rhs, strOrEmpty(cons.name), true)
	if err != nil {
		panic("Failed to create constraint in state Solving")
	}
	return Constraint{raw: consPtr, scip: m.scip}
}

// AddConsCoef adds a coefficient to the given constraint for the given
// variable.
func (m Model) AddConsCoef(c Constraint, v Variable, coef float64) {
	if err := m.scip.addConsCoef(c, v, coef); err != nil {
		panic("Failed to add constraint coefficient")
	}
}

// AddConsCoefSetppc adds a binary variable to the given set
// partitioning/covering/packing constraint.
func (m Model) AddConsCoefSetppc(c Constraint, v Variable) {
	if v.VarType() != VarTypeBinary {
		panic("variable in setppc constraint must be binary")
	}
	if err := m.scip.addConsCoefSetppc(c, v); err != nil {
		panic("Failed to add constraint coefficient")
	}
}

// AddConsQuadratic adds a new quadratic constraint to the model.
func (m Model) AddConsQuadratic(
	linVars []Variable, linCoefs []float64,
	quadVars1, quadVars2 []Variable, quadCoefs []float64,
	lhs, rhs float64, name string,
) Constraint {
	cons, err := m.scip.createConsQuadratic(linVars, linCoefs, quadVars1, quadVars2, quadCoefs, lhs, rhs, name)
	if err != nil {
		panic("Failed to create quadratic constraint")
	}
	return Constraint{raw: cons, scip: m.scip}
}

// AddConsSetPart adds a new set partitioning constraint with the given binary
// variables.
func (m Model) AddConsSetPart(vars []Variable, name string) Constraint {
	return m.addConsSet(vars, name, m.scip.createConsSetPart)
}

// AddConsSetCover adds a new set cover constraint with the given binary
// variables.
func (m Model) AddConsSetCover(vars []Variable, name string) Constraint {
	return m.addConsSet(vars, name, m.scip.createConsSetCover)
}

// AddConsSetPack adds a new set packing constraint with the given binary
// variables.
func (m Model) AddConsSetPack(vars []Variable, name string) Constraint {
	return m.addConsSet(vars, name, m.scip.createConsSetPack)
}

type consCreator func(vars []Variable, name string) (*C.SCIP_CONS, error)

func (m Model) addConsSet(vars []Variable, name string, create consCreator) Constraint {
	for _, v := range vars {
		if v.VarType() != VarTypeBinary {
			panic("variables in set partitioning constraint must be binary")
		}
	}
	cons, err := create(vars, name)
	if err != nil {
		panic("Failed to create set partitioning constraint")
	}
	return Constraint{raw: cons, scip: m.scip}
}

// AddConsCardinality adds a new cardinality constraint allowing at most
// `cardinality` non-zero variables.
func (m Model) AddConsCardinality(vars []Variable, cardinality int, name string) Constraint {
	cons, err := m.scip.createConsCardinality(vars, cardinality, name)
	if err != nil {
		panic("Failed to add cardinality constraint")
	}
	return Constraint{raw: cons, scip: m.scip}
}

// AddConsIndicator adds a new indicator constraint: binVar == 1 implies
// vars . coefs <= rhs.
func (m Model) AddConsIndicator(binVar Variable, vars []Variable, coefs []float64, rhs float64, name string) Constraint {
	if binVar.VarType() != VarTypeBinary {
		panic("indicator variable must be binary")
	}
	cons, err := m.scip.createConsIndicator(binVar, vars, coefs, rhs, name)
	if err != nil {
		panic("Failed to create indicator constraint")
	}
	return Constraint{raw: cons, scip: m.scip}
}

// AddConsSOS1 adds a new SOS1 constraint (at most one non-zero variable)
// with optional weights.
func (m Model) AddConsSOS1(vars []Variable, weights []float64, name string) Constraint {
	cons, err := m.scip.createConsSOS1(vars, weights, name)
	if err != nil {
		panic("Failed to create SOS1 constraint")
	}
	return Constraint{raw: cons, scip: m.scip}
}

// SetConsModifiable sets the constraint as modifiable or not.
func (m Model) SetConsModifiable(c Constraint, modifiable bool) {
	if err := m.scip.setConsModifiable(c, modifiable); err != nil {
		panic("Failed to set constraint modifiable")
	}
}

// SetConsRemovable sets the constraint as removable or not.
func (m Model) SetConsRemovable(c Constraint, removable bool) {
	if err := m.scip.setConsRemovable(c, removable); err != nil {
		panic("Failed to set constraint removable")
	}
}

// SetConsSeparated sets whether the constraint should be separated during LP
// processing.
func (m Model) SetConsSeparated(c Constraint, separate bool) {
	if err := m.scip.setConsSeparated(c, separate); err != nil {
		panic("Failed to set constraint separated")
	}
}

func scipVars(s *Scip, original bool) []Variable {
	m := s.vars(original)
	maxIdx := -1
	for id := range m {
		if id > maxIdx {
			maxIdx = id
		}
	}
	out := make([]Variable, 0, len(m))
	for id := 0; id <= maxIdx; id++ { // index order, like the Rust BTreeMap
		if v, ok := m[id]; ok {
			out = append(out, Variable{raw: v, scip: s})
		}
	}
	return out
}

func scipConss(s *Scip) []Constraint {
	raw := s.conss()
	out := make([]Constraint, 0, len(raw))
	for _, c := range raw {
		out = append(out, Constraint{raw: c, scip: s})
	}
	return out
}

func findConsOf(s *Scip, name string) (Constraint, bool) {
	raw := s.findCons(name)
	if raw == nil {
		return Constraint{}, false
	}
	return Constraint{raw: raw, scip: s}, true
}

func varByID(s *Scip, varID VarId) (Variable, bool) {
	// Keyed by SCIPvarGetIndex to match Variable.Index; varFromID would use
	// the probindex, which differs once the problem is transformed.
	v, ok := s.vars(false)[varID]
	if !ok {
		return Variable{}, false
	}
	return Variable{raw: v, scip: s}, true
}
