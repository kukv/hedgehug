package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/query"
)

// treeNode is one task in ls --tree.
type treeNode struct {
	e        *index.Entry
	context  bool // shown only to connect the tasks found
	children []*treeNode
}

func (a *app) lsTree(q *query.Query, idsOnly bool, tmpl *template.Template) error {
	if hasTerm(q, "group") || hasTerm(q, "sort") {
		return usageErrorf("--tree orders tasks by rank; it takes neither group: nor sort:")
	}
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
	var found []*index.Entry
	for _, g := range res.Groups {
		found = append(found, g.Tasks...)
	}
	roots, cycle := treeOf(ix, found)
	match := map[model.ID]bool{}
	for _, e := range found {
		match[e.ID] = true
	}
	switch {
	case idsOnly:
		// Only the tasks found: the ancestors drawn for them must not
		// reach a command such as done.
		var hit []*index.Entry
		for _, e := range flatten(roots, cycle) {
			if match[e.ID] {
				hit = append(hit, e)
			}
		}
		return a.printIDs(ix, hit)
	case tmpl != nil:
		inCycle := map[model.ID]bool{}
		for _, e := range cycle {
			inCycle[e.ID] = true
		}
		return a.printFormat(ix, flatten(roots, cycle), tmpl, func(m map[string]any, e *index.Entry) {
			m["context"], m["in_cycle"] = !match[e.ID], inCycle[e.ID]
		})
	case a.jsonOut:
		out := make([]*taskJSON, 0, len(roots)+len(cycle))
		for _, n := range roots {
			out = append(out, nodeJSON(ix, n))
		}
		for _, e := range cycle {
			j := entryJSON(ix, e)
			j.InCycle = true
			out = append(out, j)
		}
		return a.printJSON(out)
	}
	var b strings.Builder
	st := a.style()
	for _, n := range roots {
		writeNode(&b, ix, st, n, "", "")
	}
	if len(cycle) > 0 {
		b.WriteString("\n== in a parent cycle\n")
		for _, e := range cycle {
			b.WriteString(treeLine(ix, st, e, false) + "\n")
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// treeOf builds the tree of the tasks found and of the ancestors that
// connect them to a root. Tasks in a parent cycle never reach a root; they
// are returned apart, by ID.
func treeOf(ix *index.Index, found []*index.Entry) (roots []*treeNode, cycle []*index.Entry) {
	match := map[model.ID]bool{}
	show := map[model.ID]bool{}
	for _, e := range found {
		match[e.ID] = true
		for cur := e; cur != nil && !show[cur.ID]; {
			show[cur.ID] = true
			if cur.Parent == nil {
				break
			}
			cur, _ = ix.Task(*cur.Parent)
		}
	}
	seen := map[model.ID]bool{}
	var build func(e *index.Entry) *treeNode
	build = func(e *index.Entry) *treeNode {
		seen[e.ID] = true
		n := &treeNode{e: e, context: !match[e.ID]}
		for _, c := range ix.Children(e.ID) {
			if show[c.ID] && !seen[c.ID] {
				n.children = append(n.children, build(c))
			}
		}
		return n
	}
	for _, r := range ix.Roots() {
		if show[r.ID] {
			roots = append(roots, build(r))
		}
	}
	for id := range show {
		if !seen[id] {
			e, _ := ix.Task(id)
			cycle = append(cycle, e)
		}
	}
	slices.SortFunc(cycle, func(x, y *index.Entry) int { return cmp.Compare(x.ID.String(), y.ID.String()) })
	return roots, cycle
}

func writeNode(b *strings.Builder, ix *index.Index, st style, n *treeNode, lead, childLead string) {
	b.WriteString(lead + treeLine(ix, st, n.e, n.context) + "\n")
	for i, c := range n.children {
		if i == len(n.children)-1 {
			writeNode(b, ix, st, c, childLead+"└─ ", childLead+"   ")
		} else {
			writeNode(b, ix, st, c, childLead+"├─ ", childLead+"│  ")
		}
	}
}

// treeLine is one task of the tree: short ID, status, kind and title.
func treeLine(ix *index.Index, st style, e *index.Entry, context bool) string {
	r := taskRow(ix, e)
	line := r[0] + "  " + r[1] + "  " + r[2] + "  " + r[5]
	if context {
		return st.dim(line)
	}
	return st.id(r[0]) + line[len(r[0]):]
}

func nodeJSON(ix *index.Index, n *treeNode) *taskJSON {
	j := entryJSON(ix, n.e)
	j.Context = n.context
	for _, c := range n.children {
		j.Children = append(j.Children, nodeJSON(ix, c))
	}
	return j
}

// flatten lists the tree in the order it is drawn, then the cycle.
func flatten(roots []*treeNode, cycle []*index.Entry) []*index.Entry {
	var out []*index.Entry
	var walk func(n *treeNode)
	walk = func(n *treeNode) {
		out = append(out, n.e)
		for _, c := range n.children {
			walk(c)
		}
	}
	for _, n := range roots {
		walk(n)
	}
	return append(out, cycle...)
}
