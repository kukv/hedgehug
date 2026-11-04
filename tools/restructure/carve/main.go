// carve moves top-level declarations between files of one package, with
// their doc comments. Usage:
//
//	carve <spec file>
//
// The spec file has lines "<dir>/<from.go> <to.go> <Name>...": Name is a
// function, a type, a var or a const, or Type.Method for a method. <to.go>
// is in the same directory; it is made when missing. The imports of every
// source file are copied into the destination; goimports drops the unused
// ones afterwards.
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type cut struct{ start, end int }

func main() {
	if os.Args[1] == "-list" {
		// the declarations of a file, as carve names them, and struct fields
		af, err := parser.ParseFile(token.NewFileSet(), os.Args[2], nil, 0)
		must(err)
		for _, d := range af.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				name := d.Name.Name
				if d.Recv != nil && len(d.Recv.List) > 0 {
					t := d.Recv.List[0].Type
					if s, ok := t.(*ast.StarExpr); ok {
						t = s.X
					}
					name = t.(*ast.Ident).Name + "." + name
				}
				fmt.Println(name)
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						fmt.Println(s.Name.Name)
						if st, ok := s.Type.(*ast.StructType); ok {
							for _, f := range st.Fields.List {
								for _, n := range f.Names {
									fmt.Println(s.Name.Name + "." + n.Name)
								}
							}
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							fmt.Println(n.Name)
						}
					}
				}
			}
		}
		return
	}
	f, err := os.Open(os.Args[1])
	must(err)
	sc := bufio.NewScanner(f)
	type job struct {
		from, to string
		names    []string
	}
	var jobs []job
	for sc.Scan() {
		l := sc.Text()
		if i := strings.Index(l, "#"); i >= 0 {
			l = l[:i]
		}
		fs := strings.Fields(l)
		if len(fs) == 0 {
			continue
		}
		if len(fs) < 3 {
			fail("bad line %q", l)
		}
		jobs = append(jobs, job{fs[0], filepath.Join(filepath.Dir(fs[0]), fs[1]), fs[2:]})
	}
	for _, j := range jobs {
		carve(j.from, j.to, j.names)
	}
}

func carve(from, to string, names []string) {
	src, err := os.ReadFile(from)
	must(err)
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, from, src, parser.ParseComments)
	must(err)
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var cuts []cut
	var imports []string
	for _, im := range af.Imports {
		s := im.Path.Value
		if im.Name != nil {
			s = im.Name.Name + " " + s
		}
		imports = append(imports, s)
	}
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	add := func(name string, d ast.Decl, doc *ast.CommentGroup) {
		start := off(d.Pos())
		if doc != nil {
			start = off(doc.Pos())
		}
		end := off(d.End())
		// the rest of the line (a trailing comment) and the newline
		for end < len(src) && src[end] != '\n' {
			end++
		}
		if end < len(src) {
			end++
		}
		// a blank line after
		if end < len(src) && src[end] == '\n' {
			end++
		}
		cuts = append(cuts, cut{start, end})
		delete(want, name)
	}
	for _, d := range af.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				t := d.Recv.List[0].Type
				if s, ok := t.(*ast.StarExpr); ok {
					t = s.X
				}
				if ix, ok := t.(*ast.IndexExpr); ok {
					t = ix.X
				}
				name = t.(*ast.Ident).Name + "." + name
			}
			if want[name] {
				add(name, d, d.Doc)
			}
		case *ast.GenDecl:
			var in []string
			all := true
			for _, s := range d.Specs {
				var ns []string
				switch s := s.(type) {
				case *ast.TypeSpec:
					ns = []string{s.Name.Name}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						ns = append(ns, n.Name)
					}
				default:
					continue
				}
				for _, n := range ns {
					if want[n] {
						in = append(in, n)
					} else {
						all = false
					}
				}
			}
			if len(in) == 0 {
				continue
			}
			if !all {
				fail("%s: %v share a declaration with names not listed", from, in)
			}
			for _, n := range in[1:] {
				delete(want, n)
			}
			add(in[0], d, d.Doc)
		}
	}
	if len(want) > 0 {
		var left []string
		for n := range want {
			left = append(left, n)
		}
		sort.Strings(left)
		fail("%s: not found: %v", from, left)
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].start < cuts[j].start })
	var moved strings.Builder
	rest := []byte{}
	last := 0
	for _, c := range cuts {
		rest = append(rest, src[last:c.start]...)
		moved.Write(src[c.start:c.end])
		if !strings.HasSuffix(moved.String(), "\n\n") {
			moved.WriteString("\n")
		}
		last = c.end
	}
	rest = append(rest, src[last:]...)
	must(os.WriteFile(from, rest, 0o644))

	out, err := os.ReadFile(to)
	if os.IsNotExist(err) {
		out = []byte("package " + af.Name.Name + "\n\nimport (\n" + "\t" + strings.Join(imports, "\n\t") + "\n)\n")
	} else {
		must(err)
		// add the imports of from; goimports removes what is not used
		tf, err := parser.ParseFile(token.NewFileSet(), to, out, parser.ImportsOnly)
		must(err)
		have := map[string]bool{}
		for _, im := range tf.Imports {
			have[im.Path.Value] = true
		}
		var extra []string
		for _, s := range imports {
			p := s[strings.LastIndex(s, " ")+1:]
			if !have[p] {
				extra = append(extra, s)
			}
		}
		if len(extra) > 0 {
			block := "import (\n\t" + strings.Join(extra, "\n\t") + "\n)\n"
			pkgEnd := strings.Index(string(out), "\n") + 1
			// after the package clause and its doc
			i := strings.Index(string(out), "\npackage ")
			if i >= 0 {
				pkgEnd = i + 1 + strings.Index(string(out[i+1:]), "\n") + 1
			} else if strings.HasPrefix(string(out), "package ") {
				pkgEnd = strings.Index(string(out), "\n") + 1
			}
			out = []byte(string(out[:pkgEnd]) + "\n" + block + string(out[pkgEnd:]))
		}
	}
	out = append(out, '\n')
	out = append(out, []byte(moved.String())...)
	must(os.WriteFile(to, out, 0o644))
	fmt.Printf("carve: %s -> %s: %d declarations\n", filepath.Base(from), filepath.Base(to), len(cuts))
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "carve: "+format+"\n", a...)
	os.Exit(1)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
