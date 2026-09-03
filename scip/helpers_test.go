package scip

import (
	"path/filepath"
	"testing"
)

const testdata = "../data/test"

func testFile(name string) string { return filepath.Join(testdata, name) }

func createTestModel(t *testing.T) Model {
	t.Helper()
	model := NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		CreateProb("test").
		Maximize()

	x1 := model.AddVar(0, Infinity, 3, "x1", VarTypeInteger)
	x2 := model.AddVar(0, Infinity, 4, "x2", VarTypeInteger)
	model.AddCons([]Variable{x1, x2}, []float64{2, 1}, NegInfinity, 100, "c1")
	model.AddCons([]Variable{x1, x2}, []float64{1, 2}, NegInfinity, 80, "c2")
	return model
}

func mustRead(t *testing.T, m Model, file string) Model {
	t.Helper()
	m2, err := m.ReadProb(file)
	if err != nil {
		t.Fatalf("read_prob %s: %v", file, err)
	}
	return m2
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
