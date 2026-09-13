# Branch rules

A branching rule decides how to split the current node when its LP
solution is fractional. SCIP asks the rules in priority order until one
acts.

## Interface

```go
type BranchRule interface {
	Execute(model Model, rule BranchRulePlugin, candidates []BranchingCandidate) BranchingResult
}
```

`candidates` lists the integer variables with fractional LP value:

```go
type BranchingCandidate struct {
	VarProbID int     // index of the variable in the current problem; Model.VarInProb resolves it
	LpSolVal  float64 // its LP value
	Frac      float64 // its fractionality, in [0, 1) for negative values too
}
```

The result tells SCIP what happened:

| Result | Meaning |
| --- | --- |
| `scip.BranchOn(c)` | Branch on candidate `c`: SCIP creates the down and up children |
| `BranchingResultCustomBranching` | You created the children yourself |
| `BranchingResultCutOff` | The node is infeasible |
| `BranchingResultReduceDom` | You tightened a bound; no branching needed |
| `BranchingResultConsAdded` | You added a constraint that resolves the fractionality |
| `BranchingResultSeparated` | You added a cut |
| `BranchingResultDidNotRun` | Let the next rule try |

Only `BranchOn` needs the `Candidate` field; the others are returned as
`scip.BranchingResult{Kind: ...}`.

## Registration

```go
model.Add(scip.NewBranchRule(rule).
	Name("mostinf").
	Desc("most infeasible branching").
	Priority(100000).   // default
	MaxDepth(-1).       // default: any depth
	MaxBoundDist(1.0))  // default: all nodes
```

`MaxBoundDist` restricts the rule to nodes whose dual bound is within that
relative distance of the best node's. The defaults make the rule run at
every node, ahead of SCIP's own rules.

## Example: most infeasible branching

Pick the candidate whose fractional part is closest to one half:

```go
type mostInfeasible struct{}

func (mostInfeasible) Execute(model scip.Model, _ scip.BranchRulePlugin,
	cands []scip.BranchingCandidate) scip.BranchingResult {
	best := cands[0]
	for _, c := range cands[1:] {
		if math.Abs(c.Frac-0.5) < math.Abs(best.Frac-0.5) {
			best = c
		}
	}
	return scip.BranchOn(best)
}
```

The full program under `examples/most_infeasible_branching` reads an MPS
file, turns off presolving, heuristics and separation so that the rule is
exercised, and checks the solve reaches optimality.

## Custom branching

A rule can create the children itself and shape them however the problem
needs. Create children, change bounds in them, and report
`BranchingResultCustomBranching`:

```go
down := model.CreateChild()
up := model.CreateChild()
model.SetUbNode(&down, v, math.Floor(cand.LpSolVal))
model.SetLbNode(&up, v, math.Ceil(cand.LpSolVal))
return scip.BranchingResult{Kind: scip.BranchingResultCustomBranching}
```

`AddConsNode` attaches a constraint to a child instead of a bound, which
is how disjunctions that are not single-variable bounds are branched on.
The bin packing example implements Ryan-Foster branching this way: each
child receives a "these two items go together" or "these two items are
apart" decision, recorded in the datastore under the child's node number
so the pricer can honour it.

## Notes

- `Execute` is only called for LP branching, at nodes where the LP was
  solved and is fractional. It is not called for pseudo-solution branching
  or external candidates.
- Resolve a candidate to its variable with `model.VarInProb(c.VarProbID)`
  when you need its name, bounds or column.
- A rule that returns `BranchingResultDidNotRun` defers to the next rule by
  priority, ending with SCIP's default `relpscost`.
