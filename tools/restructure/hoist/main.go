// hoist moves Go files from one package to another and keeps every
// reference in the module compiling. Usage, at the module root:
//
//	hoist <spec file>
//
// The spec file has one directive per line ('#' starts a comment):
//
//	from <dir>                 the package the files leave
//	to <dir> <name>            the package the "file" lines after it go to; it
//	                           may exist already; there may be several
//	file <name.go>             a file of <from> that moves (prod or test)
//	external <name_test.go>    a test file of <from> that stays in <from>'s
//	                           directory but becomes package <from>_test
//	rename <Obj> <New>         rename a package-level object of <from>, or a
//	                           member <Type>.<member>; "to:" before <Obj>
//	                           names one of <to>
//	tests external             moving test files become package <to>_test
//	alias <Obj> <Alias>        a <to>_test file that uses the unexported <Obj>
//	                           (or <Type>.<method>) of <to> uses <Alias>, which
//	                           hoist declares in export_test.go
//	dup <Name>                 a moving file declares <Name> that <to> has
//	                           already, on purpose (a step after deletes it)
//	bind <helper> [<other>]    a moving file that calls the test helper <helper>
//	                           of <from> calls <other> (default the same name)
//	                           of <to> instead
//	recv <Type>                methods on <Type> of <from> that are declared
//	                           in a moving file become functions taking the
//	                           receiver as their first parameter
//
// With no "to", nothing moves: only "external" and "rename" apply.
//
// Every reference is qualified by where its object ends up: an object in the
// same package as the referencing file is named bare, any other by its
// package name. Problems are printed and nothing is written: an unexported
// object referenced across packages, a test-only object referenced from
// another package, a method value of a converted method, edits that
// overlap.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

type pkgID struct{ path, name string } // name ends in _test for an external test package

type edit struct {
	start, end int
	text       string
}

var (
	fset     = token.NewFileSet()
	modPath  string
	modRoot  string
	fromDir  string
	toDir    string
	toName   string
	moving   = map[string]bool{}      // base names in fromDir
	moveAs   = map[string]string{}    // base name in fromDir -> base name in toDir
	destOf   = map[string][2]string{} // base name in fromDir -> {dir, package name} it goes to
	destPath = map[string]bool{}      // import paths of the destinations
	external = map[string]bool{}
	renames  = map[string]string{} // "pkgpath.Obj" or "pkgpath.Type.member" -> new
	recvs    = map[string]bool{}   // "pkgpath.Type"
	binds    = map[string]string{} // test helper of <from> -> the one of <to> to use instead
	dups     = map[string]bool{}   // names a moving file declares again on purpose
	aliases  = map[string]string{} // "pkgpath.Obj" -> the name export_test.go gives it
	// export_test.go to write: dir -> alias -> declaration
	aliasOut      = map[string]map[string]string{}
	aliasPkg      = map[string]string{}
	aliasImports  = map[string]map[string]string{}
	testsExternal bool
	afterFile     func(types.Object) string
	aliasFor      func(types.Object) (string, bool)
	needAlias     func(pkgID, types.Object, string)
	errs          []string
	edits         = map[string][]edit{}            // abs file -> edits
	imports       = map[string]map[string]string{} // abs file -> path -> name
	newPkgOf      = map[string]pkgID{}             // abs file -> package after
)

