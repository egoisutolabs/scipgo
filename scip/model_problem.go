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
	if err := m.guard("Write"); err != nil {
		return err
	}
	return m.wrap("Write", m.scip.write(path, ext, symb), path)
}

// ------------------------------------------------------------- variables

// TryAddVar adds a new variable with the given bounds, objective coefficient,
// name and type. During solving the transformed variable is returned,
// mirroring russcip's Model<Solving>::add_var.
func (m Model) TryAddVar(lb, ub, obj float64, name string, varType VarType) (Variable, error) {
	if err := m.guard("AddVar"); err != nil {
		return Variable{}, err
	}
	if !validVarType(varType) {
		return Variable{}, m.invalid("AddVar", RetcodeInvalidData, "unknown VarType")
	}
	var varPtr *C.SCIP_VAR
	var err error
	if m.scip.stage() == StageSolving {
		varPtr, err = m.scip.createVarSolving(lb, ub, obj, name, varType)
	} else {
		varPtr, err = m.scip.createVar(lb, ub, obj, name, varType)
	}
	if err != nil {
		return Variable{}, m.wrap("AddVar", err, name)
	}
	return Variable{raw: varPtr, scip: m.scip}, nil
}

// AddVar adds a new variable; see TryAddVar. It panics on failure.
func (m Model) AddVar(lb, ub, obj float64, name string, varType VarType) Variable {
	v, err := m.TryAddVar(lb, ub, obj, name, varType)
	must(err)
	return v
}

// TryAddPricedVar adds a new priced variable during pricing.
func (m Model) TryAddPricedVar(lb, ub, obj float64, name string, varType VarType) (Variable, error) {
	if err := m.guard("AddPricedVar"); err != nil {
		return Variable{}, err
	}
	if !validVarType(varType) {
		return Variable{}, m.invalid("AddPricedVar", RetcodeInvalidData, "unknown VarType")
	}
	varPtr, err := m.scip.createPricedVar(lb, ub, obj, name, varType)
	if err != nil {
		return Variable{}, m.wrap("AddPricedVar", err, name)
	}
	return Variable{raw: varPtr, scip: m.scip}, nil
}

// AddPricedVar adds a new priced variable during pricing. It panics on failure.
func (m Model) AddPricedVar(lb, ub, obj float64, name string, varType VarType) Variable {
	v, err := m.TryAddPricedVar(lb, ub, obj, name, varType)
	must(err)
	return v
}

// ------------------------------------------------------------- constraints

func (m Model) cons(op, name string, raw *C.SCIP_CONS, err error) (Constraint, error) {
	if err != nil {
		return Constraint{}, m.wrap(op, err, name)
	}
	return Constraint{raw: raw, scip: m.scip}, nil
}

// TryAddCons adds a new linear constraint lhs <= sum(coefs*vars) <= rhs.
func (m Model) TryAddCons(vars []Variable, coefs []float64, lhs, rhs float64, name string) (Constraint, error) {
	if err := m.guard("AddCons"); err != nil {
		return Constraint{}, err
	}
	if err := m.checkVars("AddCons", vars...); err != nil {
		return Constraint{}, err
	}
	raw, err := m.scip.createCons(nil, vars, coefs, lhs, rhs, name, false)
	return m.cons("AddCons", name, raw, err)
}

// AddCons adds a new linear constraint; see TryAddCons. It panics on failure.
func (m Model) AddCons(vars []Variable, coefs []float64, lhs, rhs float64, name string) Constraint {
	c, err := m.TryAddCons(vars, coefs, lhs, rhs, name)
	must(err)
	return c
}

// TryAddConsLocal locally adds a constraint (built with NewCons) to the
// current node and its subnodes.
func (m Model) TryAddConsLocal(cons ConsBuilder) (Constraint, error) {
	return m.addLocalCons("AddConsLocal", nil, cons)
}

// AddConsLocal locally adds a constraint to the current node and its
// subnodes. It panics on failure.
func (m Model) AddConsLocal(cons ConsBuilder) Constraint {
	c, err := m.TryAddConsLocal(cons)
	must(err)
	return c
}

