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
			if id == nil {
				continue
			}
			entersC, keeps := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if x, ok := sel.X.(*ast.Ident); ok {
					if x.Name == "C" && (strings.HasPrefix(sel.Sel.Name, "SCIP") || strings.HasPrefix(sel.Sel.Name, "scipgo")) {
						entersC = true
					}
					if x.Name == "runtime" && sel.Sel.Name == "KeepAlive" {
						keeps = true
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