func fail(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

func main() {
	wd, _ := os.Getwd()
	modRoot = wd
	modPath = modulePath()
	readSpec(os.Args[1])
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Tests: true,
		Fset:  fset,
		ParseFile: func(fs *token.FileSet, name string, src []byte) (*ast.File, error) {
			return parser.ParseFile(fs, name, src, parser.ParseComments)
		},
	}
	pkgs, err := packages.Load(cfg, "./...")
	must(err)
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}
	fromPath := modPath + "/" + fromDir
	absFrom := filepath.Join(modRoot, fromDir)

	type unit struct {
		p *packages.Package
		f *ast.File
	}
	var units []unit
	done := map[string]bool{}
	for _, p := range pkgs {
		for _, f := range p.Syntax {
			fn := fset.File(f.Pos()).Name()
			if done[fn] || !strings.HasPrefix(fn, modRoot+string(filepath.Separator)) {
				continue // a test main the go command made
			}
			done[fn] = true
			units = append(units, unit{p, f})
			id := pkgID{strings.TrimSuffix(p.PkgPath, "_test"), f.Name.Name}
			if filepath.Dir(fn) == absFrom {
				base := filepath.Base(fn)
				switch {
				case moving[base]:
					d := destOf[base]
					id = pkgID{modPath + "/" + d[0], d[1]}
					if strings.HasSuffix(f.Name.Name, "_test") || (testsExternal && strings.HasSuffix(moveAs[base], "_test.go") && !strings.HasSuffix(moveAs[base], "export_test.go")) {
						id.name = d[1] + "_test"
					}
				case external[base]:
					if !strings.HasSuffix(f.Name.Name, "_test") {
						id.name = f.Name.Name + "_test"
					}
				}
			}
			newPkgOf[fn] = id
		}
	}
	for base := range moving {
		if !done[filepath.Join(absFrom, base)] {
			fail("file %s not found in %s", base, fromDir)
		}
	}
	for base := range external {
		if !done[filepath.Join(absFrom, base)] {
			fail("file %s not found in %s", base, fromDir)
		}
	}

	declFile := func(o types.Object) string { return fset.Position(o.Pos()).Filename }
	objPkg := func(o types.Object) pkgID {
		if id, ok := newPkgOf[declFile(o)]; ok {
			return id
		}
		return pkgID{o.Pkg().Path(), o.Pkg().Name()}
	}
	pkgLevel := func(o types.Object) bool { return o.Parent() != nil && o.Parent() == o.Pkg().Scope() }
	member := func(o types.Object) bool {
		switch v := o.(type) {
		case *types.Func:
			return v.Type().(*types.Signature).Recv() != nil
		case *types.Var:
			return v.IsField()
		}
		return false
	}
	newName := func(o types.Object) string {
		if !pkgLevel(o) && !member(o) {
			return o.Name()
		}
		if n, ok := renames[strings.TrimSuffix(o.Pkg().Path(), "_test")+"."+memberKey(o)]; ok {
			return n
		}
		return o.Name()
	}
	converted := func(o types.Object) bool {
		fn, ok := o.(*types.Func)
		if !ok {
			return false
		}
		r := fn.Type().(*types.Signature).Recv()
		if r == nil {
			return false
		}
		nt := namedOf(r.Type())
		if nt == nil || !recvs[nt.Obj().Pkg().Path()+"."+nt.Obj().Name()] {
			return false
		}
		f := declFile(o)
		return filepath.Dir(f) == absFrom && moving[filepath.Base(f)]
	}
	relevant := func(o types.Object) bool {
		if o == nil || o.Pkg() == nil {
			return false
		}
		pp := strings.TrimSuffix(o.Pkg().Path(), "_test")
		return pp == fromPath || destPath[pp]
	}

	// the file an object is declared in, after the move
	afterFile = func(o types.Object) string {
		f := declFile(o)
		if filepath.Dir(f) == absFrom && moving[filepath.Base(f)] {
			return moveAs[filepath.Base(f)]
		}
		return f
	}
	aliasFor = func(o types.Object) (string, bool) {
		if !pkgLevel(o) && !member(o) {
			return "", false
		}
		a, ok := aliases[strings.TrimSuffix(o.Pkg().Path(), "_test")+"."+memberKey(o)]
		return a, ok
	}
	needAlias = func(op pkgID, o types.Object, a string) {
		dir := filepath.Join(modRoot, strings.TrimPrefix(op.path, modPath+"/"))
		if aliasOut[dir] == nil {
			aliasOut[dir] = map[string]string{}
			aliasPkg[dir] = op.name
			aliasImports[dir] = map[string]string{}
		}
		if _, done := aliasOut[dir][a]; done {
			return
		}
		qf := func(p *types.Package) string {
			if p.Path() == op.path || p.Path() == o.Pkg().Path() {
				return ""
			}
			aliasImports[dir][p.Path()] = p.Name()
			return p.Name()
		}
		name := newName(o)
		var decl string
		switch v := o.(type) {
		case *types.TypeName:
			decl = fmt.Sprintf("type %s = %s", a, name)
		case *types.Const:
			decl = fmt.Sprintf("const %s = %s", a, name)
		case *types.Var:
			if v.IsField() {
				fail("alias of the field %s: write an accessor by hand", memberKey(o))
				return
			}
			decl = fmt.Sprintf("var %s = %s", a, name)
		case *types.Func:
			sig := v.Type().(*types.Signature)
			if sig.Recv() == nil || converted(o) {
				if sig.TypeParams().Len() > 0 {
					fail("alias of the generic %s: write it by hand", name)
					return
				}
				decl = fmt.Sprintf("var %s = %s", a, name)
				break
			}
			// a method: a method of the same receiver that calls it
			var params, args []string
			for i := range sig.Params().Len() {
				p := sig.Params().At(i)
				pn := fmt.Sprintf("a%d", i)
				t := types.TypeString(p.Type(), qf)
				if sig.Variadic() && i == sig.Params().Len()-1 {
					t = "..." + types.TypeString(p.Type().(*types.Slice).Elem(), qf)
					pn += "..."
					params = append(params, fmt.Sprintf("a%d %s", i, t))
				} else {
					params = append(params, pn+" "+t)
				}
				args = append(args, pn)
			}
			var res []string
			for i := range sig.Results().Len() {
				res = append(res, types.TypeString(sig.Results().At(i).Type(), qf))
			}
			rs := strings.Join(res, ", ")
			if len(res) > 1 {
				rs = "(" + rs + ")"
			}
			ret := "return "
			if len(res) == 0 {
				ret = ""
			}
			rn := sig.Recv().Name()
			if rn == "" || rn == "_" {
				rn = "r"
			}
			decl = fmt.Sprintf("func (%s %s) %s(%s) %s {\n\t%s%s.%s(%s)\n}", rn, types.TypeString(sig.Recv().Type(), qf), a, strings.Join(params, ", "), rs, ret, rn, name, strings.Join(args, ", "))
		}
		aliasOut[dir][a] = decl
	}

	// names that would be declared twice in the destination
	if len(destPath) > 0 {
		have := map[string]string{} // path.name -> file, of what is in a destination already
		for _, p := range pkgs {
			if !destPath[strings.TrimSuffix(p.PkgPath, "_test")] || strings.HasSuffix(p.Name, "_test") {
				continue
			}
			for _, n := range p.Types.Scope().Names() {
				o := p.Types.Scope().Lookup(n)
				have[p.PkgPath+"."+newName(o)] = pos(o.Pos())
			}
		}
		for _, p := range pkgs {
			if strings.TrimSuffix(p.PkgPath, "_test") != fromPath || strings.HasSuffix(p.Name, "_test") {
				continue
			}
			for _, n := range p.Types.Scope().Names() {
				o := p.Types.Scope().Lookup(n)
				f := declFile(o)
				if filepath.Dir(f) != absFrom || !moving[filepath.Base(f)] {
					continue
				}
				d := destOf[filepath.Base(f)]
				if dups[newName(o)] {
					continue
				}
				if other, ok := have[modPath+"/"+d[0]+"."+newName(o)]; ok && other != pos(o.Pos()) {
					fail("%s (%s) is declared in %s of %s already: rename it", newName(o), filepath.Base(f), other, d[1])
				}
				have[modPath+"/"+d[0]+"."+newName(o)] = pos(o.Pos())
			}
		}
		// methods that become functions, against the rest
		for _, p := range pkgs {
			if strings.TrimSuffix(p.PkgPath, "_test") != fromPath || strings.HasSuffix(p.Name, "_test") {
				continue
			}
			for id, o := range p.TypesInfo.Defs {
				if o == nil || !converted(o) || id.Name != o.Name() {
					continue
				}
				d := destOf[filepath.Base(declFile(o))]
				k := modPath + "/" + d[0] + "." + newName(o)
				if other, ok := have[k]; ok && other != pos(o.Pos()) {
					fail("method %s becomes the function %s of %s, which %s declares already: rename it", memberKey(o), newName(o), d[1], other)
					continue
				}
				have[k] = pos(o.Pos())
			}
		}
	}

	for _, u := range units {
		p, f := u.p, u.f
		fn := fset.File(f.Pos()).Name()
		mine := newPkgOf[fn]
		if mine.name != f.Name.Name {
			addEdit(fn, f.Name.Pos(), f.Name.End(), mine.name)
		}
		nm := func(o types.Object) string {
			if a, ok := aliasFor(o); ok {
				if op := objPkg(o); mine.path == op.path && mine.name == op.name+"_test" {
					return a
				}
			}
			return newName(o)
		}
		qual := func(o types.Object, at token.Pos) string {
			op := objPkg(o)
			if op == mine {
				return ""
			}
			if a, ok := aliasFor(o); ok && mine.path == op.path && mine.name == op.name+"_test" {
				needAlias(op, o, a)
				addImport(fn, op.path, op.name)
				return op.name + "."
			}
			if strings.HasSuffix(afterFile(o), "_test.go") && op.path != mine.path {
				fail("%s: %s is declared in a test file of %s, which %s cannot see", pos(at), o.Name(), op.name, mine.name)
			}
			if !ast.IsExported(newName(o)) {
				fail("%s: unexported %s (%s) from %s", pos(at), o.Name(), op.name, mine.name)
			}
			addImport(fn, op.path, op.name)
			return op.name + "."
		}
		// the parts of the file handled by a larger node
		pkgSel := map[*ast.Ident]*ast.SelectorExpr{} // Sel of pkg.Name
		selIdent := map[*ast.Ident]bool{}            // Sel of a field or method selection
		calls := map[*ast.SelectorExpr]*ast.CallExpr{}
		var skip [][2]token.Pos                // receivers removed from converted methods
		docOf := map[*ast.Ident]*ast.Comment{} // the first line of a declaration's doc
		ast.Inspect(f, func(n ast.Node) bool {
			first := func(g *ast.CommentGroup) *ast.Comment {
				if g == nil || len(g.List) == 0 {
					return nil
				}
				return g.List[0]
			}
			switch n := n.(type) {
			case *ast.GenDecl:
				for _, s := range n.Specs {
					doc := first(n.Doc)
					switch s := s.(type) {
					case *ast.TypeSpec:
						if c := first(s.Doc); c != nil || len(n.Specs) > 1 {
							doc = c
						}
						docOf[s.Name] = doc
					case *ast.ValueSpec:
						if c := first(s.Doc); c != nil || len(n.Specs) > 1 {
							doc = c
						}
						for _, nm := range s.Names {
							docOf[nm] = doc
						}
					}
				}
			case *ast.Field:
				for _, nm := range n.Names {
					docOf[nm] = first(n.Doc)
				}
			case *ast.FuncDecl:
				docOf[n.Name] = first(n.Doc)
			}
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if x, ok := n.X.(*ast.Ident); ok {
					if _, ok := p.TypesInfo.Uses[x].(*types.PkgName); ok {
						pkgSel[n.Sel] = n
					}
				}
				if p.TypesInfo.Selections[n] != nil {
					selIdent[n.Sel] = true
				}
			case *ast.CallExpr:
				if s, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr); ok {
					calls[s] = n
				}
			case *ast.FuncDecl:
				if n.Recv != nil && len(n.Recv.List) > 0 {
					if o := p.TypesInfo.Defs[n.Name]; o != nil && converted(o) {
						skip = append(skip, [2]token.Pos{n.Recv.Opening, n.Name.Pos()})
					}
				}
			}
			return true
		})
		inSkip := func(at token.Pos) bool {
			for _, r := range skip {
				if at >= r[0] && at < r[1] {
					return true
				}
			}
			return false
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Recv == nil || len(n.Recv.List) == 0 {
					return true
				}
				o := p.TypesInfo.Defs[n.Name]
				if o == nil || !converted(o) {
					return true
				}
				// func (ss *Session) M(a A) -> func M(ss *usecase.Session, a A)
				rf := n.Recv.List[0]
				rname := "ss"
				if len(rf.Names) > 0 && rf.Names[0].Name != "_" {
					rname = rf.Names[0].Name
				}
				nt := namedOf(p.TypesInfo.TypeOf(rf.Type))
				star := ""
				if _, ok := rf.Type.(*ast.StarExpr); ok {
					star = "*"
				}
				rtype := star + qual(nt.Obj(), rf.Pos()) + nm(nt.Obj())
				addEdit(fn, n.Recv.Opening, n.Name.Pos(), "")
				sep := ""
				if n.Type.Params.NumFields() > 0 {
					sep = ", "
				}
				addEdit(fn, n.Type.Params.Opening+1, n.Type.Params.Opening+1, rname+" "+rtype+sep)
				if nn := newName(o); nn != o.Name() {
					addEdit(fn, n.Name.Pos(), n.Name.End(), nn)
				}
				return true
			case *ast.SelectorExpr:
				sel := p.TypesInfo.Selections[n]
				if sel == nil {
					return true // a qualified identifier: the Ident case
				}
				o := sel.Obj()
				if !relevant(o) {
					return true
				}
				if converted(o) {
					call, ok := calls[n]
					if !ok || sel.Kind() != types.MethodVal {
						fail("%s: method value %s of a converted method", pos(n.Pos()), o.Name())
						return true
					}
					// X.M(args) -> q.M(X[.embedded], args)
					path := ""
					t := sel.Recv()
					idx := sel.Index()
					for _, i := range idx[:len(idx)-1] {
						fld := structOf(t).Field(i)
						path += "." + fld.Name()
						t = fld.Type()
					}
					amp := ""
					_, isPtr := t.Underlying().(*types.Pointer)
					_, want := o.Type().(*types.Signature).Recv().Type().(*types.Pointer)
					switch {
					case !isPtr && want:
						amp = "&"
					case isPtr && !want:
						amp = "*"
					}
					sep := ""
					if len(call.Args) > 0 {
						sep = ", "
					}
					addEdit(fn, n.X.Pos(), n.X.Pos(), qual(o, n.Pos())+nm(o)+"("+amp)
					addEdit(fn, n.X.End(), call.Lparen+1, path+sep)
					return true
				}
				if op := objPkg(o); op != mine && !ast.IsExported(newName(o)) {
					if a, ok := aliasFor(o); ok && mine.path == op.path && mine.name == op.name+"_test" {
						needAlias(op, o, a)
						addEdit(fn, n.Sel.Pos(), n.Sel.End(), a)
						return true
					}
					fail("%s: unexported member %s (%s) from %s", pos(n.Sel.Pos()), o.Name(), op.name, mine.name)
				}
				if nn := newName(o); nn != o.Name() {
					addEdit(fn, n.Sel.Pos(), n.Sel.End(), nn)
				}
				return true
			case *ast.Ident:
				if selIdent[n] || inSkip(n.Pos()) {
					return true
				}
				o := p.TypesInfo.Uses[n]
				def := false
				if o == nil {
					o, def = p.TypesInfo.Defs[n], true
				}
				if o == nil || !relevant(o) {
					return true
				}
				if _, ok := o.(*types.PkgName); ok {
					return true
				}
				if def || !pkgLevel(o) {
					// a declaration, or a field key in a composite literal
					if nn := newName(o); nn != o.Name() {
						addEdit(fn, n.Pos(), n.End(), nn)
						if c := docOf[n]; def && c != nil && strings.HasPrefix(c.Text, "// "+o.Name()+" ") {
							addEdit(fn, c.Slash+3, c.Slash+3+token.Pos(len(o.Name())), nn)
						}
					}
					return true
				}
				if b, ok := binds[o.Name()]; ok && destPath[mine.path] && objPkg(o) != mine && strings.HasSuffix(declFile(o), "_test.go") {
					// a test helper left behind: the destination has its own
					if _, isSel := pkgSel[n]; !isSel {
						if b != o.Name() {
							addEdit(fn, n.Pos(), n.End(), b)
						}
						return true
					}
				}
				if se, ok := pkgSel[n]; ok {
					if q := qual(o, n.Pos()); q != se.X.(*ast.Ident).Name+"." || nm(o) != o.Name() {
						addEdit(fn, se.Pos(), se.End(), q+nm(o))
					}
					return true
				}
				if q := qual(o, n.Pos()); q != "" || nm(o) != o.Name() {
					addEdit(fn, n.Pos(), n.End(), q+nm(o))
				}
				return true
			}
			return true
		})
	}
	checkOverlaps()
	if len(errs) > 0 {
		sort.Strings(errs)
		for i, e := range errs {
			if i == 0 || e != errs[i-1] {
				fmt.Fprintln(os.Stderr, e)
			}
		}
		os.Exit(1)
	}
	for base, as := range moveAs {
		if _, err := os.Stat(filepath.Join(modRoot, destOf[base][0], as)); err == nil {
			fmt.Fprintf(os.Stderr, "hoist: %s exists in %s (file %s <new name>)\n", as, destOf[base][0], base)
			os.Exit(1)
		}
	}
	n := 0
	for fn, es := range edits {
		writeFile(fn, es)
		n++
	}
	for base := range moving {
		fn := filepath.Join(absFrom, base)
		if _, ok := edits[fn]; !ok {
			writeFile(fn, nil)
			n++
		}
	}
	for dir, decls := range aliasOut {
		writeAliases(dir, decls)
		n++
	}
	fmt.Printf("hoist: wrote %d files\n", n)
}

