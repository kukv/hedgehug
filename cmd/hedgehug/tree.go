package main

import (
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/query"
)

func (a *app) lsTree(q *query.Query, ulidsOnly bool, tmpl *template.Template) error {
	res, err := a.ss.Tree(q)
	if err != nil {
		return err
	}
	ix, roots, cycle := res.Index, res.Roots, res.Cycle
	tops := append(slices.Clone(roots), cycle...)
	drawn := ops.Flatten(tops)
	switch {
	case ulidsOnly:
		// Only the tasks found: the ancestors drawn for them must not
		// reach a command such as done.
		var hit []*index.Entry
		for _, n := range drawn {
			if !n.Context {
				hit = append(hit, n.Entry)
			}
		}
		return a.printULIDs(hit)
	case tmpl != nil:
		node := map[model.ULID]*ops.TreeNode{}
		es := make([]*index.Entry, len(drawn))
		for i, n := range drawn {
			node[n.Entry.ULID], es[i] = n, n.Entry
		}
		return a.printFormat(ix, es, tmpl, func(m map[string]any, e *index.Entry) {
			m["context"], m["in_cycle"] = node[e.ULID].Context, node[e.ULID].InCycle
		})
	case a.jsonOut:
		out := make([]*ops.TaskView, len(tops))
		for i, n := range tops {
			out[i] = ops.TreeViewOf(ix, n)
		}
		return a.printJSON(out)
	}
	var b strings.Builder
	st := a.style()
	for _, n := range roots {
		writeNode(&b, st, ix, n, "", "")
	}
	if len(cycle) > 0 {
		b.WriteString("\n== in a parent cycle\n")
		for _, n := range cycle {
			writeNode(&b, st, ix, n, "", "")
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

func writeNode(b *strings.Builder, st style, ix *index.Index, n *ops.TreeNode, lead, childLead string) {
	b.WriteString(lead + treeLine(st, ix, n.Entry, n.Context) + "\n")
	for i, c := range n.Children {
		if i == len(n.Children)-1 {
			writeNode(b, st, ix, c, childLead+"└─ ", childLead+"   ")
		} else {
			writeNode(b, st, ix, c, childLead+"├─ ", childLead+"│  ")
		}
	}
}

// treeLine is one task of the tree: id and ULID, status, kind and title,
// then the checklist and the progress of a project.
func treeLine(st style, ix *index.Index, e *index.Entry, context bool) string {
	r := taskRow(e)
	name := r[0] + " " + r[1]
	line := name + "  " + r[2] + "  " + r[3] + "  " + r[6]
	if p := ops.ProgressText(ops.ChecklistOf(e), ops.ProgressOf(ix, e)); p != "" {
		line += "  " + p
	}
	if context {
		return st.dim(line)
	}
	return st.id(name) + line[len(name):]
}
