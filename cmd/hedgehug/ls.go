package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/query"
)

func (a *app) lsCmd() *cobra.Command {
	var tree, idsOnly bool
	var format string
	cmd := &cobra.Command{
		Use:   "ls [query]",
		Short: "List tasks",
		RunE: func(_ *cobra.Command, args []string) error {
			n := 0
			for _, on := range []bool{a.jsonOut, idsOnly, format != ""} {
				if on {
					n++
				}
			}
			if n > 1 {
				return usageErrorf("give only one of --json, --ids-only and --format")
			}
			var tmpl *template.Template
			if format != "" {
				var err error
				if tmpl, err = template.New("format").Option("missingkey=error").Parse(format); err != nil {
					return usageErrorf("--format: %w", err)
				}
			}
			q, err := query.Parse(strings.Join(args, " "))
			if err != nil {
				return usageError{err}
			}
			if tree {
				return a.lsTree(q, idsOnly, tmpl)
			}
			return a.ls(q, idsOnly, tmpl)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&tree, "tree", false, "show the tasks found under their ancestors")
	f.BoolVar(&idsOnly, "ids-only", false, "print only the short IDs")
	f.StringVar(&format, "format", "", "print each task with a Go template over its JSON fields")
	return cmd
}

func (a *app) ls(q *query.Query, idsOnly bool, tmpl *template.Template) error {
	_, ix, err := a.open()
	if err != nil {
		return err
	}
	res, err := query.Run(ix, q, query.Options{})
	if err != nil {
		var qe *query.Error
		if errors.As(err, &qe) {
			return usageError{err}
		}
		return err
	}
	var all []*index.Entry
	for _, g := range res.Groups {
		all = append(all, g.Tasks...)
	}
	switch {
	case idsOnly:
		return a.printIDs(ix, all)
	case tmpl != nil:
		return a.printFormat(ix, all, tmpl, nil)
	case a.jsonOut || !hasTerm(q, "group"):
		return a.printTasks(ix, all)
	}
	for i, g := range res.Groups {
		key := g.Key
		if key == "" {
			key = "(none)"
		}
		if i > 0 {
			if _, err := fmt.Fprintln(a.stdout); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(a.stdout, "== %s (%d)\n", key, len(g.Tasks)); err != nil {
			return err
		}
		if err := a.printTasks(ix, g.Tasks); err != nil {
			return err
		}
	}
	return nil
}

func hasTerm(q *query.Query, key string) bool {
	for _, t := range q.Terms {
		if t.Key == key {
			return true
		}
	}
	return false
}

func (a *app) printIDs(ix *index.Index, es []*index.Entry) error {
	var b strings.Builder
	for _, e := range es {
		b.WriteString(ix.ShortID(e.ID) + "\n")
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}

// printFormat runs tmpl on each task, with the task as --json has it: a map
// of the JSON field names. extra, when not nil, adds keys to each map. A
// template that fails, as on a key that is not there, is a usage error; the
// output is held until every task is done, so none of it is printed then.
func (a *app) printFormat(ix *index.Index, es []*index.Entry, tmpl *template.Template, extra func(m map[string]any, e *index.Entry)) error {
	var out bytes.Buffer
	for _, e := range es {
		b, err := json.Marshal(entryJSON(ix, e))
		if err != nil {
			return err
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
		// --json leaves broken out of a task that is not broken; a
		// template needs the key to test it.
		if _, ok := m["broken"]; !ok {
			m["broken"] = ""
		}
		if extra != nil {
			extra(m, e)
		}
		if err := tmpl.Execute(&out, m); err != nil {
			return usageErrorf("--format: %w", err)
		}
		out.WriteByte('\n')
	}
	_, err := a.stdout.Write(out.Bytes())
	return err
}