// TryAddConsNode locally adds a constraint (built with NewCons) to the given
// node and its children.
func (m Model) TryAddConsNode(node *Node, cons ConsBuilder) (Constraint, error) {
	if err := m.guard("AddConsNode"); err != nil {
		return Constraint{}, err
	}
	// A nil node would silently mean "the focus node" inside createCons.
	if err := m.checkNode("AddConsNode", node); err != nil {
		return Constraint{}, err
	}
	return m.addLocalCons("AddConsNode", node, cons)
}

// AddConsNode locally adds a constraint to the given node and its children.
// It panics on failure.
func (m Model) AddConsNode(node *Node, cons ConsBuilder) Constraint {
	c, err := m.TryAddConsNode(node, cons)
	must(err)
	return c
}

func (m Model) addLocalCons(op string, node *Node, cons ConsBuilder) (Constraint, error) {
	if err := m.guard(op); err != nil {
		return Constraint{}, err
	}
	if cons.expr != nil {
		return Constraint{}, m.invalid(op, RetcodeInvalidCall, "nonlinear constraints cannot be added locally")
	}
	if err := m.checkVars(op, cons.vars()...); err != nil {
		return Constraint{}, err
	}
	raw, err := m.scip.createCons(node, cons.vars(), cons.vals(), cons.lhs, cons.rhs, strOrEmpty(cons.name), true)
	return m.cons(op, strOrEmpty(cons.name), raw, err)
}

// TryAddConsCoef adds a coefficient to the given linear constraint.
func (m Model) TryAddConsCoef(c Constraint, v Variable, coef float64) error {
	if err := m.guard("AddConsCoef"); err != nil {
		return err
	}
	if err := m.checkCons("AddConsCoef", c); err != nil {
		return err
	}
	if err := m.checkVars("AddConsCoef", v); err != nil {
		return err
	}
	return m.wrap("AddConsCoef", m.scip.addConsCoef(c, v, coef), v.Name())
}

// AddConsCoef adds a coefficient to the given linear constraint. It panics on
// failure.
func (m Model) AddConsCoef(c Constraint, v Variable, coef float64) {
	must(m.TryAddConsCoef(c, v, coef))
}

// TryAddConsCoefSetppc adds a binary variable to the given set
// partitioning/covering/packing constraint.
func (m Model) TryAddConsCoefSetppc(c Constraint, v Variable) error {
	if err := m.guard("AddConsCoefSetppc"); err != nil {
		return err
	}
	if err := m.checkCons("AddConsCoefSetppc", c); err != nil {
		return err
	}
	if err := m.checkVars("AddConsCoefSetppc", v); err != nil {
		return err
	}
	if v.VarType() != VarTypeBinary {
		return m.invalid("AddConsCoefSetppc", RetcodeInvalidData, "variable "+v.Name()+" is not binary")
	}
	return m.wrap("AddConsCoefSetppc", m.scip.addConsCoefSetppc(c, v), v.Name())
}

// AddConsCoefSetppc adds a binary variable to a set partitioning/covering/
// packing constraint. It panics on failure.
func (m Model) AddConsCoefSetppc(c Constraint, v Variable) { must(m.TryAddConsCoefSetppc(c, v)) }

// TryAddConsQuadratic adds a quadratic constraint
// lhs <= sum(linCoefs*linVars) + sum(quadCoefs*quadVars1*quadVars2) <= rhs.
func (m Model) TryAddConsQuadratic(
	linVars []Variable, linCoefs []float64,
	quadVars1, quadVars2 []Variable, quadCoefs []float64,
	lhs, rhs float64, name string,
) (Constraint, error) {
	if err := m.guard("AddConsQuadratic"); err != nil {
		return Constraint{}, err
	}
	for _, vs := range [][]Variable{linVars, quadVars1, quadVars2} {
		if err := m.checkVars("AddConsQuadratic", vs...); err != nil {
			return Constraint{}, err
		}
	}
	raw, err := m.scip.createConsQuadratic(linVars, linCoefs, quadVars1, quadVars2, quadCoefs, lhs, rhs, name)
	return m.cons("AddConsQuadratic", name, raw, err)
}

