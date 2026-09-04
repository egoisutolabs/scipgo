package scip

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryNativeCallKeepsItsOwnerAlive fails if a function that enters C
// through a Scip or a handle, received or passed as a parameter, lacks
// runtime.KeepAlive on the wrapper. The instance registry holds only weak
// pointers, so without it the finalizer may free the SCIP instance while
// the C call is still using it.
func TestEveryNativeCallKeepsItsOwnerAlive(t *testing.T) {
	receivers := map[string]bool{"Scip": true, "Model": true, "Variable": true, "Constraint": true, "Solution": true,
		"Node": true, "Row": true, "Col": true, "Prober": true, "Diver": true, "BranchRulePlugin": true,
		"ConshdlrPlugin": true, "EventhdlrPlugin": true, "HeurPlugin": true, "NodeselPlugin": true,
		"PricerPlugin": true, "PresolverPlugin": true, "SeparatorPlugin": true}
	files, _ := filepath.Glob("*.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "callbacks.go" || file == "cgo.go" {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			// the wrapper type: the receiver, or else the first parameter of a wrapper type
			var fields []*ast.Field
			if fn.Recv != nil {
				fields = fn.Recv.List
			} else {
				fields = fn.Type.Params.List
			}
			var id *ast.Ident
			for _, fld := range fields {
				rt := fld.Type
				if star, ok := rt.(*ast.StarExpr); ok {
					rt = star.X
				}
				if cand, ok := rt.(*ast.Ident); ok && receivers[cand.Name] {
					id = cand
					break
				}
			}
			if id == nil || livenessChecks[fn.Name.Name] || checkHelpers[fn.Name.Name] {
				continue // the checks themselves; their callers pin the root
			}
			entersC, keeps := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				// a liveness check resolves the weak owner; the call that follows
				// must share that strong reference, so it counts as entering C
				if livenessChecks[sel.Sel.Name] {
					entersC = true
				}
				if x, ok := sel.X.(*ast.Ident); ok {
					if x.Name == "C" && (strings.HasPrefix(sel.Sel.Name, "SCIP") || strings.HasPrefix(sel.Sel.Name, "scipgo")) {
						entersC = true
					}
					if x.Name == "runtime" && sel.Sel.Name == "KeepAlive" {
						keeps = true
					}
				}
				// the pinned value must be the strong instance (.root()), never a
				// weak wrapper whose owner could still be collected mid-call
				if call, ok := n.(*ast.CallExpr); ok {
					if fsel, ok := call.Fun.(*ast.SelectorExpr); ok && fsel.Sel.Name == "KeepAlive" && len(call.Args) == 1 {
						if arg, ok := call.Args[0].(*ast.CallExpr); !ok || !isRootCall(arg) {
							t.Errorf("%s: %s.%s pins something other than <wrapper>.root()", fset.Position(call.Pos()), id.Name, fn.Name.Name)
						}
					}
				}
				return true
			})
			if entersC && !keeps {
				t.Errorf("%s: %s.%s enters C without runtime.KeepAlive on its wrapper", fset.Position(fn.Pos()), id.Name, fn.Name.Name)
			}
		}
	}
}

// livenessChecks are the methods that resolve a weak owner into a temporary
// strong reference; a function calling one and then touching the instance
// must pin the root for its whole body.
var livenessChecks = map[string]bool{"guard": true, "query": true, "checkHandle": true, "checkVars": true,
	"checkCons": true, "checkNode": true, "live": true, "mustLive": true, "mustStage": true, "active": true,
	"varOp": true, "rowOp": true, "varGet": true, "lpSolved": true, "alive": true, "stage": true}

// checkHelpers build errors for the checks and are only reached through a
// pinned caller.
var checkHelpers = map[string]bool{"wrap": true, "invalid": true, "handleErr": true, "call": true}

func isRootCall(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "root" && len(c.Args) == 0
}
