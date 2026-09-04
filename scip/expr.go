package scip

/*
#include "helpers.h"
*/
import "C"

import (
	"fmt"
	"strconv"
	"strings"
)

// Expr is an immutable expression tree over variables and constants, used to
// state nonlinear constraints. Build one with Variable.Expr, Const, Sum,
// Product, Pow and friends, or the arithmetic methods, then pass it to
// Model.AddConsNonlinear or ConsBuilder.Expression. Nothing is sent to SCIP
// until the constraint is added, so an Expr can be built before its model
// exists and reused across models. The zero Expr is invalid.
type Expr struct{ n *exprNode }

type exprKind int

const (
	exprVar exprKind = iota
	exprConst
	exprSum
	exprProduct
	exprPow
	exprSignPower
	exprExp
	exprLog
	exprSin
	exprCos
	exprAbs
	exprEntropy
	exprParse
)

type exprNode struct {
	kind     exprKind
	v        Variable
	c        float64 // constant value, sum constant, product coefficient, or exponent
	coefs    []float64
	children []Expr
	text     string
}

// Expr returns the variable as an expression.
func (v Variable) Expr() Expr { return Expr{&exprNode{kind: exprVar, v: v}} }

// Const returns a constant expression.
func Const(c float64) Expr { return Expr{&exprNode{kind: exprConst, c: c}} }

// Sum returns the sum of the terms.
func Sum(terms ...Expr) Expr {
	coefs := make([]float64, len(terms))
	for i := range coefs {
		coefs[i] = 1
	}
	return WeightedSum(coefs, terms)
}

// WeightedSum returns sum(coefs[i] * terms[i]).
func WeightedSum(coefs []float64, terms []Expr) Expr {
	if len(coefs) != len(terms) {
		panic(fmt.Sprintf("scip: WeightedSum got %d coefficients for %d terms", len(coefs), len(terms)))
	}
	return Expr{&exprNode{kind: exprSum, coefs: append([]float64(nil), coefs...), children: append([]Expr(nil), terms...)}}
}

// Product returns the product of the factors.
func Product(factors ...Expr) Expr {
	return Expr{&exprNode{kind: exprProduct, c: 1, children: append([]Expr(nil), factors...)}}
}

// Pow returns e raised to the constant power p.
func Pow(e Expr, p float64) Expr { return Expr{&exprNode{kind: exprPow, c: p, children: []Expr{e}}} }

// SignPower returns sign(e) * |e|^p.
func SignPower(e Expr, p float64) Expr {
	return Expr{&exprNode{kind: exprSignPower, c: p, children: []Expr{e}}}
}

func unary(k exprKind, e Expr) Expr { return Expr{&exprNode{kind: k, children: []Expr{e}}} }

// Exp returns e^x.
func Exp(e Expr) Expr { return unary(exprExp, e) }

// Log returns the natural logarithm.
func Log(e Expr) Expr { return unary(exprLog, e) }

// Sin returns the sine.
func Sin(e Expr) Expr { return unary(exprSin, e) }

// Cos returns the cosine.
func Cos(e Expr) Expr { return unary(exprCos, e) }

// Abs returns the absolute value.
func Abs(e Expr) Expr { return unary(exprAbs, e) }

// Entropy returns -e*log(e).
func Entropy(e Expr) Expr { return unary(exprEntropy, e) }

// ParseExpr returns an expression in SCIP's own syntax, e.g. "<x>^2 + 2*<x>*<y>".
// Variables are written as <name> and are looked up by name in the model the
// constraint is added to, so the variables must exist by then.
func ParseExpr(text string) Expr { return Expr{&exprNode{kind: exprParse, text: text}} }

// Add returns e + o.
func (e Expr) Add(o Expr) Expr { return Sum(e, o) }

// Sub returns e - o.
func (e Expr) Sub(o Expr) Expr { return WeightedSum([]float64{1, -1}, []Expr{e, o}) }

// Mul returns e * o.
func (e Expr) Mul(o Expr) Expr { return Product(e, o) }

// Div returns e / o, as e * o^-1.
func (e Expr) Div(o Expr) Expr { return Product(e, Pow(o, -1)) }

// Scale returns c * e.
func (e Expr) Scale(c float64) Expr { return WeightedSum([]float64{c}, []Expr{e}) }

// Neg returns -e.
func (e Expr) Neg() Expr { return e.Scale(-1) }

// Pow returns e^p.
func (e Expr) Pow(p float64) Expr { return Pow(e, p) }

// Sqrt returns e^0.5.
func (e Expr) Sqrt() Expr { return Pow(e, 0.5) }

// Exp returns exp(e).
func (e Expr) Exp() Expr { return Exp(e) }

// Log returns log(e).
func (e Expr) Log() Expr { return Log(e) }

// Abs returns |e|.
func (e Expr) Abs() Expr { return Abs(e) }

