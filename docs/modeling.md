# Modeling

How to state a problem: variables, the objective, every constraint kind the
binding exposes, nonlinear expressions, and reading and writing files.

All of this happens in the Problem stage, which the model is in after
`CreateProb` or `ReadProb` and again after `FreeTransform`. Adding to a
model while it is solving is possible from inside plugin callbacks and is
covered in [Plugins](plugins/README.md).

## Variables

```go
x := model.AddVar(lb, ub, obj, "x", scip.VarTypeInteger)
```

| Type | Meaning |
| --- | --- |
| `VarTypeContinuous` | Real-valued within its bounds |
| `VarTypeInteger` | Integer within its bounds |
| `VarTypeBinary` | Integer in `{0, 1}`; bounds must lie within `[0, 1]` |
| `VarTypeImplInt` | Continuous, but SCIP may treat it as integral because the constraints force it |

Use `scip.Infinity` and `scip.NegInfinity` for unbounded sides. SCIP treats
any magnitude of at least `1e20` as infinite.

The builder covers the same ground with named steps:

```go
x := scip.NewVar().Name("x").IntRange(0, 10).Obj(2.5).AddTo(model)
b := scip.NewVar().Name("b").Bin().AddTo(model)
c := scip.NewVar().Name("c").ContRange(-1, 1).AddTo(model)
```

| Builder method | Effect |
| --- | --- |
| `Bin()` | Binary with bounds `[0, 1]` |
| `Int()`, `Cont()`, `ImplInt()` | Set the type, keep the current bounds |
| `IntRange(lb, ub)`, `ContRange(lb, ub)`, `ImplIntRange(lb, ub)` | Set type and bounds together |
| `Bounds(lb, ub)` | Set bounds only |
| `Obj(c)` | Objective coefficient |
| `Name(s)` | Name, used by `FindCons`, file writers and `AsNameMap` |

A `Variable` is a small value type wrapping SCIP's pointer. Copy it freely;
it stays valid as long as the model and the problem it belongs to exist
(see [Lifecycle](lifecycle.md)). It answers `Name`, `Index`, `VarType`,
`Obj`, `Lb`, `Ub`, `LbGlobal`, `UbGlobal` and a set of status queries such
as `IsActive` and `IsTransformed`.

## Objective

```go
model.Minimize()             // the default
model.Maximize()
model.SetObjSense(scip.ObjSenseMaximize)
model.SetObjIntegral()       // every solution has an integral objective value
```

The objective is the sum of each variable's objective coefficient times
its value. `SetObjIntegral` lets SCIP prove optimality as soon as the dual
bound is within one unit of the incumbent; it is worth setting whenever
all coefficients are integers and all variables with nonzero coefficients
are integral.

## Linear constraints

```go
model.AddCons([]scip.Variable{x, y}, []float64{2, 1}, scip.NegInfinity, 100, "c1")
```

states `-inf <= 2x + y <= 100`. Both sides are always given; an equality
passes the same value twice. The builder spells the sides out:

```go
model.Add(scip.NewCons().Name("c1").Coef(x, 2).Coef(y, 1).Le(100))
model.Add(scip.NewCons().Name("eq").Coefs(vars, coefs).Eq(1))
model.Add(scip.NewCons().Expr(scip.CoefPair{Var: x, Coef: 1}, scip.CoefPair{Var: y, Coef: 1}).Bounds(1, 5))
```

`Coef` appends one term, `Coefs` a parallel pair of slices, `Expr` a list
of `CoefPair`. `Eq`, `Le`, `Ge` and `Bounds` set the sides; the last call
wins. Coefficients can be added to an existing linear constraint later
with `AddConsCoef`, which is how column generation attaches priced
variables.

Three flags control how SCIP treats a constraint during the solve:

| Flag | Default | Meaning |
| --- | --- | --- |
| `Modifiable(true)` | `false` | Variables may be added during the solve. Required for constraints a [pricer](plugins/pricers.md) extends |
| `Removable(true)` | `false` | SCIP may drop the constraint's row from the LP when it is inactive |
| `Separated(false)` | `true` | Whether the constraint is separated during LP processing |

They can be read back and changed on the model (`ConsIsModifiable`,
`SetConsModifiable` and so on).

## Specialised constraint types

SCIP has dedicated constraint handlers for common structures. They
propagate and separate better than a linear encoding, so prefer them when
the structure is there:

| Method | Constraint |
| --- | --- |
| `AddConsSetPart(vars, name)` | Exactly one of the binary variables is 1 |
| `AddConsSetPack(vars, name)` | At most one is 1 |
| `AddConsSetCover(vars, name)` | At least one is 1 |
| `AddConsCardinality(vars, k, name)` | At most `k` of the variables are nonzero |
| `AddConsSOS1(vars, weights, name)` | At most one variable is nonzero (weights order the branching) |
| `AddConsIndicator(b, vars, coefs, rhs, name)` | `b = 1` implies `sum(coefs * vars) <= rhs` |
| `AddConsQuadratic(linVars, linCoefs, qVars1, qVars2, qCoefs, lhs, rhs, name)` | `lhs <= linear + sum(qCoefs * qVars1 * qVars2) <= rhs` |

