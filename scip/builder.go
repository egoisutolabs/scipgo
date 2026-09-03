package scip

import (
	"fmt"
	"math"
	"strconv"
)

// VarBuilder is a builder for variables, created with NewVar.
//
//	x := scip.NewVar().Name("x").Bin().Obj(1.0).AddTo(model)
type VarBuilder struct {
	name    *string
	obj     float64
	lb      float64
	ub      float64
	varType VarType
}

// NewVar creates a new default VarBuilder: continuous, bounds [0, +inf), obj 0.
func NewVar() VarBuilder {
	return VarBuilder{
		lb:      0,
		ub:      math.Inf(1),
		varType: VarTypeContinuous,
	}
}

// Bin makes the variable binary with bounds [0, 1].
func (b VarBuilder) Bin() VarBuilder {
	b.lb, b.ub = 0, 1
	b.varType = VarTypeBinary
	return b
}

// Int makes the variable an integer, keeping current bounds.
func (b VarBuilder) Int() VarBuilder {
	b.varType = VarTypeInteger
	return b
}

// IntRange makes the variable an integer with inclusive bounds [lb, ub].
func (b VarBuilder) IntRange(lb, ub int) VarBuilder {
	b.lb, b.ub = float64(lb), float64(ub)
	b.varType = VarTypeInteger
	return b
}

// Cont makes the variable continuous, keeping current bounds.
func (b VarBuilder) Cont() VarBuilder {
	b.varType = VarTypeContinuous
	return b
}

// ContRange makes the variable continuous with bounds [lb, ub].
func (b VarBuilder) ContRange(lb, ub float64) VarBuilder {
	b.lb, b.ub = lb, ub
	b.varType = VarTypeContinuous
	return b
}

// ImplInt makes the variable an implicit integer, keeping current bounds.
func (b VarBuilder) ImplInt() VarBuilder {
	b.varType = VarTypeImplInt
	return b
}

// ImplIntRange makes the variable an implicit integer with inclusive bounds
// [lb, ub].
func (b VarBuilder) ImplIntRange(lb, ub int) VarBuilder {
	b.lb, b.ub = float64(lb), float64(ub)
	b.varType = VarTypeImplInt
	return b
}

// Bounds sets explicit bounds on the variable.
func (b VarBuilder) Bounds(lb, ub float64) VarBuilder {
	b.lb, b.ub = lb, ub
	return b
}

// Name sets the name of the variable.
func (b VarBuilder) Name(name string) VarBuilder {
	b.name = &name
	return b
}

// Obj sets the objective coefficient of the variable.
func (b VarBuilder) Obj(obj float64) VarBuilder {
	b.obj = obj
	return b
}

// AddTo adds the variable to a model in the ProblemCreated stage.
func (b VarBuilder) AddTo(m Model) Variable {
	return m.AddVar(b.lb, b.ub, b.obj, b.defaultName(m.NVars()), b.varType)
}

// AddToSolving adds the variable to a model in the Solving stage.
func (b VarBuilder) AddToSolving(m Model) Variable { return b.AddTo(m) }

func (b VarBuilder) defaultName(nVars int) string {
	if b.name != nil {
		return *b.name
	}
	return "x" + strconv.Itoa(nVars)
}

// CoefPair is a (variable, coefficient) pair used by ConsBuilder.
type CoefPair struct {
	Var  Variable
	Coef float64
}

// ConsBuilder is a builder for linear constraints, created with NewCons.
//
//	c := scip.NewCons().Name("c").Eq(1).Coef(x, 1).Coef(y, 1).AddTo(model)
type ConsBuilder struct {
	lhs        float64
	rhs        float64
	name       *string
	coefs      []CoefPair
	expr       *Expr
	modifiable *bool
	removable  *bool
	separated  *bool
}

// NewCons creates a new default ConsBuilder: -inf <= expr <= +inf.
func NewCons() ConsBuilder {
	return ConsBuilder{
		lhs: math.Inf(-1),
		rhs: math.Inf(1),
	}
}

// Bounds sets both sides: lhs <= expr <= rhs.
func (b ConsBuilder) Bounds(lhs, rhs float64) ConsBuilder {
	b.lhs, b.rhs = lhs, rhs
	return b
}

// Expression makes this a nonlinear constraint on e; any Coef terms are added
// as its linear part.
func (b ConsBuilder) Expression(e Expr) ConsBuilder {
	b.expr = &e
	return b
}

// Le sets the constraint to the form expr <= val.
func (b ConsBuilder) Le(val float64) ConsBuilder {
	b.lhs = math.Inf(-1)
	b.rhs = val
	return b
}

// Ge sets the constraint to the form val <= expr.
func (b ConsBuilder) Ge(val float64) ConsBuilder {
	b.lhs = val
	b.rhs = math.Inf(1)
	return b
}

// Eq sets the constraint to the form expr = val.
func (b ConsBuilder) Eq(val float64) ConsBuilder {
	b.lhs, b.rhs = val, val
	return b
}

// Name sets the name of the constraint.
func (b ConsBuilder) Name(name string) ConsBuilder {
	b.name = &name
	return b
}

// Coef adds a coefficient for the given variable.
func (b ConsBuilder) Coef(v Variable, coef float64) ConsBuilder {
	b.coefs = append(b.coefs, CoefPair{Var: v, Coef: coef})
	return b
}

// Coefs adds coefficients for the given variables.
func (b ConsBuilder) Coefs(vars []Variable, vals []float64) ConsBuilder {
	if len(vars) != len(vals) {
		panic(fmt.Sprintf("scip: Coefs got %d vars but %d coefficients", len(vars), len(vals)))
	}
	for i := range vars {
		b.coefs = append(b.coefs, CoefPair{Var: vars[i], Coef: vals[i]})
	}
	return b
}