// writeAliases adds decls to export_test.go of dir.
func writeAliases(dir string, decls map[string]string) {
	fn := filepath.Join(dir, "export_test.go")
	src, err := os.ReadFile(fn)
	if os.IsNotExist(err) {
		src = []byte("package " + aliasPkg[dir] + "\n")
	} else {
		must(err)
	}
	var names []string
	for a := range decls {
		names = append(names, a)
	}
	sort.Strings(names)
	var b strings.Builder
	b.Write(src)
	for _, a := range names {
		b.WriteString("\n" + decls[a] + "\n")
	}
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, fn, b.String(), parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n%s", fn, err, b.String())
		os.Exit(1)
	}
	for path, name := range aliasImports[dir] {
		if filepath.Base(path) == name {
			astutil.AddImport(fs, f, path)
		} else {
			astutil.AddNamedImport(fs, f, name, path)
		}
	}
	var buf bytes.Buffer
	must(format.Node(&buf, fs, f))
	must(os.MkdirAll(dir, 0o755))
	must(os.WriteFile(fn, buf.Bytes(), 0o644))
}

func writeFile(fn string, es []edit) {
	src, err := os.ReadFile(fn)
	must(err)
	// from the end, so offsets stay good; at one offset, the replacement
	// before the insertion, so the insertion ends up in front of it
	sort.SliceStable(es, func(i, j int) bool {
		if es[i].start != es[j].start {
			return es[i].start > es[j].start
		}
		return es[i].end > es[j].end
	})
	for _, e := range es {
		src = append(src[:e.start:e.start], append([]byte(e.text), src[e.end:]...)...)
	}
	if im := imports[fn]; len(im) > 0 {
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, fn, src, parser.ParseComments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fn, err)
			os.Exit(1)
		}
		for path, name := range im {
			if filepath.Base(path) == name {
				astutil.AddImport(fs, f, path)
			} else {
				astutil.AddNamedImport(fs, f, name, path)
			}
		}
		var buf bytes.Buffer
		must(format.Node(&buf, fs, f))
		src = buf.Bytes()
	}
	dst := fn
	if filepath.Dir(fn) == filepath.Join(modRoot, fromDir) && moving[filepath.Base(fn)] {
		dst = filepath.Join(modRoot, destOf[filepath.Base(fn)][0], moveAs[filepath.Base(fn)])
		if _, err := os.Stat(dst); err == nil {
			fmt.Fprintf(os.Stderr, "hoist: %s exists\n", dst)
			os.Exit(1)
		}
		must(os.MkdirAll(filepath.Dir(dst), 0o755))
		must(os.Remove(fn))
	}
	must(os.WriteFile(dst, src, 0o644))
}

