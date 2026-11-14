package bulkchange

import (
	"context"
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Member は、担当者の選択肢。
type Member struct{ Login, Name string }

// Choices は、ダイアログの値の選択肢(F2b2-3)。Kinds は選んだアイテムの種類(F2b2-4)。
type Choices struct {
	Kinds, Statuses, Priorities, Sprints, Releases, Systems []string
	Members                                                 []Member
}

// kindOrder は、種類の並び。
var kindOrder = []item.Kind{item.Epic, item.Story, item.Task, item.Bug, item.Step}

// wsKey は、Store をまたいで区別するワークスペース。
type wsKey struct {
	store port.StoreID
	ws    workspace.ID
}

// wsChoices は、1 つのワークスペースから読んだ、選択肢と判定の材料。projects は、選んだアイテムのプロジェクト。
// autos(自動化のルール)と prefix(接頭辞)は、一括変更の保存で使う。
type wsChoices struct {
	settings workspace.Settings
	plan     planning.Plan
	rules    projectrule.Set
	autos    []automation.Rule
	prefix   string
	projects []project.ID
	deleted  bool // ワークスペースが削除済み。ほかの項目は読まない
}

// picked は、選んだアイテム 1 つの、選択肢に要る部分。
type picked struct {
	kind item.Kind
	key  wsKey
}

// Choices は、targets から選択肢を作る。閲覧のみの Store のもの・削除済みのもの・存在しないものは見ない。
// 一覧にない Store があれば、何も読まずに port.ErrStoreNotFound。
func (s Service) Choices(ctx context.Context, targets []Target) (Choices, error) {
	groups, err := s.groups(ctx, targets)
	if err != nil {
		return Choices{}, err
	}
	l := loaded{byTarget: map[Target]picked{}, wss: map[wsKey]*wsChoices{}, members: map[port.StoreID][]member.Member{}}
	for _, g := range groups {
		if g.readOnly {
			continue
		}
		if err := s.Stores.Read(ctx, g.id, func(r port.Reader) error { return l.read(r, g) }); err != nil {
			return Choices{}, err
		}
	}
	return build(targets, l), nil
}

// loaded は、選んだアイテムから読んだ、選択肢の材料。
type loaded struct {
	byTarget map[Target]picked
	wss      map[wsKey]*wsChoices
	members  map[port.StoreID][]member.Member
}

// read は、Store g の選んだアイテムと、そのワークスペース・メンバーを読む。
func (l loaded) read(r port.Reader, g storeGroup) error {
	ms, err := r.Members().List()
	if err != nil {
		return err
	}
	l.members[g.id] = ms
	for _, t := range g.targets {
		it, err := port.LiveItem(r, t.ID)
		if errors.Is(err, port.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if err := l.add(r, g.id, t, it); err != nil {
			return err
		}
	}
	return nil
}

// add は、アイテム it を材料に足す。ワークスペースは初めて出たときに読む。
func (l loaded) add(r port.Reader, store port.StoreID, t Target, it item.Item) error {
	key := wsKey{store, it.Place.Workspace}
	if l.wss[key] == nil {
		w, err := readWorkspace(r, key.ws)
		if err != nil {
			return err
		}
		l.wss[key] = w
	}
	if l.wss[key].deleted {
		return nil // ワークスペースが削除済み(アイテムは残っている)。削除済みのアイテムと同じに見ない
	}
	if p := it.Place.Project; !slices.Contains(l.wss[key].projects, p) {
		l.wss[key].projects = append(l.wss[key].projects, p)
	}
	l.byTarget[t] = picked{kind: it.Kind, key: key}
	return nil
}

// readWorkspace は、ワークスペースの設定・スプリントとリリース・プロジェクト別のルール・自動化のルールを読む。
// スプリントとリリースの読み方は app/planning の planOf と同じ(app の機能どうしは import しない)。
func readWorkspace(r port.Reader, ws workspace.ID) (*wsChoices, error) {
	w, err := port.LiveWorkspace(r, ws)
	if errors.Is(err, port.ErrNotFound) {
		return &wsChoices{deleted: true}, nil
	}
	if err != nil {
		return nil, err
	}
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return nil, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return nil, err
	}
	return &wsChoices{
		settings: st, plan: planning.FromLog(log), rules: projectrule.FromLog(log), autos: automation.FromLog(log), prefix: w.Prefix,
	}, nil
}

// build は、選んだ順(targets)にワークスペースとメンバーを見て、まだ出ていない名前を足していく。
func build(targets []Target, l loaded) Choices {
	var out Choices
	seenWs := map[wsKey]bool{}
	seenStore := map[port.StoreID]bool{}
	kinds := map[item.Kind]bool{}
	for _, t := range targets {
		p, ok := l.byTarget[t]
		if !ok {
			continue
		}
		kinds[p.kind] = true
		if !seenStore[p.key.store] {
			seenStore[p.key.store] = true
			out.Members = addMembers(out.Members, l.members[p.key.store])
		}
		if !seenWs[p.key] {
			seenWs[p.key] = true
			addWorkspace(&out, l.wss[p.key])
		}
	}
	for _, k := range kindOrder {
		if kinds[k] {
			out.Kinds = append(out.Kinds, string(k))
		}
	}
	return out
}

// addMembers は、ms のうち、まだいないログイン名(大文字と小文字を区別しない)を足す。
func addMembers(out []Member, ms []member.Member) []Member {
	for _, m := range ms {
		if !slices.ContainsFunc(out, func(x Member) bool { return sameName(x.Login, m.Login) }) {
			out = append(out, Member{Login: m.Login, Name: m.Name})
		}
	}
	return out
}

// addWorkspace は、ワークスペース w の値のうち、まだ出ていない名前を、それぞれの一覧の順で足す。
func addWorkspace(out *Choices, w *wsChoices) {
	for _, v := range w.settings.Statuses {
		if usedByAny(w, v.ID) {
			out.Statuses = addName(out.Statuses, v.Name)
		}
	}
	for _, v := range w.settings.Priorities {
		out.Priorities = addName(out.Priorities, v.Name)
	}
	for _, v := range w.settings.Systems {
		out.Systems = addName(out.Systems, v.Name)
	}
	for _, sp := range w.plan.LiveSprints() {
		if sp.State != planning.Closed {
			out.Sprints = addName(out.Sprints, sp.Name)
		}
	}
	for _, rl := range w.plan.LiveReleases() {
		if rl.State != planning.Archived {
			out.Releases = addName(out.Releases, rl.Name)
		}
	}
}

// usedByAny は、選んだアイテムのプロジェクトのどれかがステータス id を使うか。
func usedByAny(w *wsChoices, id change.ID) bool {
	return slices.ContainsFunc(w.projects, func(p project.ID) bool {
		return projectrule.Uses(w.settings, w.rules.Of(p), id)
	})
}

// addName は、names にない名前(大文字と小文字を区別しない)なら足す。表記は先に出たもの。
func addName(names []string, name string) []string {
	if slices.ContainsFunc(names, func(n string) bool { return sameName(n, name) }) {
		return names
	}
	return append(names, name)
}
