package main

import (
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/mount"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/query"
)

func (a *app) lsTree(q *query.Query, ulidsOnly bool, tmpl *template.Template) error {
	res, err := a.ss.Tree(q)
	if err != nil {
		return err
	}
	// every store's trees, roots then the cycle, in the order they are drawn
	type drawnNode struct {
		store ops.TreeStore
		node  *ops.TreeNode
	}
	var tops, drawn []drawnNode
	for _, ts := range res.Stores {
		own := append(slices.Clone(ts.Roots), ts.Cycle...)
		for _, n := range own {
			tops = append(tops, drawnNode{ts, n})
		}
		for _, n := range ops.Flatten(own) {
			drawn = append(drawn, drawnNode{ts, n})
		}
	}
	switch {
	case ulidsOnly:
		// Only the tasks found: the ancestors drawn for them must not
		// reach a command such as done.
		var hit []mount.Entry
		for _, d := range drawn {
			if !d.node.Context {
				hit = append(hit, mount.Entry{Store: d.store.Store, Entry: d.node.Entry})
			}
		}
		return a.printULIDs(hit)
	case tmpl != nil:
		node := map[*index.Entry]*ops.TreeNode{}
		es := make([]mount.Entry, len(drawn))
		for i, d := range drawn {
			node[d.node.Entry], es[i] = d.node, mount.Entry{Store: d.store.Store, Entry: d.node.Entry}
		}
		return a.printFormat(es, tmpl, func(m map[string]any, e mount.Entry) {
			m["context"], m["in_cycle"] = node[e.Entry].Context, node[e.Entry].InCycle
		})
	case a.jsonOut:
		out := make([]*ops.TaskView, len(tops))
		for i, d := range tops {
			out[i] = ops.TreeViewOf(d.store.Index, d.node)
		}
		return a.printJSON(out)
	}
	var b strings.Builder
	st := a.style()
	for _, ts := range res.Stores {
		if res.Many {
			writeStoreTree(&b, st, ts)
			continue
		}
		for _, n := range ts.Roots {
			writeNode(&b, st, ts.Index, n, "", "")
		}
		if len(ts.Cycle) > 0 {
			b.WriteString("\n== in a parent cycle\n")
			for _, n := range ts.Cycle {
				writeNode(&b, st, ts.Index, n, "", "")
			}
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// writeStoreTree writes the trees of one store of several: a heading with
// its name, and the tasks in a parent cycle under their own heading that
// names the store. A store with nothing to show has no heading.
func writeStoreTree(b *strings.Builder, st style, ts ops.TreeStore) {
	section := func(heading string, nodes []*ops.TreeNode) {
		if len(nodes) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("== " + heading + "\n")
		for _, n := range nodes {
			writeNode(b, st, ts.Index, n, "", "")
		}
	}
	section(ts.Store.Name, ts.Roots)
	section(ts.Store.Name+" (in a parent cycle)", ts.Cycle)
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
	r := taskRow(ix, e)
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
