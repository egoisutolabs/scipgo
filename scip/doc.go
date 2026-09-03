// Copyright (c) 2026 Egoisuto Labs. MIT License, see LICENSE.
// Ported from russcip (https://github.com/scipopt/russcip), Apache-2.0.

// Package scip is a Go binding for SCIP, the solver for mixed integer
// (nonlinear) programming from Zuse Institute Berlin.
//
// It links against an installed libscip through cgo; SCIP 10 is required and
// is not bundled. The API follows the Rust crate russcip closely: a Model is
// built with AddVar/AddCons or the builder types (NewVar, NewCons, NewRow),
// solved with Solve, and queried for solutions and statistics.
//
//	model := scip.DefaultModel().Minimize()
//	x := model.AddVar(0, 1, 1, "x", scip.VarTypeBinary)
//	y := model.AddVar(0, 1, 2, "y", scip.VarTypeBinary)
//	model.AddCons([]scip.Variable{x, y}, []float64{1, 1}, 1, 1, "c")
//	solved := model.Solve()
//	fmt.Println(solved.Status(), solved.ObjVal())
//
// Nonlinear constraints are stated as expression trees: Variable.Expr, Const,
// Sum, Product, Pow and friends build an Expr, and Model.AddConsNonlinear or
// ConsBuilder.Expression adds it. ParseExpr accepts SCIP's own syntax.
//
// Custom plugins (BranchRule, Conshdlr, Eventhdlr, Heuristic, NodeSel,
// Pricer, Separator) are Go interfaces registered through Model.Add or the
// Include* methods. A plugin that also implements Copyable is copied into the
// sub-SCIPs SCIP creates for LNS heuristics and SolveConcurrent workers.
// Panics inside a callback are captured and re-raised from the enclosing
// Solve.
//
// A Model and every handle derived from it belong to one goroutine. SCIP
// instances are released by a finalizer, or immediately by Model.Free, which
// is preferable in long-running services. Concurrent solves are serialised
// across goroutines because SCIP's thread pool is a process-wide global.
package scip
