// TSP with subtour elimination constraints enforced by a custom constraint
// handler. Instance a280 from TSPLIB95.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/egoisutolabs/scipgo/scip"
)

// points of instance a280.tsp from TSPLIB95
// Reinelt, G. (1995). Tsplib95. Interdisziplinaeres Zentrum fuer
// Wissenschaftliches Rechnen (IWR), Heidelberg, 338, 1-16.
var points = [][2]int{
	{288, 149}, {288, 129}, {270, 133}, {256, 141}, {256, 157}, {246, 157},
	{236, 169}, {228, 169}, {228, 161}, {220, 169}, {212, 169}, {204, 169},
	{196, 169}, {188, 169}, {196, 161}, {188, 145}, {172, 145}, {164, 145},
	{156, 145}, {148, 145}, {140, 145}, {148, 169}, {164, 169}, {172, 169},
	{156, 169}, {140, 169}, {132, 169}, {124, 169}, {116, 161}, {104, 153},
	{104, 161}, {104, 169}, {90, 165}, {80, 157}, {64, 157}, {64, 165},
	{56, 169}, {56, 161}, {56, 153}, {56, 145}, {56, 137}, {56, 129},
	{56, 121}, {40, 121}, {40, 129}, {40, 137}, {40, 145}, {40, 153},
	{40, 161}, {40, 169}, {32, 169}, {32, 161}, {32, 153}, {32, 145},
	{32, 137}, {32, 129}, {32, 121}, {32, 113}, {40, 113}, {56, 113},
	{56, 105}, {48, 99}, {40, 99}, {32, 97}, {32, 89}, {24, 89},
	{16, 97}, {16, 109}, {8, 109}, {8, 97}, {8, 89}, {8, 81},
	{8, 73}, {8, 65}, {8, 57}, {16, 57}, {8, 49}, {8, 41},
	{24, 45}, {32, 41}, {32, 49}, {32, 57}, {32, 65}, {32, 73},
	{32, 81}, {40, 83}, {40, 73}, {40, 63}, {40, 51}, {44, 43},
	{44, 35}, {44, 27}, {32, 25}, {24, 25}, {16, 25}, {16, 17},
	{24, 17}, {32, 17}, {44, 11}, {56, 9}, {56, 17}, {56, 25},
	{56, 33}, {56, 41}, {64, 41}, {72, 41}, {72, 49}, {56, 49},
	{48, 51}, {56, 57}, {56, 65}, {48, 63}, {48, 73}, {56, 73},
	{56, 81}, {48, 83}, {56, 89}, {56, 97}, {104, 97}, {104, 105},
	{104, 113}, {104, 121}, {104, 129}, {104, 137}, {104, 145}, {116, 145},
	{124, 145}, {132, 145}, {132, 137}, {140, 137}, {148, 137}, {156, 137},
	{164, 137}, {172, 125}, {172, 117}, {172, 109}, {172, 101}, {172, 93},
	{172, 85}, {180, 85}, {180, 77}, {180, 69}, {180, 61}, {180, 53},
	{172, 53}, {172, 61}, {172, 69}, {172, 77}, {164, 81}, {148, 85},
	{124, 85}, {124, 93}, {124, 109}, {124, 125}, {124, 117}, {124, 101},
	{104, 89}, {104, 81}, {104, 73}, {104, 65}, {104, 49}, {104, 41},
	{104, 33}, {104, 25}, {104, 17}, {92, 9}, {80, 9}, {72, 9},
	{64, 21}, {72, 25}, {80, 25}, {80, 25}, {80, 41}, {88, 49},
	{104, 57}, {124, 69}, {124, 77}, {132, 81}, {140, 65}, {132, 61},
	{124, 61}, {124, 53}, {124, 45}, {124, 37}, {124, 29}, {132, 21},
	{124, 21}, {120, 9}, {128, 9}, {136, 9}, {148, 9}, {162, 9},
	{156, 25}, {172, 21}, {180, 21}, {180, 29}, {172, 29}, {172, 37},
	{172, 45}, {180, 45}, {180, 37}, {188, 41}, {196, 49}, {204, 57},
	{212, 65}, {220, 73}, {228, 69}, {228, 77}, {236, 77}, {236, 69},
	{236, 61}, {228, 61}, {228, 53}, {236, 53}, {236, 45}, {228, 45},
	{228, 37}, {236, 37}, {236, 29}, {228, 29}, {228, 21}, {236, 21},
	{252, 21}, {260, 29}, {260, 37}, {260, 45}, {260, 53}, {260, 61},
	{260, 69}, {260, 77}, {276, 77}, {276, 69}, {276, 61}, {276, 53},
	{284, 53}, {284, 61}, {284, 69}, {284, 77}, {284, 85}, {284, 93},
	{284, 101}, {288, 109}, {280, 109}, {276, 101}, {276, 93}, {276, 85},
	{268, 97}, {260, 109}, {252, 101}, {260, 93}, {260, 85}, {236, 85},
	{228, 85}, {228, 93}, {236, 93}, {236, 101}, {228, 101}, {228, 109},
	{228, 117}, {228, 125}, {220, 125}, {212, 117}, {204, 109}, {196, 101},
	{188, 93}, {180, 93}, {180, 101}, {180, 109}, {180, 117}, {180, 125},
	{196, 145}, {204, 145}, {212, 145}, {220, 145}, {228, 145}, {236, 145},
	{246, 141}, {252, 125}, {260, 129}, {280, 133},
}

