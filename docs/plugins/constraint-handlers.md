# Constraint handlers

A constraint handler defines a constraint type SCIP does not know:
subtour elimination, a scheduling rule, a black-box feasibility check. It
must be able to say whether a solution is feasible and to do something
about an infeasible LP solution; it may also separate cuts and propagate
bounds.

## Interface

```go
type Conshdlr interface {
	// Check reports whether the solution satisfies the constraint.
	Check(model Model, conshdlr ConshdlrPlugin, solution Solution) bool
	// Enforce handles the current LP solution, which is integral for the
	// usual priorities (see below).
	Enforce(model Model, conshdlr ConshdlrPlugin) ConshdlrResult
}
```

Three optional interfaces add callbacks. They are registered only when
the handler implements them:

```go
type ConshdlrEnfoPS interface {
	// Enforce a pseudo solution, at nodes where no LP was solved.
	EnforcePseudo(model Model, conshdlr ConshdlrPlugin, solInfeasible, objInfeasible bool) ConshdlrResult
}
type ConshdlrSepa interface {
	// Separate the current LP solution.
	SeparateLP(model Model, conshdlr ConshdlrPlugin) SeparationResult
}
type ConshdlrProp interface {
	// Propagate variable domains before the node's LP is solved.
	Propagate(model Model, conshdlr ConshdlrPlugin) PropResult
}
```

`Enforce` and `EnforcePseudo` return a `ConshdlrResult`:

| Result | Meaning |
| --- | --- |
| `ConshdlrResultFeasible` | The solution satisfies the constraint |
| `ConshdlrResultConsAdded` | You added a constraint (usually with `AddConsLocal` or `Add`) that cuts the solution off |
| `ConshdlrResultSeparated` | You added a cut with `AddCut` |
| `ConshdlrResultReducedDom` | You tightened a bound |
| `ConshdlrResultBranched` | You created children |
| `ConshdlrResultCutOff` | The node is infeasible |
| `ConshdlrResultSolveLP` | Ask SCIP to re-solve the LP |
| `ConshdlrResultInfeasible` | The solution is infeasible but you did nothing; SCIP branches |
| `ConshdlrResultDidNotRun` | `EnforcePseudo` only, allowed when `objInfeasible` is true |

## Registration

```go
model.IncludeConshdlr("SEC", "subtour elimination", -1, -1, &subtourHandler{...})
```

There is no builder; the two integers are the enforcement priority and the
check priority, and they matter:

- **Check priority.** SCIP calls handlers' `Check` in descending priority
  and stops at the first that says infeasible. SCIP's integrality handler
  has priority 0. A **negative** check priority means your `Check` is only
  reached for solutions that are already integral, which is what a
  combinatorial constraint like subtour elimination wants. A positive
  priority means it also sees fractional candidates.
- **Enforcement priority.** Same rule for `Enforce`: with a negative
  priority the integrality handler runs first and branches on fractional
  LP solutions, so your `Enforce` only ever sees integral ones.

Most custom handlers use `-1, -1`, as the TSP example does. SCIP's linear
constraint handler sits at -1000000 for both, so `-1` still runs before it.

## Example: subtour elimination

The TSP formulation has a binary variable per edge and a degree-two
constraint per city, which admits solutions made of disjoint cycles. The
handler rejects those and adds a cut for each cycle it finds:

```go
type subtourHandler struct {
	vars  []scip.Variable // parallel to edges
	edges []edge
	n     int
}

func (h *subtourHandler) Check(_ scip.Model, _ scip.ConshdlrPlugin, sol scip.Solution) bool {
	return len(h.subtours(func(i int) float64 { return sol.Val(h.vars[i]) })) == 1
}

func (h *subtourHandler) Enforce(model scip.Model, _ scip.ConshdlrPlugin) scip.ConshdlrResult {
	tours := h.subtours(func(i int) float64 { return model.CurrentVal(h.vars[i]) })
	if len(tours) == 1 {
		return scip.ConshdlrResultFeasible
	}
	for _, tour := range tours {
		var terms []scip.CoefPair
		for i, e := range h.edges {
			if inTour(tour, e.u) && inTour(tour, e.v) {
				terms = append(terms, scip.CoefPair{Var: h.vars[i], Coef: 1})
			}
		}
		// the edges inside a proper subset of cities cannot form a cycle
		model.Add(scip.NewCons().Expr(terms...).Le(float64(len(tour) - 1)))
	}
	return scip.ConshdlrResultConsAdded
}

func (h *subtourHandler) Copy() any { return h }
```

`Check` receives a `Solution` and reads it with `Val`; `Enforce` reads the
LP solution with `CurrentVal`. Both are called with the same handler
value, so the edge list is shared through its fields. The complete program
is under `examples/tsp` and solves the 280-city TSPLIB instance `a280`.

## Sub-MIP heuristics and `Copy`

SCIP's large neighbourhood search heuristics copy the problem into a
sub-SCIP and solve it. A copy is only valid if every constraint handler
can be copied. A handler without `Copy` marks every copy invalid, which
silently disables RENS, RINS, crossover and the other sub-MIP heuristics
for the whole solve. Implement `Copyable` unless that is what you want;
returning the receiver is correct for a handler whose fields are read-only
during the solve.

## Notes

- The Go handler does not create individual constraint objects: one
  handler applies to the whole problem. In SCIP's terms it is a constraint
  handler registered with `needscons` false.
- `ConshdlrPlugin.CreateEmptyRow` creates an LP row attributed to the
  handler, for use with `SeparateLP` and `AddCut`; see
  [Separators](separators.md) for the row workflow.
- `AddSol` runs `Check` on the solution being added, so a panic in `Check`
  surfaces from `AddSol` as a `*scip.CallbackPanic`.
