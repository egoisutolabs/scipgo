package scip

import "testing"

func TestStatusLimits(t *testing.T) {
	cases := []struct {
		name  string
		param string
		value any
		want  Status
	}{
		{"time", "limits/time", 0.0, StatusTimeLimit},
		{"memory", "limits/memory", 0.0, StatusMemoryLimit},
		{"gap", "limits/gap", 100000.0, StatusGapLimit},
		{"solutions", "limits/solutions", int32(0), StatusSolutionLimit},
		{"totalnodes", "limits/totalnodes", int64(0), StatusTotalNodeLimit},
		{"stallnodes", "limits/stallnodes", int64(0), StatusStallNodeLimit},
		{"bestsol", "limits/bestsol", int32(0), StatusBestSolutionLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := NewModel().HideOutput()
			model, err := SetParam(model, tc.param, tc.value)
			if err != nil {
				t.Fatal(err)
			}
			file := "simple.lp"
			if tc.name == "memory" || tc.name == "gap" || tc.name == "primal" || tc.name == "dual" {
				file = "gen-ip054.mps"
			}
			solved := mustRead(t, model.IncludeDefaultPlugins(), testFile(file)).Solve()
			if solved.Status() != tc.want {
				t.Fatalf("got status %v, want %v", solved.Status(), tc.want)
			}
		})
	}
}

func TestStatusPrimalDualLimit(t *testing.T) {
	for _, tc := range []struct {
		name, param string
		value       any
		want        Status
	}{
		{"primal", "limits/primal", 100000.0, StatusPrimalLimit},
		{"dual", "limits/dual", -100000.0, StatusDualLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := mustRead(t, NewModel().
				HideOutput().
				IncludeDefaultPlugins(), testFile("gen-ip054.mps"))
			model, err := model.SetIntParam("presolving/maxrounds", 0)
			if err != nil {
				t.Fatal(err)
			}
			model, err = model.SetRealParam(tc.param, tc.value.(float64))
			if err != nil {
				t.Fatal(err)
			}
			solved := model.Solve()
			if solved.Status() != tc.want {
				t.Fatalf("got status %v, want %v", solved.Status(), tc.want)
			}
		})
	}
}

func TestStatusUnknown(t *testing.T) {
	if NewModel().Status() != StatusUnknown {
		t.Fatal("fresh model status != Unknown")
	}
}
