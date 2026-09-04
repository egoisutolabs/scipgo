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

// TestHandlesAreBuiltByConstructors fails if any non-test file builds a
// handle with a struct literal outside its constructor. A literal bypasses
// the transform generation and would make the handle dead on arrival after
// FreeTransform (or immortal, which is worse).
func TestHandlesAreBuiltByConstructors(t *testing.T) {
	ctors := map[string]string{"Variable": "newVar", "Constraint": "newCons", "Solution": "newSol",
		"Node": "newNode", "Row": "newRow", "Col": "newCol"}
	files, _ := filepath.Glob("*.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
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
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				id, ok := cl.Type.(*ast.Ident)
				if !ok {
					return true
				}
				if ctor, isHandle := ctors[id.Name]; isHandle && len(cl.Elts) > 0 && fn.Name.Name != ctor {
					t.Errorf("%s: %s literal in %s; use %s so the handle carries its transform generation",
						fset.Position(cl.Pos()), id.Name, fn.Name.Name, ctor)
				}
				return true
			})
		}
	}
}