func addEdit(fn string, start, end token.Pos, text string) {
	s, e := fset.Position(start).Offset, fset.Position(end).Offset
	for _, x := range edits[fn] {
		if x.start == s && x.end == e {
			if x.text != text {
				fail("%s:%d: two edits of one place: %q and %q", fn, fset.Position(start).Line, x.text, text)
			}
			return
		}
	}
	edits[fn] = append(edits[fn], edit{s, e, text})
}

// checkOverlaps fails on a replacement that overlaps another, or an
// insertion strictly inside a replacement.
func checkOverlaps() {
	for fn, es := range edits {
		for i, a := range es {
			for _, b := range es[i+1:] {
				lo, hi := a, b
				if lo.start > hi.start || (lo.start == hi.start && lo.end < hi.end) {
					lo, hi = hi, lo
				}
				if hi.start < lo.end && !(hi.start == hi.end && hi.start == lo.start) {
					fail("%s: overlapping edits at offsets %d-%d %q and %d-%d %q", fn, lo.start, lo.end, lo.text, hi.start, hi.end, hi.text)
				}
			}
		}
	}
}

func addImport(fn, path, name string) {
	if imports[fn] == nil {
		imports[fn] = map[string]string{}
	}
	imports[fn][path] = name
}

func memberKey(o types.Object) string {
	switch v := o.(type) {
	case *types.Func:
		if r := v.Type().(*types.Signature).Recv(); r != nil {
			if nt := namedOf(r.Type()); nt != nil {
				return nt.Obj().Name() + "." + o.Name()
			}
		}
	case *types.Var:
		if v.IsField() {
			return fieldOwner(v) + "." + o.Name()
		}
	}
	return o.Name()
}

