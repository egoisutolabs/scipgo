# Examples

Each directory is a standalone program. Run one from inside its directory
so the relative paths to `data/test` resolve:

```bash
cd examples/knapsack && go run .
```

Every example checks its own result and exits nonzero on a mismatch; CI
builds all of them.

## Modeling and solving

| Example | Shows |
| --- | --- |
| [`create_and_solve`](create_and_solve) | A two-variable MIP built with `AddVar` and `AddCons`, solved, and read back |
| [`knapsack`](knapsack) | 0/1 knapsack with the variable builder; reading which items were chosen |
| [`concurrent_solve`](concurrent_solve) | Reading an MPS file and solving it with SCIP's concurrent solvers on every core, in deterministic or opportunistic mode |

## Custom plugins

| Example | Plugin kind | Shows |
| --- | --- | --- |
| [`most_infeasible_branching`](most_infeasible_branching) | Branch rule | Picking the candidate closest to one half; resolving candidates to variables |
| [`depth_first_node_selection`](depth_first_node_selection) | Node selector | A `Select` that dives (child, sibling, best leaf) and a `Comp` that orders by depth |
| [`node_event_handler`](node_event_handler) | Event handler | Counting `NodeFocused` events and checking the count against `NNodes` |
| [`random_rounding`](random_rounding) | Heuristic | Rounding the LP solution, submitting it with `CreateSolFor` and `AddSol`, and reading the heuristic's statistics |
| [`clique_separator`](clique_separator) | Separator | Building a conflict graph, finding a clique and adding the clique cut as a row |
| [`tsp`](tsp) | Constraint handler | Subtour elimination for a 280-city TSP: `Check` on integral solutions, `Enforce` adding cuts |

## Branch-and-price

| Example | Shows |
| --- | --- |
| [`cutting_stock`](cutting_stock) | Column generation with a pricer that solves a knapsack in a nested model; pricing at the root only |
| [`bin_packing`](bin_packing) | Pricer plus a Ryan-Foster branching rule; branching decisions stored in the datastore per node and enforced in the pricer |

The `tsp` example is derived from russcip's; `bin_packing` and
`cutting_stock` follow the SCIP book's treatment of the problems. The
instances under `data/test` come from russcip's test suite.
