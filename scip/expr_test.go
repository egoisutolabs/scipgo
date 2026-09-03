package scip

import (
	"math"
	"testing"
)

func circleModel(t *testing.T) (Model, Variable, Variable) {
	t.Helper()
	m := NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("circle").Minimize()
	x := m.AddVar(-1, 1, 1, "x", VarTypeContinuous)
	y := m.AddVar(-1, 1, 1, "y", VarTypeContinuous)
	return m, x, y
}

func TestNonlinearCircle(t *testing.T) {
	m, x, y := circleModel(t)
	// min x + y  s.t.  x^2 + y^2 <= 1  ->  -sqrt(2)
	m.AddConsNonlinear(x.Expr().Pow(2).Add(y.Expr().Pow(2)), NegInfinity, 1, "disc")
	solved := m.Solve()
	if solved.Status() != StatusOptimal {
		t.Fatalf("status %v", solved.Status())
	}
	if got := solved.ObjVal(); math.Abs(got+math.Sqrt2) > 1e-4 {
		t.Fatalf("obj %v, want %v", got, -math.Sqrt2)
	}
	m.Free()
}

func TestParseExprCons(t *testing.T) {
	m, _, _ := circleModel(t)
	m.AddConsNonlinear(ParseExpr("<x>^2 + <y>^2"), NegInfinity, 1, "disc")
	if got := m.Solve().ObjVal(); math.Abs(got+math.Sqrt2) > 1e-4 {
		t.Fatalf("obj %v, want %v", got, -math.Sqrt2)
	}
	m.Free()
}

func TestNonlinearBuilderWithLinearPart(t *testing.T) {
	// max x + y  s.t.  x*y + 0*x <= 1, x,y in [0,2]  ->  x=2, y=1/2, obj 2.5
	m := NewModel().HideOutput().IncludeDefaultPlugins().CreateProb("bilinear").Maximize()
	x := m.AddVar(0, 2, 1, "x", VarTypeContinuous)
	y := m.AddVar(0, 2, 1, "y", VarTypeContinuous)
	m.Add(NewCons().Expression(x.Expr().Mul(y.Expr())).Coef(x, 0).Le(1).Name("xy"))
	m, _ = m.SetRealParam("limits/gap", 1e-6)
	if m.NConss() != 1 {
		t.Fatalf("nconss %d", m.NConss())
	}
	if got := m.Solve().ObjVal(); math.Abs(got-2.5) > 1e-3 {
		t.Fatalf("obj %v, want 2.5", got)
	}
	m.Free()
}

func TestExprString(t *testing.T) {
	m, x, y := circleModel(t)
	defer m.Free()
	e := x.Expr().Pow(2).Add(Const(1)).Mul(Log(y.Expr())).Sub(y.Expr().Scale(3))
	if got, want := e.String(), "(((x^2 + 1) * log(y)) + -1*(3*y))"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if (Expr{}).String() != "<empty>" {
		t.Fatal("empty expr string")
	}
}

func TestEmptyExprPanics(t *testing.T) {
	m, _, _ := circleModel(t)
	defer m.Free()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for empty expression")
		}
	}()
	m.AddConsNonlinear(Expr{}, 0, 1, "bad")
}

func TestNonlinearLocalPanics(t *testing.T) {
	m, x, _ := circleModel(t)
	defer m.Free()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic adding a nonlinear constraint locally")
		}
	}()
	m.AddConsLocal(NewCons().Expression(x.Expr().Pow(2)).Le(1))
}
