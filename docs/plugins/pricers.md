# Pricers

A pricer generates variables during the solve: column generation. The
master problem starts with a few columns, and at each node, after the LP
is solved, the pricer looks at the duals and adds columns with negative
reduced cost until none remain. Cutting stock, bin packing, vehicle
routing and crew scheduling are the classic uses.

## Interface

```go
type Pricer interface {
	GenerateColumns(model Model, pricer PricerPlugin, farkas bool) PricerResult
}
```

`farkas` is false for regular reduced-cost pricing and true when the LP is
infeasible and SCIP needs columns that restore feasibility (Farkas
pricing). The result:

```go
type PricerResult struct {
	State      PricerResultState
	LowerBound *float64 // optional: a valid lower bound on the node's LP value
}
```

| State | Meaning |
| --- | --- |
| `PricerResultStateFoundColumns` | Columns were added; SCIP re-solves the LP and prices again |
| `PricerResultStateNoColumns` | No improving column exists; the LP is optimal for this node |
| `PricerResultStateStopEarly` | Stop pricing and branch now, even though columns might exist |
| `PricerResultStateDidNotRun` | The pricer was skipped |

`LowerBound`, when set for reduced-cost pricing, lets SCIP prune the node
before pricing converges: a Lagrangian bound from the pricing subproblem
is the usual source.

## Registration

```go
model.Add(scip.NewPricer(&patternPricer{...}).
	Name("patterns").
	Desc("knapsack pricing").
	Priority(100000).   // default
	Delay(false))       // default
```

Including a pricer activates it. `Delay(true)` tells SCIP to call this
pricer only after all non-delayed pricers found nothing.

## Setting up the master problem

Two things the model needs for pricing to work:

- **Constraints the pricer extends must be modifiable.** SCIP will not
  add variables to a constraint that is not; set `Modifiable(true)` on the
  builder or `SetConsModifiable` on the constraint.
- **Presolving should be off**, at least for those constraints. Presolving
  can fix or aggregate away structure the pricer relies on, and it does not
  know columns will be added later. `SetPresolving(scip.ParamSettingOff)`
  is the safe choice.

## Inside the pricer

```go
func (p *patternPricer) GenerateColumns(model scip.Model, _ scip.PricerPlugin, farkas bool) scip.PricerResult {
	// 1. duals of the demand constraints
	duals := make([]float64, len(p.demand))
	for i, cons := range p.demand {
		t, _ := cons.Transformed() // the constraint SCIP is solving with
		var ok bool
		if farkas {
			duals[i], ok = t.FarkasDualSol()
		} else {
			duals[i], ok = t.DualSol()
		}
		if !ok {
			panic("demand constraint is not linear")
		}
	}

	// 2. pricing subproblem: a knapsack with the duals as profits
	items, profit := p.solveKnapsack(duals)
	const cost = 1.0 // every pattern costs one roll in the master
	testCost := cost
	if farkas {
		testCost = 0 // Farkas pricing ignores the objective
	}
	if testCost-profit >= -model.Eps() {
		return scip.PricerResult{State: scip.PricerResultStateNoColumns}
	}

	// 3. add the column, with its real master cost
	v := model.AddPricedVar(0, scip.Infinity, cost, p.name(items), scip.VarTypeInteger)
	for _, i := range items {
		model.AddConsCoef(p.demand[i], v, 1)
	}
	return scip.PricerResult{State: scip.PricerResultStateFoundColumns}
}
```

The pieces:

- **Duals.** The constraints you hold from building the model are
  original ones; `Transformed` returns the working copy whose duals are
  current. `DualSol` and `FarkasDualSol` are defined for linear
  constraints. In Farkas pricing the reduced-cost test treats the
  objective as zero, since the goal is feasibility; the column added to
  the master still gets its real cost.
- **The subproblem.** Anything works: a nested `scip.Model` (what the
  examples do), a dynamic program, a hand-written solver. A nested model
  is a separate SCIP instance and may be created and freed inside the
  callback.
- **Adding the column.** `AddPricedVar` adds a variable during the solve
  and returns the transformed variable. `AddConsCoef` puts it into the
  modifiable constraints with the right coefficients. The objective
  coefficient is the column's cost in the master.

## Branching and pricing

Standard variable branching on a column-generation master is problematic:
fixing a column to zero does not stop the pricer from regenerating it.
Two approaches, both in the examples:

- **Price only at the root.** `examples/cutting_stock` returns
  `NoColumns` at depth greater than zero, so the tree below is solved
  over the columns found at the root. This is a heuristic, not exact
  branch-and-price: returning `NoColumns` tells SCIP the node's LP is
  optimal, while a column that was not improving at the root can have
  negative reduced cost under a child's duals. The result is the optimum
  over the root's column set, which may be worse than the true optimum.
  Adequate when the root generates the useful columns, which it often
  does; say so in your documentation if you ship it.
- **Branch on the structure.** `examples/bin_packing` implements
  Ryan-Foster branching: a custom [branch rule](branch-rules.md) creates
  children that force two items together or apart, records the decision
  in the datastore under the child's node number, and the pricer reads the
  decisions for `model.FocusNode().Number()` and adds them as constraints
  to its knapsack subproblem.

## Notes

- `model.FocusNode().Depth()` tells you where you are; the duals are
  those of the focus node's LP.
- A pricer that implements `Copyable` runs in sub-SCIPs too. That is
  rarely wanted: sub-MIP heuristics on a column-generation master usually
  work with the columns present. Leave `Copy` out, or return a pricer that
  answers `NoColumns`.
- Reduced costs of existing columns are available through
  `Variable.Redcost` while the node LP is solved.
