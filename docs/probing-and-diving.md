# Probing and diving

Both modes let a plugin explore tentative bound changes with LP support
without touching the real tree. Probing is the more general one; diving
is cheaper and made for heuristics that walk down one path.

Both are started on the model inside a callback, must be ended before the
callback returns, and every operation has a `Try` form that returns the
error the plain form panics with.

## Probing

```go
p := model.StartProbing()
defer p.End()

p.NewNode()            // open a probing level
p.FixVar(x, 1)
cutoff, n := p.Propagate(-1) // propagate to a fixed point; n reductions found
if !cutoff {
	cutoff, err := p.SolveLp(0) // 0: no iteration limit
	// read model.CurrentVal(v), model.LpObjVal(), model.LpStatus()
}
p.Backtrack(0)         // undo everything above depth 0
```

| Method | Effect |
| --- | --- |
| `NewNode` | Push a probing level; changes made after it can be undone together |
| `Backtrack(depth)` | Undo every change above `depth`; `depth` must be at most `Depth()` |
| `Depth` | The current probing depth |
| `FixVar`, `ChgVarLb`, `ChgVarUb` | Tentative bound changes |
| `ChgVarObj`, `VarObj`, `IsObjChanged` | Tentative objective changes |
| `AddRow` | Add a row to the probing LP |
| `Propagate(maxRounds)` | Domain propagation; returns whether the node is cut off and the number of reductions |
| `PropagateImplications` | Implication propagation only |
| `SolveLp(iterLimit)` | Solve the probing LP; returns whether it proved the node cut off |
| `SolveLpWithPricing(maxRounds)` | The same with column generation |
| `End` | Leave probing mode and discard every change |

Probing is what SCIP's own probing presolver and many heuristics use to
test what fixing a variable implies. A heuristic that wants to know
whether a rounding is feasible before submitting it can fix the rounded
variables, propagate, and read the result.

## Diving

```go
d := model.StartDiving()
defer d.End()

d.ChgVarUb(x, 0)
solved, err := d.SolveLp(0)   // solved: reached optimality
if solved && model.LpStatus() == scip.LPStatusOptimal {
	obj := model.LpObjVal()
}
```

| Method | Effect |
| --- | --- |
| `ChgVarLb`, `ChgVarUb`, `ChgVarObj` | Change a bound or objective coefficient in the dive |
| `VarLb`, `VarUb`, `VarObj` | Read the dive's current values |
| `ChgRowLhs`, `ChgRowRhs`, `AddRow` | Change or add rows in the dive LP |
| `ChgCutoffBound` | Change the cutoff bound for the dive |
| `SolveLp(iterLimit)` | Solve the dive LP; returns whether it reached optimality |
| `LastDiveNode` | The number of the node the last dive started at |
| `End` | Leave diving mode; every change is discarded |

Diving has no levels and no propagation; it is a sequence of bound changes
and LP re-solves. That is exactly what a diving heuristic does: round one
variable, re-solve, repeat until integral or infeasible, then submit the
point through `CreateSolFor` and `AddSol` after ending the dive.

## Rules

- Start either mode only from within a callback, while the model is in the
  Solving stage. `scip.InProbing(model)` and `scip.InDive(model)` report
  the current mode.
- End the session before returning from the callback. `End` on a session
  that is not active is an error.
- Do not start one mode inside the other.
- Values read while probing or diving, through `CurrentVal`, `LpObjVal`
  and the row and column accessors, describe the tentative LP, not the
  node's real one.
- `Prober` and `Diver` are pointers to per-session state; do not keep
  them past `End`.
