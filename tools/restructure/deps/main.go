package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// usage: deps <pkgpattern> <mapping file: "file.go group" lines> <dir>
// TESTS=1 includes the test files.
func main() {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles, Tests: os.Getenv("TESTS") == "1", Dir: os.Args[3]}
	pkgs, err := packages.Load(cfg, os.Args[1])
	if err != nil {
		panic(err)
	}
	group := map[string]string{}
	b, _ := os.ReadFile(os.Args[2])
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) == 2 {
			group[f[0]] = f[1]
		}
	}
	grp := func(fn string) string {
		if g := group[fn]; g != "" {
			return g
		}
		if strings.HasSuffix(fn, "_test.go") {
			if g := group[strings.TrimSuffix(fn, "_test.go")+".go"]; g != "" {
				return g
			}
		}
		return "?" + fn
	}
	type key struct{ from, to, obj string }
	seen := map[key]map[string]bool{}
	done := map[string]bool{}
	for _, p := range pkgs {
		for _, f := range p.Syntax {
			full := p.Fset.File(f.Pos()).Name()
			if done[full+p.ID] {
				continue
			}
			done[full+p.ID] = true
			fn := filepath.Base(full)
			fg := grp(fn)
			ast.Inspect(f, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				o := p.TypesInfo.Uses[id]
				if o == nil || o.Pkg() == nil || o.Pkg() != p.Types {
					return true
				}
				name := o.Name()
				switch v := o.(type) {
				case *types.Func:
					if r := v.Type().(*types.Signature).Recv(); r != nil {
						t := r.Type()
						if pt, ok := t.(*types.Pointer); ok {
							t = pt.Elem()
						}
						if nt, ok := t.(*types.Named); ok {
							name = nt.Obj().Name() + "." + name
						}
					} else if o.Parent() != p.Types.Scope() {
						return true
					}
				case *types.Var:
					if v.IsField() || o.Parent() != p.Types.Scope() {
						return true
					}
				default:
					if o.Parent() != p.Types.Scope() {
						return true
					}
				}
				if !o.Pos().IsValid() {
					return true
				}
				df := filepath.Base(p.Fset.File(o.Pos()).Name())
				if os.Getenv("UNEXP") == "1" {
					// test files' uses of unexported non-test objects (methods and fields included)
					if strings.HasSuffix(fn, "_test.go") && !strings.HasSuffix(df, "_test.go") && !ast.IsExported(o.Name()) {
						k := key{"", grp(df), name + "@" + df}
						if seen[k] == nil {
							seen[k] = map[string]bool{}
						}
						seen[k][fn] = true
					}
					return true
				}
				dg := grp(df)
				if dg == fg {
					return true
				}
				k := key{fg, dg, name + "@" + df}
				if seen[k] == nil {
					seen[k] = map[string]bool{}
				}
				seen[k][fn] = true
				return true
			})
		}
	}
	if os.Getenv("CLASSIFY") == "1" {
		// per test file: references to non-test objects, by group
		cnt := map[string]map[string]int{}
		for k, fs := range seen {
			if strings.HasSuffix(strings.SplitN(k.obj, "@", 2)[1], "_test.go") {
				continue
			}
			for f := range fs {
				if !strings.HasSuffix(f, "_test.go") {
					continue
				}
				if cnt[f] == nil {
					cnt[f] = map[string]int{}
				}
				cnt[f][k.to]++
			}
		}
		var fs []string
		for f := range cnt {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		for _, f := range fs {
			var parts []string
			for g, n := range cnt[f] {
				parts = append(parts, fmt.Sprintf("%s:%d", g, n))
			}
			sort.Strings(parts)
			fmt.Printf("%s %s %s\n", f, grp(f), strings.Join(parts, " "))
		}
		return
	}
	var ks []key
	for k := range seen {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool {
		if ks[i].to != ks[j].to {
			return ks[i].to < ks[j].to
		}
		if ks[i].obj != ks[j].obj {
			return ks[i].obj < ks[j].obj
		}
		return ks[i].from < ks[j].from
	})
	for _, k := range ks {
		var fs []string
		for f := range seen[k] {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		fmt.Printf("%s -> %s  %s  [%s]\n", k.from, k.to, k.obj, strings.Join(fs, ","))
	}
}