// Sin returns sin(e).
func (e Expr) Sin() Expr { return Sin(e) }

// Cos returns cos(e).
func (e Expr) Cos() Expr { return Cos(e) }

// String renders the tree, e.g. "(x^2 + 1)".
func (e Expr) String() string {
	if e.n == nil {
		return "<empty>"
	}
	n := e.n
	f := func(x float64) string { return strconv.FormatFloat(x, 'g', -1, 64) }
	switch n.kind {
	case exprVar:
		if n.v.raw == nil {
			return "<zero Variable>"
		}
		return n.v.Name()
	case exprConst:
		return f(n.c)
	case exprSum:
		parts := make([]string, len(n.children))
		for i, ch := range n.children {
			if n.coefs[i] == 1 {
				parts[i] = ch.String()
			} else {
				parts[i] = f(n.coefs[i]) + "*" + ch.String()
			}
		}
		return "(" + strings.Join(parts, " + ") + ")"
	case exprProduct:
		parts := make([]string, len(n.children))
		for i, ch := range n.children {
			parts[i] = ch.String()
		}
		return "(" + strings.Join(parts, " * ") + ")"
	case exprPow:
		return n.children[0].String() + "^" + f(n.c)
	case exprSignPower:
		return "signpower(" + n.children[0].String() + ", " + f(n.c) + ")"
	case exprParse:
		return n.text
	}
	names := map[exprKind]string{exprExp: "exp", exprLog: "log", exprSin: "sin", exprCos: "cos", exprAbs: "abs", exprEntropy: "entropy"}
	return names[n.kind] + "(" + n.children[0].String() + ")"
}

// build materialises the tree as a SCIP expression owned by the caller, who
// must release it. Children are released here once the parent captured them.
func (e Expr) build(s *Scip) (*C.SCIP_EXPR, error) {
	if e.n == nil {
		return nil, fmt.Errorf("scip: empty expression")
	}
	n := e.n
	children := make([]*C.SCIP_EXPR, len(n.children))
	for i, ch := range n.children {
		c, err := ch.build(s)
		if err != nil {
			for _, done := range children[:i] {
				C.SCIPreleaseExpr(s.raw, &done)
			}
			return nil, err
		}
		children[i] = c
	}
	defer func() {
		for _, c := range children {
			C.SCIPreleaseExpr(s.raw, &c)
		}
	}()
	var out *C.SCIP_EXPR
	var rc C.SCIP_RETCODE
	var first **C.SCIP_EXPR
	if len(children) > 0 {
		first = &children[0]
	}
	switch n.kind {
	case exprVar:
		switch {
		case n.v.raw == nil:
			return nil, fmt.Errorf("zero Variable in expression")
		case n.v.scip == nil || n.v.scip.raw == nil:
			return nil, fmt.Errorf("variable %s belongs to a freed model", n.v.Name())
		case n.v.scip.raw != s.raw:
			return nil, fmt.Errorf("variable %s belongs to another model", n.v.Name())
		}
		rc = C.SCIPcreateExprVar(s.raw, &out, n.v.raw, nil, nil)
	case exprConst:
		rc = C.SCIPcreateExprValue(s.raw, &out, C.double(n.c), nil, nil)
	case exprSum:
		rc = C.SCIPcreateExprSum(s.raw, &out, C.int(len(children)), first, cDoubleSlice(n.coefs), 0, nil, nil)
	case exprProduct:
		rc = C.SCIPcreateExprProduct(s.raw, &out, C.int(len(children)), first, C.double(n.c), nil, nil)
	case exprPow:
		rc = C.SCIPcreateExprPow(s.raw, &out, children[0], C.double(n.c), nil, nil)
	case exprSignPower:
		rc = C.SCIPcreateExprSignpower(s.raw, &out, children[0], C.double(n.c), nil, nil)
	case exprExp:
		rc = C.SCIPcreateExprExp(s.raw, &out, children[0], nil, nil)
	case exprLog:
		rc = C.SCIPcreateExprLog(s.raw, &out, children[0], nil, nil)
	case exprSin:
		rc = C.SCIPcreateExprSin(s.raw, &out, children[0], nil, nil)
	case exprCos:
		rc = C.SCIPcreateExprCos(s.raw, &out, children[0], nil, nil)
	case exprAbs:
		rc = C.SCIPcreateExprAbs(s.raw, &out, children[0], nil, nil)
	case exprEntropy:
		rc = C.SCIPcreateExprEntropy(s.raw, &out, children[0], nil, nil)
	case exprParse:
		ct := cString(n.text)
		defer freeCString(ct)
		var end *C.char
		rc = C.SCIPparseExpr(s.raw, &out, ct, &end, nil, nil)
	}
	if err := retcodeError(rc); err != nil {
		return nil, fmt.Errorf("scip: building expression %s: %w", e, err)
	}
	return out, nil
}
