package main

// usage: uses <dir> <forbidden path substring,...> <pkgpattern>...
// prints "pkg<TAB>file<TAB>forbiddenpkg.Name" for each use of a forbidden
// package's object (test files included).
import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

func main() {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles, Tests: true, Dir: os.Args[1]}
	pkgs, err := packages.Load(cfg, os.Args[3:]...)
	if err != nil {
		panic(err)
	}
	bad := strings.Split(os.Args[2], ",")
	out := map[string]bool{}
	for _, p := range pkgs {
		for _, f := range p.Syntax {
			fn := filepath.Base(p.Fset.File(f.Pos()).Name())
			ast.Inspect(f, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				o := p.TypesInfo.Uses[sel.Sel]
				if o == nil || o.Pkg() == nil || o.Pkg() == p.Types {
					return true
				}
				path := o.Pkg().Path()
				for _, b := range bad {
					if strings.Contains(path, b) {
						out[filepath.Base(p.PkgPath)+"\t"+fn+"\t"+filepath.Base(path)+"."+o.Name()] = true
					}
				}
				return true
			})
		}
	}
	var ks []string
	for k := range out {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Println(k)
	}
}
