package gitstore

import (
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// view と種類ごとの Reader は、偽物(app/apptest)にもほぼ同じものがある。gitstore は apptest を
// import できず、port は口の定義だけを置くので、まとめない。両者は RunStoresContract で揃える。
//
// view は、作業ツリーのログと、同じ Tx で書いたログ(pending)を合わせて読む。作業ツリーは、
// ロックを持っているあいだ、いつもいまのコミットと同じ中身である。
type view struct {
	root    string
	pending map[change.Ref][]change.Record
}

// log は、呼び出し側が変えてもまだ書いていない行(pending)が変わらないように、pending の行を深く写して返す。
// 作業ツリーの行は毎回ファイルから読むので写さない。
func (v *view) log(ref change.Ref) ([]change.Record, error) {
	base, err := readLog(v.root, ref)
	if err != nil {
		return nil, err
	}
	for _, r := range v.pending[ref] {
		base = append(base, cloneRecord(r))
	}
	return base, nil
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

func (v *view) ids(kind change.Kind) ([]change.ID, error) {
	ids, err := listIDs(v.root, kind)
	if err != nil {
		return nil, err
	}
	for ref := range v.pending {
		if ref.Kind == kind && !slices.Contains(ids, ref.ID) {
			ids = append(ids, ref.ID)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

func (v *view) Workspaces() port.WorkspaceReader { return workspaces{v} }
func (v *view) Projects() port.ProjectReader     { return projects{v} }
func (v *view) Items() port.ItemReader           { return items{v} }
func (v *view) Members() port.MemberReader       { return members{v} }
func (v *view) Changes() port.ChangeReader       { return changes{v} }

// get は、ログを読んで from で値にする。作成の行がなければ port.ErrNotFound。
func get[T any](v *view, ref change.Ref, from func([]change.Record) (T, bool)) (T, error) {
	var zero T
	log, err := v.log(ref)
	if err != nil {
		return zero, err
	}
	x, ok := from(log)
	if !ok {
		return zero, port.ErrNotFound
	}
	return x, nil
}

type workspaces struct{ v *view }

func (r workspaces) Get(id workspace.ID) (workspace.Workspace, error) {
	return get(r.v, change.Ref{Kind: change.KindWorkspace, ID: id}, workspace.FromLog)
}

func (r workspaces) List() ([]workspace.Workspace, error) {
	ids, err := r.v.ids(change.KindWorkspace)
	if err != nil {
		return nil, err
	}
	var out []workspace.Workspace
	for _, id := range ids {
		if w, err := r.Get(id); err == nil {
			out = append(out, w)
		}
	}
	return out, nil
}

type projects struct{ v *view }

func (r projects) Get(id project.ID) (project.Project, error) {
	return get(r.v, change.Ref{Kind: change.KindProject, ID: id}, project.FromLog)
}

type items struct{ v *view }

func (r items) Get(id item.ID) (item.Item, error) {
	return get(r.v, change.Ref{Kind: change.KindItem, ID: id}, item.FromLog)
}

type members struct{ v *view }

func (r members) List() ([]member.Member, error) {
	log, err := r.v.log(change.Ref{Kind: change.KindMembers})
	if err != nil {
		return nil, err
	}
	return member.FromLog(log), nil
}

type changes struct{ v *view }

func (r changes) Of(ref change.Ref) ([]change.Record, error) {
	log, err := r.v.log(ref)
	if err != nil {
		return nil, err
	}
	if len(log) == 0 {
		return nil, port.ErrNotFound
	}
	return log, nil
}