var fieldOwners = map[string]string{} // position of the field -> struct type name

func fieldOwner(v *types.Var) string {
	k := fset.Position(v.Pos()).String()
	if s, ok := fieldOwners[k]; ok {
		return s
	}
	sc := v.Pkg().Scope()
	for _, n := range sc.Names() {
		tn, ok := sc.Lookup(n).(*types.TypeName)
		if !ok {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := range st.NumFields() {
			fieldOwners[fset.Position(st.Field(i).Pos()).String()] = n
		}
	}
	return fieldOwners[k]
}

func namedOf(t types.Type) *types.Named {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, _ := types.Unalias(t).(*types.Named)
	return n
}

func structOf(t types.Type) *types.Struct {
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	return t.Underlying().(*types.Struct)
}

func pos(p token.Pos) string {
	ps := fset.Position(p)
	rel, err := filepath.Rel(modRoot, ps.Filename)
	if err != nil {
		rel = ps.Filename
	}
	return fmt.Sprintf("%s:%d", rel, ps.Line)
}

func readSpec(name string) {
	f, err := os.Open(name)
	must(err)
	defer f.Close()
	sc := bufio.NewScanner(f)
	var pending [][]string
	for sc.Scan() {
		l := sc.Text()
		if i := strings.Index(l, "#"); i >= 0 {
			l = l[:i]
		}
		fs := strings.Fields(l)
		if len(fs) == 0 {
			continue
		}
		switch fs[0] {
		case "from":
			fromDir = filepath.Clean(fs[1])
		case "to":
			toDir, toName = filepath.Clean(fs[1]), fs[2]
		case "file":
			moving[fs[1]] = true
			destOf[fs[1]] = [2]string{toDir, toName}
			destPath[modPath+"/"+toDir] = true
			moveAs[fs[1]] = fs[1]
			if len(fs) > 2 {
				moveAs[fs[1]] = fs[2]
			}
		case "external":
			external[fs[1]] = true
		case "dup":
			dups[fs[1]] = true
		case "bind":
			binds[fs[1]] = fs[len(fs)-1]
		case "tests":
			testsExternal = len(fs) > 1 && fs[1] == "external"
		case "rename", "recv", "alias":
			pending = append(pending, fs)
		default:
			fmt.Fprintf(os.Stderr, "unknown directive %q\n", fs[0])
			os.Exit(2)
		}
	}
	if toDir == "" {
		toDir = fromDir
		toName = "-"
		if len(moving) > 0 {
			fmt.Fprintln(os.Stderr, "file needs to")
			os.Exit(2)
		}
	}
	for _, fs := range pending {
		k := fs[1]
		dir := fromDir
		if strings.HasPrefix(k, "to:") {
			k, dir = k[3:], toDir
		}
		switch fs[0] {
		case "rename":
			renames[modPath+"/"+dir+"."+k] = fs[2]
		case "recv":
			recvs[modPath+"/"+dir+"."+k] = true
		case "alias":
			aliases[modPath+"/"+dir+"."+k] = fs[2]
		}
	}
}

func modulePath() string {
	b, err := os.ReadFile("go.mod")
	must(err)
	return strings.Fields(strings.SplitN(string(b), "\n", 2)[0])[1]
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