// Expr adds a list of (variable, coefficient) pairs.
func (b ConsBuilder) Expr(pairs ...CoefPair) ConsBuilder {
	b.coefs = append(b.coefs, pairs...)
	return b
}

// Modifiable sets the modifiable flag of the constraint.
func (b ConsBuilder) Modifiable(modifiable bool) ConsBuilder {
	b.modifiable = &modifiable
	return b
}

// Removable sets the removable flag of the constraint.
func (b ConsBuilder) Removable(removable bool) ConsBuilder {
	b.removable = &removable
	return b
}

// Separated sets whether the constraint should be separated during LP
// processing.
func (b ConsBuilder) Separated(separate bool) ConsBuilder {
	b.separated = &separate
	return b
}

// AddTo adds the constraint to a model in the ProblemCreated stage.
func (b ConsBuilder) AddTo(m Model) Constraint {
	var c Constraint
	if b.expr != nil {
		raw, err := m.scip.createConsNonlinear(*b.expr, b.vars(), b.vals(), b.lhs, b.rhs, b.defaultName(m.NConss()))
		if err != nil {
			panic(fmt.Sprintf("Failed to create nonlinear constraint: %v", err))
		}
		c = Constraint{raw: raw, scip: m.scip}
	} else {
		c = m.AddCons(b.vars(), b.vals(), b.lhs, b.rhs, b.defaultName(m.NConss()))
	}
	b.applyFlags(m, c)
	return c
}

// AddToSolving adds the constraint to a model in the Solving stage.
func (b ConsBuilder) AddToSolving(m Model) Constraint { return b.AddTo(m) }

func (b ConsBuilder) vars() []Variable {
	out := make([]Variable, 0, len(b.coefs))
	for _, pc := range b.coefs {
		out = append(out, pc.Var)
	}
	return out
}

func (b ConsBuilder) vals() []float64 {
	out := make([]float64, 0, len(b.coefs))
	for _, pc := range b.coefs {
		out = append(out, pc.Coef)
	}
	return out
}

func (b ConsBuilder) defaultName(nConss int) string {
	if b.name != nil {
		return *b.name
	}
	return "cons" + strconv.Itoa(nConss)
}

func (b ConsBuilder) applyFlags(m Model, c Constraint) {
	if b.modifiable != nil {
		m.SetConsModifiable(c, *b.modifiable)
	}
	if b.removable != nil {
		m.SetConsRemovable(c, *b.removable)
	}
	if b.separated != nil {
		m.SetConsSeparated(c, *b.separated)
	}
}

// RowSource describes where a row created by RowBuilder comes from.
type RowSource struct {
	separator         *SCIPSeparator
	constraintHandler *SCIPConshdlr
	constraint        *Constraint
}

// SourceSepa marks the row as coming from a separator.
func SourceSepa(sep SCIPSeparator) RowSource { return RowSource{separator: &sep} }

// SourceConshdlr marks the row as coming from a constraint handler.
func SourceConshdlr(ch SCIPConshdlr) RowSource { return RowSource{constraintHandler: &ch} }

// SourceCons marks the row as coming from a constraint.
func SourceCons(c Constraint) RowSource { return RowSource{constraint: &c} }

// RowBuilder is a builder for LP rows, created with NewRow.
//
//	row := scip.NewRow().Eq(5).Local(true).AddToSolving(model)
type RowBuilder struct {
	lhs        float64
	rhs        float64
	name       *string
	modifiable *bool
	removable  *bool
	local      *bool
	source     *RowSource
}

// NewRow creates a new default RowBuilder: -inf <= expr <= +inf.
func NewRow() RowBuilder {
	return RowBuilder{
		lhs: math.Inf(-1),
		rhs: math.Inf(1),
	}
}

// Le sets the row to the form expr <= val.
func (b RowBuilder) Le(val float64) RowBuilder {
	b.lhs = math.Inf(-1)
	b.rhs = val
	return b
}

// Ge sets the row to the form val <= expr.
func (b RowBuilder) Ge(val float64) RowBuilder {
	b.lhs = val
	b.rhs = math.Inf(1)
	return b
}

// Eq sets the row to the form expr = val.
func (b RowBuilder) Eq(val float64) RowBuilder {
	b.lhs, b.rhs = val, val
	return b
}

// Bounds sets both sides of the row.
func (b RowBuilder) Bounds(lhs, rhs float64) RowBuilder {
	b.lhs, b.rhs = lhs, rhs
	return b
}

// Name sets the name of the row.
func (b RowBuilder) Name(name string) RowBuilder {
	b.name = &name
	return b
}

// Modifiable sets the modifiable flag of the row.
func (b RowBuilder) Modifiable(modifiable bool) RowBuilder {
	b.modifiable = &modifiable
	return b
}

// Removable sets the removable flag of the row.
func (b RowBuilder) Removable(removable bool) RowBuilder {
	b.removable = &removable
	return b
}

// Local sets whether the row is only valid locally.
func (b RowBuilder) Local(local bool) RowBuilder {
	b.local = &local
	return b
}

// Source sets the origin of the row.
func (b RowBuilder) Source(src RowSource) RowBuilder {
	b.source = &src
	return b
}

// AddTo adds the row to a model in the ProblemCreated stage.
func (b RowBuilder) AddTo(m Model) Row {
	rowPtr, err := m.scip.createEmptyRow(&b)
	if err != nil {
		panic("Failed to create row")
	}
	return Row{raw: rowPtr, scip: m.scip}
}

// AddToSolving adds the row to a model in the Solving stage.
func (b RowBuilder) AddToSolving(m Model) Row { return b.AddTo(m) }
