package scip

import "testing"

func TestVarBuilder(t *testing.T) {
	model := DefaultModel().Maximize()
	x := NewVar().Name("x").Obj(1).ContRange(0, 1).AddTo(model)

	if model.NVars() != 1 {
		t.Fatalf("n_vars %d", model.NVars())
	}
	if x.Name() != "x" || x.Obj() != 1 || x.Lb() != 0 || x.Ub() != 1 {
		t.Fatal("wrong variable attributes")
	}
	solved := model.Solve()
	if solved.Status() != StatusOptimal || solved.ObjVal() != 1 {
		t.Fatalf("status %v obj %v", solved.Status(), solved.ObjVal())
	}
}

func TestVarBuilderDefaultName(t *testing.T) {
	model := DefaultModel()
	x := NewVar().Bin().AddTo(model)
	if x.Name() != "x0" {
		t.Fatalf("got name %q, want x0", x.Name())
	}
}

func TestConsBuilder(t *testing.T) {
	model := MinimalModel().HideOutput()
	v := NewVar().Bin().Obj(1).AddTo(model)
	c := NewCons().Name("c").Eq(1).Coef(v, 1)
	if c.lhs != 1 || c.rhs != 1 || len(c.coefs) != 1 || c.coefs[0].Coef != 1 {
		t.Fatal("builder state wrong")
	}
	model.Add(c)

	if model.NConss() != 1 {
		t.Fatalf("n_conss %d", model.NConss())
	}
	if model.Conss()[0].Name() != "c" {
		t.Fatal("wrong constraint name")
	}
	solved := model.Solve()
	if solved.Status() != StatusOptimal || solved.ObjVal() != 1 {
		t.Fatalf("status %v obj %v", solved.Status(), solved.ObjVal())
	}
}

func TestConsBuilderFlags(t *testing.T) {
	model := MinimalModel().HideOutput()
	vars := []Variable{
		NewVar().Bin().Obj(1).AddTo(model),
		NewVar().Bin().Obj(1).AddTo(model),
		NewVar().Bin().Obj(1).AddTo(model),
	}

	cons1 := NewCons().Name("c1").Le(2).Coefs(vars, []float64{1, 1, 1}).Modifiable(true).AddTo(model)
	cons2 := NewCons().Name("c2").Ge(1).Coefs(vars, []float64{1, 1, 1}).Modifiable(false).AddTo(model)
	cons3 := NewCons().Name("c3").Ge(1).Coef(vars[0], 1).AddTo(model)

	if !cons1.IsModifiable() || cons2.IsModifiable() || cons3.IsModifiable() {
		t.Fatal("wrong modifiable flags")
	}

	rowB := NewCons().Name("r").Le(2).Coef(vars[0], 1).Removable(true)
	if rowB.removable == nil || !*rowB.removable {
		t.Fatal("removable flag not stored")
	}
}

func TestConsBuilderCoefsMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on length mismatch")
		}
	}()
	NewCons().Coefs([]Variable{{}, {}}, []float64{1})
}
