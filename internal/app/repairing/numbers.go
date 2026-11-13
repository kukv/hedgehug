package repairing

import (
	"context"
	"errors"
	"strconv"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// numbered は、番号を持つもの 1 つ。ref はログ、place は、番号を n にした場所の値、display は、番号 n の画面の ID。
type numbered struct {
	repair.Numbered
	ref     change.Ref
	place   func(n int) (from, to any)
	display func(n int) string
}

// renumbering は、ワークスペースの 1 つの連番(アイテムかプロジェクト)の、いまの番号の使われ方。
type renumbering struct {
	all  []repair.Numbered
	next int
	by   map[change.ID]numbered
}

// renumberCollisions は、同じワークスペースで番号が重なったアイテムとプロジェクトを、番号を振り直して直す(決定 C3a-13)。
// 1 件ごとに 1 回保存し、次の番号は書くたびに読み直す。直したあとは重ならないので、なくなるまで繰り返す。
func renumberCollisions(ctx context.Context, w port.RepairWriter) error {
	wss, err := workspaceIDs(w)
	if err != nil {
		return err
	}
	for _, ws := range wss {
		for _, kind := range []numbering.Kind{numbering.Items, numbering.Projects} {
			if err := renumberKind(ctx, w, ws, kind); err != nil {
				return err
			}
		}
	}
	return nil
}

func workspaceIDs(w port.RepairWriter) ([]workspace.ID, error) {
	var ids []workspace.ID
	err := w.Read(func(r port.Reader) error {
		wss, err := r.Workspaces().List()
		for _, ws := range wss {
			ids = append(ids, ws.ID)
		}
		return err
	})
	return ids, err
}

// renumberKind は、ワークスペース ws の kind の連番で、重なった番号がなくなるまで 1 件ずつ振り直す。
func renumberKind(ctx context.Context, w port.RepairWriter, ws workspace.ID, kind numbering.Kind) error {
	var prev repair.Renumber
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		cur, err := readNumbers(w, ws, kind)
		if err != nil {
			return err
		}
		rs := repair.Renumbers(cur.all, w.Facts(), cur.next)
		if len(rs) == 0 {
			return nil
		}
		if rs[0] == prev {
			return errors.New("重なった番号の振り直しが進みません")
		}
		prev = rs[0]
		n := cur.by[rs[0].ID]
		op := port.Operation{Actor: syncActor, Summary: n.display(rs[0].From) + " を " + n.display(rs[0].To) + " に振り直し"}
		err = w.Write(op, func(tx port.Tx) error {
			from, to := n.place(rs[0].To)
			return errors.Join(
				tx.WriteWorkspaces().Append(ws, numbering.Draft(kind, rs[0].To)),
				appendTo(tx, n.ref, change.NewSet(placeField, from, to)),
			)
		})
		if err != nil {
			return err
		}
	}
}

// readNumbers は、ワークスペース ws の kind の連番について、削除済みを含むすべての番号と、次に振る番号を読む。
func readNumbers(w port.RepairWriter, ws workspace.ID, kind numbering.Kind) (renumbering, error) {
	cur := renumbering{by: map[change.ID]numbered{}}
	err := w.Read(func(r port.Reader) error {
		wsLog, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
		if err != nil {
			return err
		}
		cur.next = numbering.Next(wsLog, kind)
		ns, err := listNumbered(r, ws, kind)
		for _, n := range ns {
			cur.all = append(cur.all, n.Numbered)
			cur.by[n.ID] = n
		}
		return err
	})
	return cur, err
}

// listNumbered は、ワークスペース ws の kind の連番を持つもの(削除済みを含む)を、作成の行つきで読む。
func listNumbered(r port.Reader, ws workspace.ID, kind numbering.Kind) ([]numbered, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	var prefix string
	for _, x := range wss {
		if x.ID == ws {
			prefix = x.Prefix
		}
	}
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var ns []numbered
	if kind == numbering.Projects {
		ns = projectNumbers(ps, prefix)
	} else if ns, err = itemNumbers(r, ps, prefix); err != nil {
		return nil, err
	}
	for i := range ns {
		log, err := r.Changes().Of(ns[i].ref)
		if err != nil {
			return nil, err
		}
		ns[i].Create = createRow(log)
	}
	return ns, nil
}

// createRow は、ログの作成の行。
func createRow(log []change.Record) change.Record {
	for _, r := range log {
		if r.T == change.Create {
			return r
		}
	}
	return change.Record{}
}

func projectNumbers(ps []project.Project, prefix string) []numbered {
	var out []numbered
	for _, p := range ps {
		out = append(out, numbered{
			Numbered: repair.Numbered{ID: p.ID, Number: p.Place.Number},
			ref:      change.Ref{Kind: change.KindProject, ID: p.ID},
			place: func(n int) (any, any) {
				to := p.Place
				to.Number = n
				return p.Place, to
			},
			display: func(n int) string { return prefix + "-P" + strconv.Itoa(n) },
		})
	}
	return out
}

func itemNumbers(r port.Reader, ps []project.Project, prefix string) ([]numbered, error) {
	its, err := itemsOf(r, ps)
	if err != nil {
		return nil, err
	}
	var out []numbered
	for _, it := range its {
		out = append(out, numbered{
			Numbered: repair.Numbered{ID: it.ID, Number: it.Place.Number},
			ref:      change.Ref{Kind: change.KindItem, ID: it.ID},
			place: func(n int) (any, any) {
				to := it.Place
				to.Number = n
				return it.Place, to
			},
			display: func(n int) string { return prefix + "-" + strconv.Itoa(n) },
		})
	}
	return out, nil
}
