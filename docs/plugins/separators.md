# Separators

A separator looks at the current LP solution and adds cutting planes that
it violates but every integer solution satisfies. Cuts tighten the LP
relaxation and shrink the tree.

## Interface

```go
type Separator interface {
	ExecuteLP(model Model, sepa SeparatorPlugin) SeparationResult
}
```

The result:

| Result | Meaning |
| --- | --- |
| `SeparationResultSeparated` | At least one cut was added |
| `SeparationResultDidNotFind` | Searched, no violated cut |
| `SeparationResultDidNotRun` | Skipped |
| `SeparationResultDelayed` | Skipped, call again later |
| `SeparationResultNewRound` | Start a new separation round |
| `SeparationResultConsAdded` | A constraint was added instead of a cut |
| `SeparationResultReducedDomain` | A bound was tightened |
| `SeparationResultCutoff` | The node is infeasible |

## Registration

```go
model.Add(scip.NewSeparator(&clique{}).
	Name("clique").
	Desc("clique cuts on the conflict graph").
	Priority(100000).    // default
	Freq(1).             // default: every depth; 0 root only; -1 off
	MaxBoundDist(1.0).   // default: every node
	UsesSubscip(false).  // default
	Delay(false))        // default
```

`Freq(0)` restricts separation to the root, which is where cuts pay off
most and is a common choice for expensive separators. `Delay(true)` runs
the separator only if no other separator found a cut in the round.

## Building and adding a cut

```go
func (s *clique) ExecuteLP(model scip.Model, sepa scip.SeparatorPlugin) scip.SeparationResult {
	vars := model.Vars()
	members := s.findViolatedClique(model, vars) // indices into vars
	if members == nil {
		return scip.SeparationResultDidNotFind
	}

	row, err := sepa.CreateEmptyRow(model, "clique", scip.NegInfinity, 1, false, false, true)
	if err != nil {
		panic(err)
	}
	for _, i := range members {
		row.SetCoeff(vars[i], 1)
	}
	infeasible := model.AddCut(row, false)
	if infeasible {
		return scip.SeparationResultCutoff
	}
	return scip.SeparationResultSeparated
}
```

`CreateEmptyRow(model, name, lhs, rhs, local, modifiable, removable)`
makes a row attributed to the separator, so SCIP's statistics credit it.
The flags: `local` marks a cut valid only in the current subtree,
`modifiable` allows coefficients to change later, and `removable` lets
SCIP drop the cut from the LP when it goes slack, which is the usual
setting for cuts. `SetCoeff` fills it in. `AddCut(row, force)` adds it to
the separation storage; `force` bypasses SCIP's efficacy filtering, which
otherwise discards cuts that are not violated enough. The boolean it
returns says the cut is infeasible given the node's bounds, in which case
report `Cutoff`.

`scip.NewRow()` is the builder equivalent, with `Source(scip.SourceSeparator(sepa))`
for attribution and `AddToSolving(model)` to create it; see
[Tree and LP](../tree-and-lp.md#rows).

## Example

`examples/clique_separator` builds the conflict graph of a set
partitioning problem (two variables conflict if they share an equality
row), greedily finds a clique, and adds `sum <= 1` over it when the LP
solution violates it. With SCIP's own separators off, the cut closes the
gap at the root.

## Notes

- Only violated cuts help. Check the LP value of the cut before adding it;
  `CurrentVal` gives the variable values.
- Do not keep a `Row` handle across callbacks. SCIP releases a cut once
  it leaves the LP and the cut pool, and the binding cannot tell when that
  happened. Keep the cut's definition in your own data instead and rebuild
  the row when it is violated again.
- Within the callback that created it, `Row.Dual`, `Row.BasisStatus` and
  `Row.ActiveLPCount` on rows returned by `Constraint.Row` or `Col.Rows`
  tell how useful existing rows have been.
- A separator that implements `Copyable` also separates inside sub-MIP
  heuristics, which is usually beneficial for cheap separators, provided
  it works from the `Model` it is called with rather than from handles of
  the parent model; see the [plugin overview](README.md#copies-and-sub-scips).
