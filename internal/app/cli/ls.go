package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
)

func (a *app) lsCmd() *cobra.Command {
	var tree, ulidsOnly bool
	var format string
	cmd := &cobra.Command{
		Use:   "ls [query]",
		Short: "List tasks",
		Long: "List the tasks that match the query.\n\n" +
			"A term that starts with - (a negation, such as -status:done) is read as a\n" +
			"flag; put it after --: hedgehug ls -- -status:done",
		RunE: func(_ *cobra.Command, args []string) error {
			n := 0
			for _, on := range []bool{a.jsonOut, ulidsOnly, format != ""} {
				if on {
					n++
				}
			}
			if n > 1 {
				return usageErrorf("give only one of --json, --ulids-only and --format")
			}
			var tmpl *template.Template
			if format != "" {
				var err error
				if tmpl, err = template.New("format").Option("missingkey=error").Parse(format); err != nil {
					return usageErrorf("--format: %w", err)
				}
			}
			query := strings.Join(args, " ")
			if tree {
				return a.lsTree(query, ulidsOnly, tmpl)
			}
			return a.ls(query, ulidsOnly, tmpl)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&tree, "tree", false, "show the tasks found under their ancestors")
	f.BoolVar(&ulidsOnly, "ulids-only", false, "print only the full ULIDs")
	f.StringVar(&format, "format", "", "print each task with a Go template over its JSON fields")
	return cmd
}

func (a *app) ls(query string, ulidsOnly bool, tmpl *template.Template) error {
	res, err := browse.List(a.ss, query)
	if err != nil {
		return err
	}
	switch {
	case ulidsOnly:
		return a.printULIDs(res.Tasks)
	case tmpl != nil:
		return a.printFormat(res.Tasks, tmpl, nil)
	case a.jsonOut:
		return a.printJSON(res.JSON())
	case len(res.Groups) == 0:
		return a.printTasks(res.Tasks, res.Many)
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
		if err := a.printTasks(g.Tasks, res.Many); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) printULIDs(vs []*usecase.TaskView) error {
	var b strings.Builder
	for _, v := range vs {
		b.WriteString(v.ULID + "\n")
	}
	_, err := fmt.Fprint(a.stdout, b.String())
	return err
}

// printFormat runs tmpl on each task, with the task as --json has it: a map
// of the JSON field names. extra, when not nil, adds keys to each map. A
// template that fails, as on a key that is not there, is a usage error; the
// output is held until every task is done, so none of it is printed then.
func (a *app) printFormat(vs []*usecase.TaskView, tmpl *template.Template, extra func(m map[string]any, v *usecase.TaskView)) error {
	var out bytes.Buffer
	for _, v := range vs {
		own := *v
		own.Children = nil // a tree is flat here: every task has its own line
		b, err := json.Marshal(&own)
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
			extra(m, v)
		}
		if err := tmpl.Execute(&out, m); err != nil {
			return usageErrorf("--format: %w", err)
		}
		out.WriteByte('\n')
	}
	_, err := a.stdout.Write(out.Bytes())
	return err
}
