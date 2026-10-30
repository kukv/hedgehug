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
	"github.com/kukv/hedgehug/internal/schema"
)

// treeNode is one task in ls --tree.
type treeNode struct {
	e        *index.Entry
	context  bool // shown only to connect the tasks found
	inCycle  bool // in a parent cycle, out of reach of the roots
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
	tops := append(slices.Clone(roots), cycle...)
	drawn := flatten(tops)
	switch {
	case idsOnly:
		// Only the tasks found: the ancestors drawn for them must not
		// reach a command such as done.
		var hit []*index.Entry
		for _, n := range drawn {
			if !n.context {
				hit = append(hit, n.e)
			}
		}
		return a.printIDs(ix, hit)
	case tmpl != nil:
		node := map[model.ID]*treeNode{}
		es := make([]*index.Entry, len(drawn))
		for i, n := range drawn {
			node[n.e.ID], es[i] = n, n.e
		}
		return a.printFormat(ix, es, tmpl, func(m map[string]any, e *index.Entry) {
			m["context"], m["in_cycle"] = node[e.ID].context, node[e.ID].inCycle
		})
	case a.jsonOut:
		out := make([]*taskJSON, len(tops))
		for i, n := range tops {
			out[i] = nodeJSON(ix, n)
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
		for _, n := range cycle {
			writeNode(&b, ix, st, n, "", "")
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// treeOf builds the tree of the tasks found and of the ancestors that
// connect them to a root. Tasks in a parent cycle never reach a root; they
// are returned apart, by ID, each as the root of the tasks under it that
// are not in the cycle.
func treeOf(ix *index.Index, found []*index.Entry) (roots, cycle []*treeNode) {
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
	// What the roots do not reach lies in a cycle or under one. Mark the
	// tasks in the cycle seen first, so that none is drawn under another.
	var members []*index.Entry
	for id := range show {
		if !seen[id] && inParentCycle(ix, id) {
			e, _ := ix.Task(id)
			members = append(members, e)
		}
	}
	slices.SortFunc(members, func(x, y *index.Entry) int { return cmp.Compare(x.ID.String(), y.ID.String()) })
	for _, e := range members {
		seen[e.ID] = true
	}
	for _, e := range members {
		n := build(e)
		n.inCycle = true
		cycle = append(cycle, n)
	}
	return roots, cycle
}

// inParentCycle reports whether the parents of id lead back to it.
func inParentCycle(ix *index.Index, id model.ID) bool {
	for _, p := range ix.Problems(id) {
		if p.Code == schema.CodeCycle && p.Field == "parent" {
			return true
		}
	}
	return false
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
	j.Context, j.InCycle = n.context, n.inCycle
	for _, c := range n.children {
		j.Children = append(j.Children, nodeJSON(ix, c))
	}
	return j
}

// flatten lists the trees under roots in the order they are drawn.
func flatten(roots []*treeNode) []*treeNode {
	var out []*treeNode
	var walk func(n *treeNode)
	walk = func(n *treeNode) {
		out = append(out, n)
		for _, c := range n.children {
			walk(c)
		}
	}
	for _, n := range roots {
		walk(n)
	}
	return out
}
