package apptest

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// view と種類ごとの Reader は、本物(infra/gitstore)にもほぼ同じものがある。gitstore は apptest を
// import できず、port は口の定義だけを置くので、まとめない。両者は RunStoresContract で揃える。
//
// view は、保存済みのログ(base)と、同じ Tx で書いたログ(pending)を合わせて読む。
type view struct {
	base    map[change.Ref][]change.Record
	pending map[change.Ref][]change.Record
}

// log は、呼び出し側が変えても保存済みのログが変わらないように、行を深く写して返す。
// ref が Valid でなければ空(読み取りは port.ErrNotFound になる)。
func (v *view) log(ref change.Ref) []change.Record {
	if !ref.Valid() {
		return nil
	}
	out := make([]change.Record, 0, len(v.base[ref])+len(v.pending[ref]))
	for _, r := range append(slices.Clone(v.base[ref]), v.pending[ref]...) {
		out = append(out, cloneRecord(r))
	}
	return out
}

func cloneRecord(r change.Record) change.Record {
	r.Prev = slices.Clone(r.Prev)
	r.From = slices.Clone(r.From)
	r.To = slices.Clone(r.To)
	if r.Root != nil {
		root := *r.Root
		r.Root = &root
	}
	return r
}

func (v *view) ids(kind change.Kind) []change.ID {
	seen := map[change.ID]bool{}
	for _, m := range []map[change.Ref][]change.Record{v.base, v.pending} {
		for ref := range m {
			if ref.Kind == kind {
				seen[ref.ID] = true
			}
		}
	}
	out := make([]change.ID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.SortFunc(out, func(a, b change.ID) int { return strings.Compare(string(a), string(b)) })
	return out
}

func (v *view) Workspaces() port.WorkspaceReader { return workspaces{v} }
func (v *view) Projects() port.ProjectReader     { return projects{v} }
func (v *view) Items() port.ItemReader           { return items{v} }
func (v *view) Members() port.MemberReader       { return members{v} }
func (v *view) Changes() port.ChangeReader       { return changes{v} }

type workspaces struct{ v *view }

func (r workspaces) Get(id workspace.ID) (workspace.Workspace, error) {
	w, ok := workspace.FromLog(r.v.log(change.Ref{Kind: change.KindWorkspace, ID: id}))
	if !ok {
		return workspace.Workspace{}, port.ErrNotFound
	}
	return w, nil
}

func (r workspaces) List() ([]workspace.Workspace, error) {
	var out []workspace.Workspace
	for _, id := range r.v.ids(change.KindWorkspace) {
		if w, err := r.Get(id); err == nil {
			out = append(out, w)
		}
	}
	return out, nil
}

type projects struct{ v *view }

func (r projects) Get(id project.ID) (project.Project, error) {
	p, ok := project.FromLog(r.v.log(change.Ref{Kind: change.KindProject, ID: id}))
	if !ok {
		return project.Project{}, port.ErrNotFound
	}
	return p, nil
}

func (r projects) List(ws workspace.ID) ([]project.Project, error) {
	var out []project.Project
	for _, id := range r.v.ids(change.KindProject) {
		if p, err := r.Get(id); err == nil && p.Place.Workspace == ws {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b project.Project) int {
		return cmp.Or(strings.Compare(a.Rank, b.Rank), strings.Compare(string(a.ID), string(b.ID)))
	})
	return out, nil
}

type items struct{ v *view }

func (r items) Get(id item.ID) (item.Item, error) {
	it, ok := item.FromLog(r.v.log(change.Ref{Kind: change.KindItem, ID: id}))
	if !ok {
		return item.Item{}, port.ErrNotFound
	}
	return it, nil
}

func (r items) List(p project.ID) ([]item.Item, error) {
	var out []item.Item
	for _, id := range r.v.ids(change.KindItem) {
		if it, err := r.Get(id); err == nil && it.Place.Project == p {
			out = append(out, it)
		}
	}
	slices.SortFunc(out, func(a, b item.Item) int { return cmp.Compare(a.Place.Number, b.Place.Number) })
	return out, nil
}

type members struct{ v *view }

func (r members) List() ([]member.Member, error) {
	return member.FromLog(r.v.log(change.Ref{Kind: change.KindMembers})), nil
}

type changes struct{ v *view }

func (r changes) Of(ref change.Ref) ([]change.Record, error) {
	log := r.v.log(ref)
	if len(log) == 0 {
		return nil, port.ErrNotFound
	}
	return log, nil
}
