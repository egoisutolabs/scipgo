package scip

import (
	"errors"
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
	if !errors.Is(err, RetcodeParameterWrongVal) {
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

func TestSetCharParam(t *testing.T) {
	model := NewModel().HideOutput()
	if _, err := model.SetCharParam("lp/initalgorithm", 'd'); err != nil {
		t.Fatal(err)
	}
	if got := model.CharParam("lp/initalgorithm"); got != 'd' {
		t.Fatalf("got %q", got)
	}
}

func TestTrySetCharParam(t *testing.T) {
	model := NewModel().HideOutput()
	model, err := model.TrySetCharParam("lp/pricing", 'f')
	if err != nil {
		t.Fatal(err)
	}
	got, err := model.TryCharParam("lp/pricing")
	if err != nil || got != 'f' {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestCharParamWrongType(t *testing.T) {
	model := NewModel().HideOutput()
	if _, err := model.SetCharParam("visual/vbcfilename", 'd'); !errors.Is(err, RetcodeParameterWrongType) || asError(t, err).Op != "SetCharParam" || asError(t, err).Detail != "visual/vbcfilename" {
		t.Fatalf("char on string param: %v", err)
	}
	if _, err := model.SetStrParam("lp/pricing", "d"); !errors.Is(err, RetcodeParameterWrongType) || asError(t, err).Op != "SetStrParam" {
		t.Fatalf("string on char param: %v", err)
	}
	if _, err := model.TryCharParam("visual/vbcfilename"); !errors.Is(err, RetcodeParameterWrongType) || asError(t, err).Op != "CharParam" {
		t.Fatalf("get char of string: %v", err)
	}
	if _, err := model.TryStrParam("lp/pricing"); !errors.Is(err, RetcodeParameterWrongType) {
		t.Fatalf("get string of char: %v", err)
	}
}

func TestCharParamWrongVal(t *testing.T) {
	_, err := NewModel().HideOutput().SetCharParam("lp/pricing", 'z')
	if !errors.Is(err, RetcodeParameterWrongVal) || asError(t, err).Op != "SetCharParam" {
		t.Fatalf("got %v, want ParameterWrongVal", err)
	}
}

func TestCharParamFreed(t *testing.T) {
	model := NewModel().HideOutput()
	model.Free()
	if _, err := model.TrySetCharParam("lp/pricing", 'd'); !errors.Is(err, RetcodeInvalidCall) || asError(t, err).Op != "SetCharParam" {
		t.Fatalf("TrySetCharParam freed: %v", err)
	}
	if _, err := model.SetCharParam("lp/pricing", 'd'); !errors.Is(err, RetcodeInvalidCall) || asError(t, err).Op != "SetCharParam" {
		t.Fatalf("SetCharParam freed: %v", err)
	}
	if _, err := model.TryCharParam("lp/pricing"); !errors.Is(err, RetcodeInvalidCall) || asError(t, err).Op != "CharParam" {
		t.Fatalf("TryCharParam freed: %v", err)
	}
	expectErrorPanic(t, "CharParam freed", RetcodeInvalidCall, func() { model.CharParam("lp/pricing") })
}

func TestCharParamPanic(t *testing.T) {
	model := NewModel().HideOutput()
	defer model.Free()
	expectErrorPanic(t, "unknown char param", RetcodeParameterUnknown, func() { model.CharParam("no/such") })
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

	var c byte
	if err := GetParam(model, "lp/pricing", &c); err != nil || c != 'l' {
		t.Fatalf("got %q err %v", c, err)
	}
	model, _ = SetParam(model, "lp/pricing", 'd')
	if err := GetParam(model, "lp/pricing", &c); err != nil || c != 'd' {
		t.Fatalf("got %q err %v", c, err)
	}
}

func TestSetParamIntOverflow(t *testing.T) {
	if _, err := SetParam(NewModel(), "limits/nodes", int(math.MaxInt32)+1); err == nil {
		t.Fatal("expected overflow error")
	}
}

// TestGetParamErrorIsError checks the generic getter reports failures the
// same way the typed getters do: as *Error carrying the Retcode.
func TestGetParamErrorIsError(t *testing.T) {
	model := createTestModel(t)
	defer model.Free()
	var n int64
	err := GetParam(model, "limits/nosuch", &n)
	e := asError(t, err)
	if e.Op != "LongintParam" || !errors.Is(err, RetcodeParameterUnknown) {
		t.Fatalf("GetParam error = %v, want *Error with LongintParam/ParameterUnknown", err)
	}
	var i int32
	if err := GetParam(model, "limits/nodes", &i); !errors.Is(err, RetcodeParameterWrongType) {
		t.Fatalf("wrong out type: %v", err)
	}
	if err := GetParam(model, "limits/nodes", &struct{}{}); !errors.Is(err, RetcodeInvalidData) || asError(t, err).Op != "GetParam" {
		t.Fatalf("unsupported out type: %v", err)
	}
	if _, err := SetParam(model, "limits/nodes", uint(1)); !errors.Is(err, RetcodeInvalidData) || asError(t, err).Op != "SetParam" {
		t.Fatalf("unsupported value type: %v", err)
	}
	if _, err := SetParam(model, "display/freq", int(1)<<40); !errors.Is(err, RetcodeParameterWrongVal) || asError(t, err).Op != "SetIntParam" {
		t.Fatalf("int overflow: %v", err)
	}
}

// TestGenericCharParam covers byte and rune routing. A rune is an int32, so
// an ASCII value is a char only when the parameter itself is a char; the
// same bits sent to an integer parameter stay an integer.
func TestGenericCharParam(t *testing.T) {
	model := NewModel().HideOutput()
	defer model.Free()

	model, err := SetParam(model, "lp/pricing", byte('s'))
	if err != nil {
		t.Fatal(err)
	}
	var b byte
	if err := GetParam(model, "lp/pricing", &b); err != nil || b != 's' {
		t.Fatalf("byte got %q err %v", b, err)
	}

	model, err = SetParam(model, "lp/pricing", 'd')
	if err != nil {
		t.Fatal(err)
	}
	if err := GetParam(model, "lp/pricing", &b); err != nil || b != 'd' {
		t.Fatalf("rune got %q err %v", b, err)
	}
	model, err = SetParam(model, "lp/initalgorithm", int32('p'))
	if err != nil {
		t.Fatal(err)
	}
	if got := model.CharParam("lp/initalgorithm"); got != 'p' {
		t.Fatalf("int32 char got %q", got)
	}

	// 'd' is int32(100). display/freq is an integer parameter, so the value
	// stays on the integer setter.
	model, err = SetParam(model, "display/freq", 'd')
	if err != nil {
		t.Fatal(err)
	}
	var freq int32
	if err := GetParam(model, "display/freq", &freq); err != nil || freq != int32('d') {
		t.Fatalf("freq %d err %v", freq, err)
	}
	if _, err := SetParam(model, "display/freq", int32(1000)); err != nil {
		t.Fatal(err)
	}
	if err := GetParam(model, "display/freq", &freq); err != nil || freq != 1000 {
		t.Fatalf("freq %d err %v", freq, err)
	}

	if _, err := SetParam(model, "lp/pricing", '€'); !errors.Is(err, RetcodeInvalidData) || asError(t, err).Op != "SetCharParam" {
		t.Fatalf("multi-byte rune: %v", err)
	}
	if _, err := SetParam(model, "lp/pricing", rune(0x80)); !errors.Is(err, RetcodeInvalidData) || asError(t, err).Op != "SetCharParam" {
		t.Fatalf("non-ASCII rune: %v", err)
	}
	if _, err := SetParam(model, "lp/pricing", int32(-1)); !errors.Is(err, RetcodeInvalidData) || asError(t, err).Op != "SetCharParam" {
		t.Fatalf("negative rune: %v", err)
	}
	if err := GetParam(model, "lp/pricing", &b); err != nil || b != 'd' {
		t.Fatalf("pricing changed to %q err %v", b, err)
	}
	if _, err := SetParam(model, "no/such", '€'); !errors.Is(err, RetcodeParameterUnknown) || asError(t, err).Op != "SetIntParam" {
		t.Fatalf("unknown multi-byte: %v", err)
	}

	if _, err := SetParam(model, "visual/vbcfilename", byte('d')); !errors.Is(err, RetcodeParameterWrongType) || asError(t, err).Op != "SetCharParam" {
		t.Fatalf("byte on string: %v", err)
	}
	if _, err := SetParam(model, "lp/pricing", "d"); !errors.Is(err, RetcodeParameterWrongType) || asError(t, err).Op != "SetStrParam" {
		t.Fatalf("string on char: %v", err)
	}
	if err := GetParam(model, "visual/vbcfilename", &b); !errors.Is(err, RetcodeParameterWrongType) || asError(t, err).Op != "CharParam" {
		t.Fatalf("get byte of string: %v", err)
	}
	var n int32
	if err := GetParam(model, "lp/pricing", &n); !errors.Is(err, RetcodeParameterWrongType) || asError(t, err).Op != "IntParam" {
		t.Fatalf("get int of char: %v", err)
	}
}
