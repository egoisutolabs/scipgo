# Plugins

SCIP is built from plugins: branching rules, heuristics, separators,
pricers, constraint handlers, event handlers and node selectors, among
others. scipgo lets you write seven of those kinds in Go and register them
alongside SCIP's own. This page covers what they share; each kind has its
own page.

| Kind | Interface | Builder | Wrapper | Page |
| --- | --- | --- | --- | --- |
| Branching rule | `BranchRule` | `NewBranchRule` | `BranchRulePlugin` | [Branch rules](branch-rules.md) |
| Constraint handler | `Conshdlr` | `IncludeConshdlr` (no builder) | `ConshdlrPlugin` | [Constraint handlers](constraint-handlers.md) |
| Event handler | `Eventhdlr` | `NewEventhdlr` | `EventhdlrPlugin` | [Event handlers](event-handlers.md) |
| Primal heuristic | `Heuristic` | `NewHeuristic` | `HeuristicPlugin` | [Heuristics](heuristics.md) |
| Node selector | `Nodesel` | `NewNodesel` | `NodeselPlugin` | [Node selectors](node-selectors.md) |
| Pricer | `Pricer` | `NewPricer` | `PricerPlugin` | [Pricers](pricers.md) |
| Separator | `Separator` | `NewSeparator` | `SeparatorPlugin` | [Separators](separators.md) |

## Writing one

A plugin is any Go value that implements the interface. Register it with
its builder through `Model.Add` or `AddTo`, in the Problem stage:

```go
type firstCandidate struct{}

func (firstCandidate) Execute(model scip.Model, _ scip.BranchRulePlugin,
	cands []scip.BranchingCandidate) scip.BranchingResult {
	return scip.BranchOn(cands[0])
}

model.Add(scip.NewBranchRule(firstCandidate{}).Name("first").Desc("branch on the first candidate"))
```

The builder sets the name, description and the kind-specific settings such
as priority and frequency, with the defaults listed on each page. The
`Include*` methods on `Model` take every setting positionally and are what
the builders call. Names must be unique among plugins of the same kind in
one model, and that includes SCIP's own: registering a heuristic named
`rounding` or a separator named `gomory` fails with `RetcodeInvalidData`
because those exist already. Pick a name that is yours.

Use a pointer receiver when the plugin has state to mutate between calls.
The value you register is the one SCIP calls, so state lives naturally in
its fields.

## Inside a callback

Every callback receives a `Model` that refers to the model being solved,
in the Solving stage (or a presolving stage, for some events), and the
plugin's own wrapper, which reports its name and settings and, for
heuristics and separators, is needed to attribute what the plugin creates.

Through that model a callback can:

- read the current LP or pseudo solution with `CurrentVal`, and inspect the
  [tree, rows and columns](../tree-and-lp.md);
- add constraints to the current node with `AddConsLocal`, to a specific
  node with `AddConsNode`, or globally with `Add`;
- add cuts with `NewRow().AddToSolving` and `AddCut`;
- add variables with `AddPricedVar` (pricers) or `NewVar().AddToSolving`;
- create children with `CreateChild` and tighten their bounds with
  `SetLbNode` and `SetUbNode`;
- create and submit solutions with `CreateSolFor` and `AddSol`;
- run [probing and diving](../probing-and-diving.md) sessions;
- ask the solve to stop with `Interrupt`.

A callback must not call `Solve`, `FreeTransform` or `Free` on the model it
was handed. Creating a separate model inside a callback and solving that is
fine, and is how the cutting stock and bin packing examples solve their
pricing subproblems.

Variable and constraint handles obtained inside a callback stay valid
for the whole solve, not just the callback, and report an error rather
than crashing once the transformed problem is gone. Node and row handles
are different: SCIP frees a node once it is processed and releases a row
once it leaves the LP and the cut pool, and the binding does not track
either event. Use a `Node` or `Row` only while SCIP still exposes it,
that is during the callback that obtained it or while it is listed by the
tree and LP accessors, and store stable data such as `Node.Number()` for
anything you need later. The bin packing example keys its branching
decisions by node number for exactly this reason.

## Sharing data with plugins

Plugins often need the problem data that built the model: which variable
corresponds to which pattern, which constraints are the demand rows. Two
ways to give it to them:

**Fields on the plugin.** Put the data in the struct you register. Simple
and type-safe; it is what the TSP example does with its edge list.

**The model datastore.** `scip.SetData` attaches a value to the model,
keyed by its Go type, and `scip.GetData` and `scip.MustGetData` fetch it
from any `Model` value for that instance, including the one a callback
receives and the ones sub-SCIP copies receive:

```go
type patterns struct{ byVar map[int][]int }

scip.SetData(model, &patterns{byVar: map[int][]int{}})

// in a callback:
p := scip.MustGetData[*patterns](model)
p.byVar[v.Index()] = items
```

