package cli

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
)

func (a *app) lsTree(query string, ulidsOnly bool, tmpl *template.Template) error {
	res, err := browse.Tree(a.ss, query)
	if err != nil {
		return err
	}
	switch {
	case ulidsOnly || tmpl != nil:
		// every store's trees, roots then the cycle, in the order they are drawn
		var drawn []*usecase.TaskView
		for _, ts := range res.Stores {
			drawn = append(drawn, ts.Drawn()...)
		}
		if tmpl != nil {
			return a.printFormat(drawn, tmpl, func(m map[string]any, v *usecase.TaskView) {
				m["context"], m["in_cycle"] = v.Context, v.InCycle
			})
		}
		// Only the tasks found: the ancestors drawn for them must not
		// reach a command such as done.
		var hit []*usecase.TaskView
		for _, v := range drawn {
			if !v.Context {
				hit = append(hit, v)
			}
		}
		return a.printULIDs(hit)
	case a.jsonOut:
		return a.printJSON(res.JSON())
	}
	var b strings.Builder
	st := a.style()
	for _, ts := range res.Stores {
		if res.Many {
			writeStoreTree(&b, st, ts)
			continue
		}
		for _, n := range ts.Roots {
			writeNode(&b, st, n, "", "")
		}
		if len(ts.Cycle) > 0 {
			b.WriteString("\n== in a parent cycle\n")
			for _, n := range ts.Cycle {
				writeNode(&b, st, n, "", "")
			}
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// writeStoreTree writes the trees of one store of several: a heading with
// its name, and the tasks in a parent cycle under their own heading that
// names the store. A store with nothing to show has no heading.
func writeStoreTree(b *strings.Builder, st style, ts browse.TreeStoreView) {
	section := func(heading string, nodes []*usecase.TaskView) {
		if len(nodes) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("== " + heading + "\n")
		for _, n := range nodes {
			writeNode(b, st, n, "", "")
		}
	}
	section(ts.Store, ts.Roots)
	section(ts.Store+" (in a parent cycle)", ts.Cycle)
}

func writeNode(b *strings.Builder, st style, n *usecase.TaskView, lead, childLead string) {
	b.WriteString(lead + treeLine(st, n) + "\n")
	for i, c := range n.Children {
		if i == len(n.Children)-1 {
			writeNode(b, st, c, childLead+"└─ ", childLead+"   ")
		} else {
			writeNode(b, st, c, childLead+"├─ ", childLead+"│  ")
		}
	}
}

// treeLine is one task of the tree: id and ULID, status, kind and title,
// then the checklist and the progress of a project.
func treeLine(st style, v *usecase.TaskView) string {
	r := taskRow(v)
	name := r[0] + " " + r[1]
	line := name + "  " + r[2] + "  " + r[3] + "  " + r[6]
	if p := usecase.ProgressText(v.Checklist, v.Progress, v.Steps); p != "" {
		line += "  " + p
	}
	if v.Context {
		return st.dim(line)
	}
	return st.id(name) + line[len(name):]
}
