// A separator that identifies cliques in the conflict graph of a set
// partitioning problem and adds corresponding clique cuts.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/egoisutolabs/scipgo/scip"
)

type cliqueSeparator struct {
	// threshold for considering a variable as "fractional"
	fracThreshold float64
}

// buildConflictGraph builds the conflict graph for the variables in the
// model. In a set partitioning problem, two variables conflict if they share
// a row with coefficient 1.
func (s *cliqueSeparator) buildConflictGraph(model scip.Model) map[int]map[int]bool {
	vars := model.Vars()
	graph := make(map[int]map[int]bool)
	for _, v := range vars {
		graph[v.Index()] = make(map[int]bool)
	}

	var partitioningConstraints [][]int
	for _, cons := range model.Conss() {
		row, ok := cons.Row()
		if !ok {
			continue
		}
		// Only consider equality constraints (partitioning constraints)
		if row.Lhs() != 1.0 || row.Rhs() != 1.0 {
			continue
		}
		var varsInCons []int
		for _, col := range row.Cols() {
			varsInCons = append(varsInCons, col.Var().Index())
		}
		partitioningConstraints = append(partitioningConstraints, varsInCons)
	}

	for _, varsInCons := range partitioningConstraints {
		for i := 0; i < len(varsInCons); i++ {
			for j := i + 1; j < len(varsInCons); j++ {
				graph[varsInCons[i]][varsInCons[j]] = true
				graph[varsInCons[j]][varsInCons[i]] = true
			}
		}
	}
	return graph
}

// findClique greedily finds a clique among the model variables.
func (s *cliqueSeparator) findClique(graph map[int]map[int]bool, vars []scip.Variable) []int {
	// Start with the first variable
	clique := []int{0}

	for i := 1; i < len(vars); i++ {
		varIdx := vars[i].Index()
		canAdd := true
		for _, c := range clique {
			if !graph[varIdx][vars[c].Index()] {
				canAdd = false
				break
			}
		}
		if canAdd {
			clique = append(clique, i)
		}
	}
	return clique
}

func (s *cliqueSeparator) ExecuteLP(model scip.Model, sepa scip.SeparatorPlugin) scip.SeparationResult {
	fmt.Println("-- CliqueSeparator: Executing LP separation")

	vars := model.Vars()

	// Get current LP values
	lpValues := make([]float64, len(vars))
	for i, v := range vars {
		lpValues[i] = model.CurrentVal(v)
	}

	hasFractional := false
	for _, val := range lpValues {
		if val > s.fracThreshold && val < 1.0-s.fracThreshold {
			hasFractional = true
			break
		}
	}
	if !hasFractional {
		fmt.Println("-- CliqueSeparator: No fractional variables found")
		return scip.SeparationResultDidNotFind
	}

	graph := s.buildConflictGraph(model)
	fmt.Printf("-- CliqueSeparator: Conflict graph built with %d nodes\n", len(graph))

	clique := s.findClique(graph, vars)
	fmt.Printf("-- CliqueSeparator: Found clique with %d variables\n", len(clique))
	if len(clique) <= 1 {
		return scip.SeparationResultDidNotFind
	}

	// Create a clique cut: sum of variables in the clique <= 1
	cliqueCut, err := sepa.CreateEmptyRow(model, "clique_cut",
		math.Inf(-1), 1, false, false, false)
	if err != nil {
		panic(err)
	}

	sumLpValues := 0.0
	for _, i := range clique {
		sumLpValues += lpValues[i]
		cliqueCut.SetCoeff(vars[i], 1)
	}

	// Only add the cut if it's violated
	if sumLpValues > 1.0+1e-6 {
		fmt.Printf("-- CliqueSeparator: Found violated clique cut with %d variables, sum of LP values: %v\n",
			len(clique), sumLpValues)
		model.AddCut(cliqueCut, true)
		return scip.SeparationResultSeparated
	}
	return scip.SeparationResultDidNotFind
}

func main() {
	model, err := scip.SetParam(scip.NewModel().
		IncludeDefaultPlugins().
		CreateProb("setpart_clique").
		SetPresolving(scip.ParamSettingOff).
		SetSeparating(scip.ParamSettingOff).
		SetHeuristics(scip.ParamSettingOff), "branching/pscost/priority", int32(1000000))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	model, err = scip.SetParam(model, "misc/usesymmetry", int32(0))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var vars []scip.Variable
	for i := 0; i < 6; i++ {
		vars = append(vars, scip.NewVar().Bin().Obj(1).AddTo(model))
	}
	x1, x2, x3, x4, x5, x6 := vars[0], vars[1], vars[2], vars[3], vars[4], vars[5]

	// Add set partitioning constraints
	model.AddConsSetPart([]scip.Variable{x1, x2, x4}, "set1")
	model.AddConsSetPart([]scip.Variable{x1, x3, x5}, "set2")
	model.AddConsSetPart([]scip.Variable{x2, x3, x6}, "set3")

	// Add our clique separator
	model.Add(scip.NewSepa(&cliqueSeparator{fracThreshold: 0.1}).
		Name("clique_separator").
		Desc("Clique separator for set partitioning problems"))

	solved := model.Solve()

	fmt.Printf("\nSolution status: %v\n", solved.Status())
	fmt.Printf("Objective value: %.2f\n", solved.ObjVal())

	if solved.Status() != scip.StatusOptimal || solved.NNodes() != 1 {
		fmt.Println("expected optimal at 1 node, got", solved.Status(), solved.NNodes())
		os.Exit(1)
	}
}
