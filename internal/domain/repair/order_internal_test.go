package repair

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// problemOrder は、problems.go で宣言した Problem の定数を、宣言の順にすべて持つ。
func TestProblemOrderHasEveryProblem(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "problems.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var declared []Problem
	for _, d := range file.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.CONST {
			for _, spec := range g.Specs {
				declared = append(declared, problemsOf(t, spec)...)
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("Problem の定数が見つからない")
	}
	if !slices.Equal(problemOrder, declared) {
		t.Errorf("problemOrder = %v, want %v", problemOrder, declared)
	}
}

// problemsOf は、宣言 spec が Problem の定数なら、その値。
func problemsOf(t *testing.T, spec ast.Spec) []Problem {
	t.Helper()
	v, ok := spec.(*ast.ValueSpec)
	if !ok {
		return nil
	}
	if id, ok := v.Type.(*ast.Ident); !ok || id.Name != "Problem" {
		return nil
	}
	var out []Problem
	for _, val := range v.Values {
		lit, ok := val.(*ast.BasicLit)
		if !ok {
			t.Fatalf("Problem の値が文字列でない: %T", val)
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, Problem(s))
	}
	return out
}