type edge struct {
	u, v int
	w    float64
}

type graph struct {
	n     int
	edges []edge
	// adjacency: node -> incident edge indices
	adj [][]int
}

func newGraph(n int, edges []edge) *graph {
	g := &graph{n: n, edges: edges, adj: make([][]int, n)}
	for i, e := range edges {
		g.adj[e.u] = append(g.adj[e.u], i)
		g.adj[e.v] = append(g.adj[e.v], i)
	}
	return g
}

// findSubtours returns the connected components of the graph induced by the
// given selected edges ("subtours").
func findSubtours(n int, selected []edge) [][]int {
	adj := make([][]int, n)
	for _, e := range selected {
		adj[e.u] = append(adj[e.u], e.v)
		adj[e.v] = append(adj[e.v], e.u)
	}

	visited := make([]bool, n)
	var subtours [][]int
	for start := 0; start < n; start++ {
		if visited[start] {
			continue
		}
		stack := []int{start}
		visited[start] = true
		var component []int
		for len(stack) > 0 {
			u := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			component = append(component, u)
			for _, v := range adj[u] {
				if !visited[v] {
					visited[v] = true
					stack = append(stack, v)
				}
			}
		}
		subtours = append(subtours, component)
	}
	return subtours
}

func containsNode(nodes []int, x int) bool {
	for _, n := range nodes {
		if n == x {
			return true
		}
	}
	return false
}

// subtourConshdlr enforces the TSP subtour elimination constraint.
type subtourConshdlr struct {
	vars  []scip.Variable // parallel to g.edges
	graph *graph
}

func (c *subtourConshdlr) Check(_ scip.Model, _ scip.ConshdlrPlugin, solution scip.Solution) bool {
	var selected []edge
	for i, v := range c.vars {
		if solution.Val(v) > 0.5 {
			selected = append(selected, c.graph.edges[i])
		}
	}
	return len(findSubtours(c.graph.n, selected)) == 1
}

func (c *subtourConshdlr) Enforce(model scip.Model, _ scip.ConshdlrPlugin) scip.ConshdlrResult {
	var selected []edge
	for i, v := range c.vars {
		if model.CurrentVal(v) > 0.5 {
			selected = append(selected, c.graph.edges[i])
		}
	}

	subtours := findSubtours(c.graph.n, selected)
	if len(subtours) == 1 {
		return scip.ConshdlrResultFeasible
	}

	// Add a constraint to eliminate each subtour
	for _, subtour := range subtours {
		var pairs []scip.CoefPair
		for i, e := range c.graph.edges {
			if containsNode(subtour, e.u) && containsNode(subtour, e.v) {
				pairs = append(pairs, scip.CoefPair{Var: c.vars[i], Coef: 1})
			}
		}
		model.Add(scip.NewCons().Le(float64(len(subtour) - 1)).Expr(pairs...))
	}
	return scip.ConshdlrResultConsAdded
}

func solveTSP(g *graph) (tour []int, cost float64, err error) {
	model := scip.DefaultModel()
	vars := make([]scip.Variable, len(g.edges))

	for i, e := range g.edges {
		name := fmt.Sprintf("x_%d_%d", e.u, e.v)
		vars[i] = scip.NewVar().Bin().Obj(e.w).Name(name).AddTo(model)
	}

	// degree-2 constraints at each node
	for node := 0; node < g.n; node++ {
		var pairs []scip.CoefPair
		for _, edgeIdx := range g.adj[node] {
			pairs = append(pairs, scip.CoefPair{Var: vars[edgeIdx], Coef: 1})
		}
		model.Add(scip.NewCons().Eq(2).Expr(pairs...))
	}

	model.IncludeConshdlr("SEC", "Subtour Elimination Constraint", -1, -1, &subtourConshdlr{
		vars:  vars,
		graph: g,
	})

	solved := model.Solve()
	if solved.Status() != scip.StatusOptimal {
		return nil, 0, fmt.Errorf("failed to solve: %v", solved.Status())
	}

	sol, _ := solved.BestSol()
	var selected []edge
	for i, v := range vars {
		if sol.Val(v) > 0.5 {
			selected = append(selected, g.edges[i])
		}
	}
	subtours := findSubtours(g.n, selected)
	return subtours[len(subtours)-1], sol.ObjVal(), nil
}

func main() {
	var edges []edge
	for i := range points {
		for j := i + 1; j < len(points); j++ {
			dx := float64(points[i][0] - points[j][0])
			dy := float64(points[i][1] - points[j][1])
			edges = append(edges, edge{i, j, math.Hypot(dx, dy)})
		}
	}

	tour, cost, err := solveTSP(newGraph(len(points), edges))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("found tour on %d nodes with cost %v\n", len(tour), cost)

	if len(tour) != len(points) {
		fmt.Println("tour does not visit all points")
		os.Exit(1)
	}
	if d := cost - 2586.769; d > 0.1 || d < -0.1 {
		fmt.Println("unexpected tour cost:", cost)
		os.Exit(1)
	}
}
