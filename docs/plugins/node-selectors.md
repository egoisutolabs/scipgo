# Node selectors

A node selector decides which open node of the branch-and-bound tree is
processed next. SCIP ships depth-first, best-first, best-estimate and
hybrid selectors; a custom one implements a search strategy those do not
cover.

## Interface

```go
type Nodesel interface {
	// Select returns the next node, or nil to fall back to the best node by Comp.
	Select(model Model) *Node
	// Comp orders two open nodes: -1 if node1 goes first, +1 if node2, 0 if equal.
	Comp(node1, node2 Node) int
}
```

`Comp` defines the total order SCIP keeps its node queue in; `Select` is
free to pick any open node, and returning nil lets SCIP take the best
node under that order.

## Registration

```go
model.Add(scip.NewNodesel(dfs{}).
	Name("dfs").
	Desc("depth first").
	StdPriority(1000000).      // default
	MemSavePriority(1000000))  // default
```

SCIP uses the selector with the highest standard priority, and switches to
the highest memory-save priority when memory runs low. The default of one
million exceeds every built-in selector, so a registered custom selector
is the active one without further configuration.

## Choosing a node

The model exposes the open nodes relative to the focus node:

| Method | Returns |
| --- | --- |
| `PrioChild`, `PrioSibling` | The child or sibling with the highest node selection priority, or nil |
| `BestChild`, `BestSibling`, `BestLeaf` | The best child, sibling or leaf under the active selector's `Comp`, or nil |
| `BestNode` | The best open node overall, or nil |
| `BestBoundNode` | The open node with the lowest bound, or nil |
| `Children`, `Siblings`, `Leaves` | Every open node in that category |

A node answers `Number`, `Depth`, `LowerBound`, `Parent`, `NChildren` and
`Children`; see [Tree and LP](../tree-and-lp.md).

## Example: depth-first search

```go
type dfs struct{}

func (dfs) Select(model scip.Model) *scip.Node {
	if n := model.PrioChild(); n != nil {
		return n
	}
	if n := model.PrioSibling(); n != nil {
		return n
	}
	return model.BestLeaf()
}

func (dfs) Comp(a, b scip.Node) int {
	switch {
	case a.Depth() > b.Depth():
		return -1
	case a.Depth() < b.Depth():
		return 1
	case a.LowerBound() < b.LowerBound():
		return -1
	case a.LowerBound() > b.LowerBound():
		return 1
	}
	return 0
}
```

Prefer a child, then a sibling, then the best remaining leaf; order the
leaf queue deepest first with the bound as tie-breaker. The full program is
under `examples/depth_first_node_selection`.

## Notes

- `Comp` is the hot path: it runs every time the node queue is updated.
  Keep it to a handful of accessor calls.
- `Comp` must be a consistent total order; an inconsistent one corrupts
  the queue.
- `Select` runs once per node, so a lookup against your own bookkeeping is
  affordable there.
