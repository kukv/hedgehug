package main

// usage: leaks <dir> <forbidden path substring,...> <pkgpattern>...
// prints exported API (funcs, methods, struct fields, type definitions)
// whose types mention a forbidden package: "pkg<TAB>Name<TAB>forbidden types".
import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

var bad []string

func mentions(t types.Type, seen map[types.Type]bool, out map[string]bool) {
	if t == nil || seen[t] {
		return
	}
	seen[t] = true
	switch v := t.(type) {
	case *types.Named:
		if p := v.Obj().Pkg(); p != nil {
			for _, b := range bad {
				if strings.Contains(p.Path(), b) {
					out[filepath.Base(p.Path())+"."+v.Obj().Name()] = true
					return
				}
			}
		}
		if ta := v.TypeArgs(); ta != nil {
			for i := 0; i < ta.Len(); i++ {
				mentions(ta.At(i), seen, out)
			}
		}
	case *types.Alias:
		mentions(types.Unalias(v), seen, out)
	case *types.Pointer:
		mentions(v.Elem(), seen, out)
	case *types.Slice:
		mentions(v.Elem(), seen, out)
	case *types.Array:
		mentions(v.Elem(), seen, out)
	case *types.Map:
		mentions(v.Key(), seen, out)
		mentions(v.Elem(), seen, out)
	case *types.Chan:
		mentions(v.Elem(), seen, out)
	case *types.Signature:
		for i := 0; i < v.Params().Len(); i++ {
			mentions(v.Params().At(i).Type(), seen, out)
		}
		for i := 0; i < v.Results().Len(); i++ {
			mentions(v.Results().At(i).Type(), seen, out)
		}
	case *types.Struct:
		for i := 0; i < v.NumFields(); i++ {
			if v.Field(i).Exported() || v.Field(i).Embedded() {
				mentions(v.Field(i).Type(), seen, out)
			}
		}
	case *types.Interface:
		for i := 0; i < v.NumMethods(); i++ {
			mentions(v.Method(i).Type(), seen, out)
		}
	}
}

func list(m map[string]bool) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, " ")
}

func main() {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes, Dir: os.Args[1]}
	pkgs, err := packages.Load(cfg, os.Args[3:]...)
	if err != nil {
		panic(err)
	}
	bad = strings.Split(os.Args[2], ",")
	for _, p := range pkgs {
		sc := p.Types.Scope()
		pn := strings.TrimPrefix(p.PkgPath, "github.com/kukv/hedgehug/internal/app/")
		for _, n := range sc.Names() {
			o := sc.Lookup(n)
			if !o.Exported() {
				continue
			}
			report := func(name string, t types.Type) {
				out := map[string]bool{}
				mentions(t, map[types.Type]bool{}, out)
				if len(out) > 0 {
					fmt.Printf("%s\t%s\t%s\n", pn, name, list(out))
				}
			}
			switch v := o.(type) {
			case *types.TypeName:
				if a, ok := v.Type().(*types.Alias); ok {
					report(n+" (alias)", a)
					continue
				}
				nt, ok := v.Type().(*types.Named)
				if !ok {
					continue
				}
				report(n, nt.Underlying())
				for i := 0; i < nt.NumMethods(); i++ {
					m := nt.Method(i)
					if m.Exported() {
						report(n+"."+m.Name(), m.Type())
					}
				}
			default:
				report(n, o.Type())
			}
		}
	}
}