// AddConsQuadratic adds a quadratic constraint; see TryAddConsQuadratic. It
// panics on failure.
func (m Model) AddConsQuadratic(
	linVars []Variable, linCoefs []float64,
	quadVars1, quadVars2 []Variable, quadCoefs []float64,
	lhs, rhs float64, name string,
) Constraint {
	c, err := m.TryAddConsQuadratic(linVars, linCoefs, quadVars1, quadVars2, quadCoefs, lhs, rhs, name)
	must(err)
	return c
}

// TryAddConsNonlinear adds the constraint lhs <= expr <= rhs, where expr is a
// nonlinear expression tree. Use Infinity or NegInfinity for a one-sided
// constraint.
func (m Model) TryAddConsNonlinear(expr Expr, lhs, rhs float64, name string) (Constraint, error) {
	if err := m.guard("AddConsNonlinear"); err != nil {
		return Constraint{}, err
	}
	raw, err := m.scip.createConsNonlinear(expr, nil, nil, lhs, rhs, name)
	return m.cons("AddConsNonlinear", name, raw, err)
}

// AddConsNonlinear adds a nonlinear constraint; see TryAddConsNonlinear. It
// panics on failure.
func (m Model) AddConsNonlinear(expr Expr, lhs, rhs float64, name string) Constraint {
	c, err := m.TryAddConsNonlinear(expr, lhs, rhs, name)
	must(err)
	return c
}

type consCreator func(vars []Variable, name string) (*C.SCIP_CONS, error)

func (m Model) addConsSet(op string, vars []Variable, name string, create consCreator) (Constraint, error) {
	if err := m.guard(op); err != nil {
		return Constraint{}, err
	}
	if err := m.checkVars(op, vars...); err != nil {
		return Constraint{}, err
	}
	for _, v := range vars {
		if v.VarType() != VarTypeBinary {
			return Constraint{}, m.invalid(op, RetcodeInvalidData, "variable "+v.Name()+" is not binary")
		}
	}
	raw, err := create(vars, name)
	return m.cons(op, name, raw, err)
}

// TryAddConsSetPart adds a set partitioning constraint over binary variables.
func (m Model) TryAddConsSetPart(vars []Variable, name string) (Constraint, error) {
	return m.addConsSet("AddConsSetPart", vars, name, m.scip.createConsSetPart)
}

// AddConsSetPart adds a set partitioning constraint. It panics on failure.
func (m Model) AddConsSetPart(vars []Variable, name string) Constraint {
	c, err := m.TryAddConsSetPart(vars, name)
	must(err)
	return c
}

// TryAddConsSetCover adds a set covering constraint over binary variables.
func (m Model) TryAddConsSetCover(vars []Variable, name string) (Constraint, error) {
	return m.addConsSet("AddConsSetCover", vars, name, m.scip.createConsSetCover)
}

// AddConsSetCover adds a set covering constraint. It panics on failure.
func (m Model) AddConsSetCover(vars []Variable, name string) Constraint {
	c, err := m.TryAddConsSetCover(vars, name)
	must(err)
	return c
}

// TryAddConsSetPack adds a set packing constraint over binary variables.
func (m Model) TryAddConsSetPack(vars []Variable, name string) (Constraint, error) {
	return m.addConsSet("AddConsSetPack", vars, name, m.scip.createConsSetPack)
}

// AddConsSetPack adds a set packing constraint. It panics on failure.
func (m Model) AddConsSetPack(vars []Variable, name string) Constraint {
	c, err := m.TryAddConsSetPack(vars, name)
	must(err)
	return c
}

// TryAddConsCardinality adds a cardinality constraint allowing at most
// cardinality non-zero variables.
func (m Model) TryAddConsCardinality(vars []Variable, cardinality int, name string) (Constraint, error) {
	if err := m.guard("AddConsCardinality"); err != nil {
		return Constraint{}, err
	}
	if err := m.checkVars("AddConsCardinality", vars...); err != nil {
		return Constraint{}, err
	}
	raw, err := m.scip.createConsCardinality(vars, cardinality, name)
	return m.cons("AddConsCardinality", name, raw, err)
}

