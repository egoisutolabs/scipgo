package scip

import (
	"math"
	"testing"
)

func TestSetStrParam(t *testing.T) {
	model := NewModel().HideOutput()
	if _, err := model.SetStrParam("visual/vbcfilename", "ignored/test.vbc"); err != nil {
		t.Fatal(err)
	}
	if got := model.StrParam("visual/vbcfilename"); got != "ignored/test.vbc" {
		t.Fatalf("got %q", got)
	}
}

func TestSetHeursPresolvingSeparation(t *testing.T) {
	model := mustRead(t, NewModel().
		HideOutput().
		SetHeuristics(ParamSettingAggressive).
		SetPresolving(ParamSettingFast).
		SetSeparating(ParamSettingOff).
		IncludeDefaultPlugins(), testFile("simple.lp")).Solve()
	if model.Status() != StatusOptimal {
		t.Fatalf("got status %v", model.Status())
	}
}

func TestSetBoolParam(t *testing.T) {
	model := NewModel().HideOutput()
	if _, err := model.SetBoolParam("display/allviols", true); err != nil {
		t.Fatal(err)
	}
	if !model.BoolParam("display/allviols") {
		t.Fatal("param not set")
	}
}

func TestSetIntParam(t *testing.T) {
	_, err := NewModel().HideOutput().SetIntParam("display/verblevel", -1)
	if err == nil {
		t.Fatal("expected error")
	}
	if err != RetcodeParameterWrongVal {
		t.Fatalf("got %v, want ParameterWrongVal", err)
	}
}

func TestSetRealParam(t *testing.T) {
	model := NewModel().HideOutput()
	if _, err := model.SetRealParam("limits/time", 0); err != nil {
		t.Fatal(err)
	}
	if model.RealParam("limits/time") != 0 {
		t.Fatal("param not set")
	}
}

func TestSetParamAllStates(t *testing.T) {
	model := NewModel()
	model, _ = model.SetIntParam("display/verblevel", 0)
	model = model.IncludeDefaultPlugins()
	model, _ = model.SetIntParam("display/verblevel", 0)
	model = mustRead(t, model, testFile("simple.lp"))
	model, _ = model.SetIntParam("display/verblevel", 0)
	model = model.Solve()
	model, _ = model.SetIntParam("display/verblevel", 0)
}

func TestGenericParams(t *testing.T) {
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	model, _ = SetParam(model, "display/verblevel", int32(0))
	model, _ = SetParam(model, "limits/time", 0.0)
	model, _ = SetParam(model, "limits/memory", 0.0)

	var verblevel int32
	var time, memory float64
	if err := GetParam(model, "display/verblevel", &verblevel); err != nil || verblevel != 0 {
		t.Fatalf("verblevel %d err %v", verblevel, err)
	}
	if err := GetParam(model, "limits/time", &time); err != nil || time != 0 {
		t.Fatalf("time %v err %v", time, err)
	}
	if err := GetParam(model, "limits/memory", &memory); err != nil || memory != 0 {
		t.Fatalf("memory %v err %v", memory, err)
	}
}

func TestParamTypes(t *testing.T) {
	model := DefaultModel()

	var v int64
	if err := GetParam(model, "constraints/components/nodelimit", &v); err != nil || v != 10000 {
		t.Fatalf("got %d err %v", v, err)
	}
	model, _ = SetParam(model, "constraints/components/nodelimit", int64(100))
	if err := GetParam(model, "constraints/components/nodelimit", &v); err != nil || v != 100 {
		t.Fatalf("got %d err %v", v, err)
	}

	var s string
	if err := GetParam(model, "visual/vbcfilename", &s); err != nil || s != "-" {
		t.Fatalf("got %q err %v", s, err)
	}
	model, _ = SetParam(model, "visual/vbcfilename", "test")
	if err := GetParam(model, "visual/vbcfilename", &s); err != nil || s != "test" {
		t.Fatalf("got %q err %v", s, err)
	}
}

func TestSetParamIntOverflow(t *testing.T) {
	if _, err := SetParam(NewModel(), "limits/nodes", int(math.MaxInt32)+1); err == nil {
		t.Fatal("expected overflow error")
	}
}
