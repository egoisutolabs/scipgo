# Tree and LP access

What a plugin can see of the branch-and-bound tree and the LP relaxation
while a solve is running. Everything on this page is meaningful in the
Solving stage; outside it, the accessors return nil or report a
`*scip.Error`, never garbage.

## Nodes

```go
node := model.FocusNode() // the node being processed; panics if there is none
```

| Method | Returns |
| --- | --- |
| `Number` | The node's number, unique for the run and stable; use it as a key |
| `Depth` | Depth in the tree, 0 at the root |
| `LowerBound` | The node's dual bound |
| `Parent` | The parent, and false at the root |
| `NChildren`, `Children` | The children; `Children` is only available for the focus node |

The open nodes are reachable through the model: `Children`, `Siblings`
and `Leaves` list them by relation to the focus node; `BestNode`,
`BestChild`, `BestSibling`, `BestLeaf`, `PrioChild`, `PrioSibling` and
`BestBoundNode` pick one and return nil when there is none.
`NodeGetNAddedConss(&node)` counts the constraints attached to a node.

Bound changes for a specific node are made with `SetLbNode(&node, v, lb)`
and `SetUbNode(&node, v, ub)`, constraints with `AddConsNode(&node, cons)`
and, for the focus node, `AddConsLocal(cons)`. `CreateChild` creates a
child of the focus node; a [branch rule](plugins/branch-rules.md) uses it
for custom branching.

## The LP

| Method | Returns |
| --- | --- |
| `LpStatus` | `LPStatusOptimal`, `LPStatusInfeasible`, `LPStatusUnbounded`, `LPStatusIterLimit`, ... |
| `LpObjVal` | Objective value of the current LP |
| `CurrentVal(v)` | The variable's value in the current LP solution, or pseudo solution when no LP was solved |
| `NLpIterations` | Simplex iterations so far |
| `Eps`, `Eq`, `Lt`, `Le`, `Gt`, `Ge` | SCIP's epsilon and tolerance-aware comparisons |

Use the tolerance comparisons instead of `==` on LP values: `model.Eq(x,
1)` is true for `0.9999999`.

## Variables during the solve

`model.Vars()` returns the transformed variables, the ones the LP works
with. Each answers:

| Method | Returns |
| --- | --- |
| `LbLocal`, `UbLocal` | Bounds at the current node |
| `LbGlobal`, `UbGlobal` | Bounds valid everywhere |
| `Lb`, `Ub` | The local bounds |
| `Status` | `VarStatusColumn` for a variable in the LP, `VarStatusFixed`, `VarStatusAggregated`, ... |
| `IsActive`, `IsInLP`, `IsDeleted` | Status queries |
| `Col` | The LP column, if the variable is a column variable |
| `Redcost` | Reduced cost and whether one is available, which requires the node LP to be solved |
| `Transformed` | For an original variable: its transformed counterpart |

Presolving may fix, aggregate or delete variables, so the transformed set
differs from the original one. Hold on to the original `Variable` values
from model construction and call `Transformed` when you need the working
one; or resolve a branching candidate with `VarInProb`.

## Rows

A `Row` is a linear inequality in the LP: a constraint's row, a cut, or a
row you created.

| Method | Returns |
| --- | --- |
| `Name`, `Lhs`, `Rhs` | Identity and sides |
| `Cols`, `NNonZeroes` | The columns with nonzero coefficients |
| `Dual`, `FarkasDual` | Dual value in the current LP, or Farkas multiplier for an infeasible LP |
| `BasisStatus` | `BasisStatusBasic`, `BasisStatusLower`, `BasisStatusUpper`, `BasisStatusZero` |
| `IsInLP`, `IsInGlobalCutPool`, `LpPosition` | Where the row currently is |
| `IsLocal`, `IsModifiable`, `IsRemovable`, `IsIntegral` | Flags |
| `Age`, `ActiveLPCount`, `NLpSinceCreate`, `Depth`, `Rank` | Statistics SCIP keeps for cut management |
| `OriginType`, `Constraint` | Who created the row, and the constraint if it came from one |
| `SetCoeff`, `SetRank` | Mutators, for rows you created |

A constraint's row is reached through `Constraint.Row`; for a linear
constraint `DualSol` and `FarkasDualSol` on the constraint itself are
shortcuts to the row's duals.

Create a row with the builder while solving:

```go
row := scip.NewRow().Name("cut").Le(1).Local(false).Removable(true).
	Source(scip.SourceSeparator(sepa)).AddToSolving(model)
row.SetCoeff(x, 1)
row.SetCoeff(y, 1)
infeasible := model.AddCut(row, false)
```

`Source` attributes the row to a separator, constraint handler or
constraint, which is what SCIP's statistics report. `SeparatorPlugin` and
`ConshdlrPlugin` also have `CreateEmptyRow` for the same purpose. Rows
can also be added to a probing or diving LP with `AddRow` on the session.

## Columns

A `Col` is a variable's column in the LP, reached with `Variable.Col`:

| Method | Returns |
| --- | --- |
| `Var`, `Index`, `VarProbindex` | Identity |
| `Lb`, `Ub`, `Obj` | Bounds and objective in the LP |
| `PrimalSol`, `Redcost` | LP value and reduced cost (with availability) |
| `BasisStatus`, `IsInLP`, `LpPos`, `LpDepth` | Position in the LP |
| `Rows`, `Vals`, `NNonZeros`, `NLpNonZeros` | The nonzero entries |
| `MinPrimalSol`, `MaxPrimalSol`, `Age`, `NStrongBranches`, `StrongBranchingNode` | History |
| `IsIntegral`, `IsRemovable` | Flags |

Rows, columns and nodes exist only inside the solve. After `FreeTransform`
they report an error; after the solve ends but before `FreeTransform` the
LP data is still there for statistics.