Store a pointer when the data is mutated later. One value per type is
kept; setting a second value of the same type replaces the first. The
datastore is released with the model.

The datastore is one per model and is shared with every sub-SCIP copy,
including the workers of `SolveConcurrent`. A `Copyable` plugin running in
those workers therefore sees the same pointers from several threads at
once, and an unguarded map write like the one above can race or crash the
process. Either protect mutable datastore values with a mutex, or keep
mutable per-worker state in the fresh value `Copy` returns and use the
datastore for data that is read-only during the solve. A plugin without
`Copy` never runs in a worker and needs neither.

## Copies and sub-SCIPs

SCIP creates copies of the model for its large neighbourhood search
heuristics (RENS, RINS, crossover and so on) and for each `SolveConcurrent`
worker. A C plugin participates in those copies through a copy callback;
a Go plugin does so by implementing `Copyable`:

```go
func (c *myConshdlr) Copy() any { return c }            // stateless: share
func (h *myHeur) Copy() any     { return &myHeur{rng: newRNG()} } // stateful: fresh
```

`Copy` returns the value to register in the copy. Return the receiver
only for a plugin that holds neither mutable state nor handles: a copy
runs against a different SCIP instance, and a `Variable`, `Constraint` or
`Model` of the parent passed to that instance is rejected with
`RetcodeInvalidData`, which surfaces as a `*scip.CallbackPanic`. A plugin
that holds handles must resolve them again from the `Model` its callbacks
receive, for example by matching `model.Vars()` on names once per copy,
and keep the result in the fresh value `Copy` returns. A plugin with
mutable state needs a fresh value too, since a copy inside a concurrent
worker runs on that worker's thread at the same time as the other copies
and the original.

Plugins without `Copy` never run inside sub-SCIPs. For most kinds that is
harmless: the sub-MIP simply runs without your heuristic or branching
rule. For a constraint handler it is not: SCIP marks every copy of a model
with a non-copyable constraint handler as invalid, which disables the
sub-MIP heuristics entirely. Implement `Copy` on constraint handlers unless
you specifically want that.

Panics inside `Copy` surface from the `Solve` of the model you hold, and
`GetData` inside a copy reads that model's datastore.

## Panics

A panic inside a callback cannot unwind through SCIP's C frames. The
binding recovers it, makes the callback report an error to SCIP, and
re-raises it as a `*scip.CallbackPanic` from the `Solve` (or returns it
from `TrySolve`) once SCIP has unwound. Panicking on an invariant
violation inside a plugin is therefore safe and is the idiom the examples
use. See [Errors](../errors.md#scipcallbackpanic).

## Priorities and frequencies

SCIP consults plugins of one kind in descending priority order, and a
frequency of `f` means the plugin runs at every `f`th depth of the tree
(1 at every node, -1 never). The defaults the builders use are chosen so
that a custom plugin runs, and for most kinds runs first:

| Builder | Priority | Other defaults |
| --- | --- | --- |
| `NewBranchRule` | 100000 | `MaxDepth(-1)`, `MaxBoundDist(1.0)` |
| `NewHeuristic` | 100000 | `Freq(1)`, `FreqOfs(0)`, `MaxDepth(-1)`, `Timing(HeurTimingBeforeNode)`, `DispChar('?')`, `UsesSubscip(false)` |
| `NewNodesel` | 1000000 std and memsave | Higher than every built-in selector, so the custom one is used |
| `NewPricer` | 100000 | `Delay(false)` |
| `NewSeparator` | 100000 | `Freq(1)`, `MaxBoundDist(1.0)`, `UsesSubscip(false)`, `Delay(false)` |

For comparison, in SCIP 10 the default branching rule `relpscost` has
priority 10000, the built-in heuristics range from -3000010 (`trysol`) to
75000 (`dps`), and the built-in separators from -1200000
(`rapidlearning`) to 1000000 (`closecuts`). So a custom branching rule,
heuristic or node selector at the default priority runs before every
built-in one, while a custom separator at 100000 runs after `closecuts`;
raise its priority if it must go first. A constraint handler's two
priorities are explained on its page.

## Performance

Each callback is a cgo crossing plus whatever the callback does. The
crossing costs on the order of a hundred nanoseconds; a `Vars()` call
inside it allocates a slice of handles. For plugins that run at every node
of a large tree, cache what does not change (variable handles, problem
data) in the plugin's fields or the datastore, and keep per-call
allocation low. The branching rule and node selector are the hottest
callbacks; a node selector's `Comp` in particular is called many times per
node.

## Testing plugins

`scip.MinimalModel()` returns a model with presolving, heuristics and
separators off, so the branch-and-bound tree is exactly what your plugin
sees. Combined with a small instance, such as the ones under `data/test`,
it makes for fast, deterministic plugin tests. The examples under
`examples/` each assert on the outcome of a solve and are run by CI.
