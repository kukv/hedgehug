package items

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// templateOf は、作成で使うテンプレートを、保存の前に読む(A3b-6)。アクティビティの件数と作る構成は、
// この同じ中身から決める。テンプレートを選んでいない、またはプロジェクトがなければ nil(プロジェクトの
// 断りは保存の中で出す)。ないか種類が違うときも nil を返し、保存の中で ErrNotFound にする(A3b-7)。
func (s Service) templateOf(ctx context.Context, in CreateInput) (*workspace.Template, error) {
	if in.Template == nil || in.Project == "" {
		return nil, nil //nolint:nilnil // テンプレートなしは正しい結果
	}
	var found *workspace.Template
	err := s.Stores.Read(ctx, in.Store, func(r port.Reader) error {
		p, err := r.Projects().Get(in.Project)
		if err != nil {
			return err
		}
		st, err := r.Workspaces().Settings(p.Place.Workspace)
		if err != nil {
			return err
		}
		if t, ok := st.Template(*in.Template); ok && t.Kind == string(in.Kind) {
			found = &t
		}
		return nil
	})
	return found, err
}

// descTexts は、作るアイテムの説明の本文(内部 ID → 本文)。
type descTexts map[item.ID]string

// set は、空白だけでない説明を入れる(空の説明はファイルを置かない。A4a-1)。
func (d descTexts) set(id item.ID, text string) {
	if strings.TrimSpace(text) != "" {
		d[id] = text
	}
}

// others は、テンプレートと一緒に作る子と Step の数。
func others(t workspace.Template) int {
	n := len(t.Children) + len(t.Steps)
	for _, c := range t.Children {
		n += len(c.Steps)
	}
	return n
}

// withTemplate は、もとになるアイテム root と、テンプレートの構成から作る子と Step を、番号の順に返す。
// 2 つ目の値は、作るアイテムの説明(説明のないアイテムは入れない)。もとになるアイテムは入力の説明、
// 子はテンプレートの子の説明、Step は説明を持たない(A4a-4)。
// テンプレートを選んだのに読めなかったときは、ほかの検査のあとで ErrNotFound にする(A3b-7)。
func (s Service) withTemplate(tx port.Tx, in CreateInput, t *workspace.Template, root item.Item, me string) ([]item.Item, descTexts, error) {
	texts := descTexts{}
	texts.set(root.ID, in.Description)
	if in.Template == nil {
		return []item.Item{root}, texts, nil
	}
	if t == nil {
		return nil, nil, port.ErrNotFound
	}
	st, err := tx.Workspaces().Settings(root.Place.Workspace)
	if err != nil {
		return nil, nil, err
	}
	e := expander{st: st, me: me, newID: s.IDs.New, next: root.Place.Number, out: []item.Item{root}}
	e.steps(root, t.Steps)
	r := ""
	for _, c := range t.Children {
		r = rank.After(r)
		child := e.add(item.Kind(c.Kind), root, c.Title, r, workspace.TemplateValuesFor(st, c.Kind, c.Values))
		texts.set(child.ID, c.Description)
		e.steps(child, c.Steps)
	}
	return e.out, texts, nil
}

// expander は、テンプレートの構成からアイテムを深さ優先の番号で作る(A3b-10・A3b-11)。
type expander struct {
	st    workspace.Settings
	me    string
	newID func() change.ID
	next  int
	out   []item.Item
}

func (e *expander) add(kind item.Kind, parent item.Item, title, r string, vals map[change.ID]json.RawMessage) item.Item {
	e.next++
	pid := parent.ID
	it := item.Item{
		ID:       e.newID(),
		Place:    item.Place{Workspace: parent.Place.Workspace, Project: parent.Place.Project, Parent: &pid, Number: e.next},
		Kind:     kind,
		Rank:     r,
		Title:    item.NormalizeTitle(title),
		Watchers: []string{e.me},
	}
	applyValues(&it, vals, e.st)
	e.out = append(e.out, it)
	return it
}

// steps は、parent の下に Step を並び順に作る。担当者は「親と同じ」なら parent の担当者、「未割り当て」ならなし。
func (e *expander) steps(parent item.Item, ss []workspace.TemplateStep) {
	r := ""
	for _, s := range ss {
		r = rank.After(r)
		who := s.Assignee
		switch who {
		case workspace.StepParent:
			who = parent.Assignee
		case workspace.StepNone:
			who = ""
		}
		var vals map[change.ID]json.RawMessage
		if who != "" {
			vals = map[change.ID]json.RawMessage{"assignee": change.Value(who)}
		}
		e.add(item.Step, parent, s.Title, r, vals)
	}
}