`AddConsCoefSetppc` adds a variable to an existing set partitioning,
packing or covering constraint.

## Nonlinear constraints

Nonlinear constraints are stated as expression trees. An `Expr` is
immutable, built in Go, and only sent to SCIP when a constraint is added.
A tree that contains `Variable.Expr()` is bound to that variable's model;
adding it to another model is rejected with `RetcodeInvalidData`. A tree
built from `Const` and `ParseExpr` alone carries no variables and can be
built before any model exists and added to several, since `ParseExpr`
resolves names in whichever model the constraint goes into.

```go
x := model.AddVar(-1, 1, 1, "x", scip.VarTypeContinuous)
y := model.AddVar(-1, 1, 1, "y", scip.VarTypeContinuous)

// x^2 + y^2 <= 1
disc := x.Expr().Pow(2).Add(y.Expr().Pow(2))
model.AddConsNonlinear(disc, scip.NegInfinity, 1, "disc")
```

Every expression starts from `Variable.Expr()` or `scip.Const(c)`. From
there, methods chain:

| Method | Expression |
| --- | --- |
| `Add`, `Sub`, `Mul`, `Div` | `e + o`, `e - o`, `e * o`, `e * o^-1` |
| `Neg`, `Scale(c)` | `-e`, `c * e` |
| `Pow(p)`, `Sqrt` | `e^p` for constant `p`, `e^0.5` |
| `Exp`, `Log` | `exp(e)`, natural `log(e)` |
| `Sin`, `Cos`, `Abs` | as named |

The package-level functions take several operands at once: `Sum(terms...)`,
`Product(factors...)`, `WeightedSum(coefs, terms)`, and additionally
`SignPower(e, p)` for `sign(e) * |e|^p` and `Entropy(e)` for `-e * log(e)`.
`String` renders the tree, which is handy in tests.

SCIP's own text syntax is accepted too. Variables are written as `<name>`
and resolved by name in the model the constraint is added to:

```go
model.AddConsNonlinear(scip.ParseExpr("<x>^2 + <y>^2"), scip.NegInfinity, 1, "disc")
```

The constraint builder mixes a nonlinear expression with a linear part:

```go
model.Add(scip.NewCons().Name("mixed").Expression(x.Expr().Mul(y.Expr())).Coef(x, 2).Le(1))
// x*y + 2x <= 1
```

Nonlinear constraints turn the problem into a MINLP. SCIP solves those to
global optimality with spatial branch-and-bound, which is much slower than
MIP; keep the nonlinear part as small as the model allows.

## Local constraints

Inside a plugin callback, a constraint can be added to the current node
only, or to a specific node, so that it applies in that subtree alone:

```go
model.AddConsLocal(scip.NewCons().Coef(x, 1).Le(3))
model.AddConsNode(&child, scip.NewCons().Coef(x, 1).Le(3))
```

`AddConsLocal` is the usual way for a [constraint handler](plugins/constraint-handlers.md)
to cut off a subtour or a [branching rule](plugins/branch-rules.md) to
implement a custom disjunction. The returned handle belongs to the node:
SCIP frees the constraint with the subtree, and the binding does not track
that, so use the handle inside the callback that created it and not later.

## Reading and writing files

```go
model, err := scip.NewModel().IncludeDefaultPlugins().ReadProb("instance.lp")
```

`ReadProb` picks the reader by extension. The formats SCIP's default plugins
read include `.lp`, `.mps`, `.opb`, `.cip`, `.fzn`, `.pip`, `.wbo` and,
when SCIP was built with it, `.zpl`. Compressed `.gz` input works when
SCIP was built with zlib. On failure the zero `Model` is returned with the
error; the receiver may then hold a partially read problem, so call
`CreateProb` or `FreeTransform` before reusing it.

```go
err := model.Write("instance.lp", "lp", true)
```

`Write` writes the original problem. The first argument is the file name
exactly as it will be created; the second selects the writer, so `"lp"`,
`"mps"` and `"cip"` produce those formats regardless of the file name. The
last argument keeps your variable and constraint names when true and
generates `x0, x1, ...` and `c0, c1, ...` when false, which is useful for
sharing an instance without revealing what it models.

## Querying the model

| Method | Returns |
| --- | --- |
| `NVars`, `NConss` | Counts |
| `Vars` | All variables of the transformed problem once it exists, otherwise the original ones |
| `OrigVars` | The original variables, from the Problem stage on (before `CreateProb` or `ReadProb` there are none) |
| `Conss` | All constraints |
| `FindCons(name)` | A constraint by name |
| `Var(id)`, `VarInProb(i)` | A variable by index |
| `Stage` | The current [stage](lifecycle.md#stages) |

After a solve, `Vars` returns transformed variables whose names carry a
`t_` prefix. Read solution values through the original variables you hold
or through `OrigVars` when the names matter.