// AddConsCardinality adds a cardinality constraint. It panics on failure.
func (m Model) AddConsCardinality(vars []Variable, cardinality int, name string) Constraint {
	c, err := m.TryAddConsCardinality(vars, cardinality, name)
	must(err)
	return c
}

// TryAddConsIndicator adds an indicator constraint: binVar == 1 implies
// sum(coefs*vars) <= rhs.
func (m Model) TryAddConsIndicator(binVar Variable, vars []Variable, coefs []float64, rhs float64, name string) (Constraint, error) {
	if err := m.guard("AddConsIndicator"); err != nil {
		return Constraint{}, err
	}
	if err := m.checkVars("AddConsIndicator", append([]Variable{binVar}, vars...)...); err != nil {
		return Constraint{}, err
	}
	if binVar.VarType() != VarTypeBinary {
		return Constraint{}, m.invalid("AddConsIndicator", RetcodeInvalidData, "indicator variable "+binVar.Name()+" is not binary")
	}
	raw, err := m.scip.createConsIndicator(binVar, vars, coefs, rhs, name)
	return m.cons("AddConsIndicator", name, raw, err)
}

// AddConsIndicator adds an indicator constraint. It panics on failure.
func (m Model) AddConsIndicator(binVar Variable, vars []Variable, coefs []float64, rhs float64, name string) Constraint {
	c, err := m.TryAddConsIndicator(binVar, vars, coefs, rhs, name)
	must(err)
	return c
}

// TryAddConsSOS1 adds an SOS1 constraint (at most one non-zero variable) with
// optional weights.
func (m Model) TryAddConsSOS1(vars []Variable, weights []float64, name string) (Constraint, error) {
	if err := m.guard("AddConsSOS1"); err != nil {
		return Constraint{}, err
	}
	if err := m.checkVars("AddConsSOS1", vars...); err != nil {
		return Constraint{}, err
	}
	raw, err := m.scip.createConsSOS1(vars, weights, name)
	return m.cons("AddConsSOS1", name, raw, err)
}

// AddConsSOS1 adds an SOS1 constraint. It panics on failure.
func (m Model) AddConsSOS1(vars []Variable, weights []float64, name string) Constraint {
	c, err := m.TryAddConsSOS1(vars, weights, name)
	must(err)
	return c
}

// TrySetConsModifiable sets the constraint's modifiable flag.
func (m Model) TrySetConsModifiable(c Constraint, modifiable bool) error {
	if err := m.guard("SetConsModifiable"); err != nil {
		return err
	}
	if err := m.checkCons("SetConsModifiable", c); err != nil {
		return err
	}
	return m.wrap("SetConsModifiable", m.scip.setConsModifiable(c, modifiable), c.Name())
}

// SetConsModifiable sets the constraint's modifiable flag. It panics on failure.
func (m Model) SetConsModifiable(c Constraint, modifiable bool) {
	must(m.TrySetConsModifiable(c, modifiable))
}

// TrySetConsRemovable sets the constraint's removable flag.
func (m Model) TrySetConsRemovable(c Constraint, removable bool) error {
	if err := m.guard("SetConsRemovable"); err != nil {
		return err
	}
	if err := m.checkCons("SetConsRemovable", c); err != nil {
		return err
	}
	return m.wrap("SetConsRemovable", m.scip.setConsRemovable(c, removable), c.Name())
}

// SetConsRemovable sets the constraint's removable flag. It panics on failure.
func (m Model) SetConsRemovable(c Constraint, removable bool) {
	must(m.TrySetConsRemovable(c, removable))
}

// TrySetConsSeparated sets whether the constraint is separated during LP
// processing.
func (m Model) TrySetConsSeparated(c Constraint, separate bool) error {
	if err := m.guard("SetConsSeparated"); err != nil {
		return err
	}
	if err := m.checkCons("SetConsSeparated", c); err != nil {
		return err
	}
	return m.wrap("SetConsSeparated", m.scip.setConsSeparated(c, separate), c.Name())
}

// SetConsSeparated sets whether the constraint is separated. It panics on failure.
func (m Model) SetConsSeparated(c Constraint, separate bool) {
	must(m.TrySetConsSeparated(c, separate))
}

// ------------------------------------------------------------- helpers

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
