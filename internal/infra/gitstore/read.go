package gitstore

import (
	"cmp"
	"errors"
	"slices"
	"strings"

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
	// texts は、同じ Tx で書いた説明(ない説明は "")。いまの作業ツリーと同じ説明は入れない。
	texts map[change.Ref]string
	// base は、この view で読んだ作業ツリーのログ。ロックを持っている 1 回の Read/Write のあいだ、
	// 作業ツリーは変わらないので、プロジェクトごとに全アイテムを読む List で読み直さない。pending は入れない。
	base map[change.Ref][]change.Record
	// places は、この view で読んだアイテムのプロジェクト。items.List は、別のプロジェクトにあると分かっている
	// アイテムをログから作り直さない。Tx でアイテムに行を足したら、そのアイテムの分を消す。
	places map[item.ID]project.ID
	// projectPlaces は、この view で読んだプロジェクトのワークスペース。projects.List は、別のワークスペースに
	// あると分かっているプロジェクトをログから作り直さない。Tx でプロジェクトに行を足したら、そのプロジェクトの分を消す。
	projectPlaces map[project.ID]workspace.ID
	// gone は、同じ Tx で完全に削除したもの。ログは空として読み、ids に出さない。
	gone map[change.Ref]bool
}

// log は、呼び出し側が変えても、覚えた作業ツリーの行(base)とまだ書いていない行(pending)が
// 変わらないように、どちらも深く写して返す。ref が Valid でなければ空(読み取りは port.ErrNotFound になる)。
func (v *view) log(ref change.Ref) ([]change.Record, error) {
	if !ref.Valid() || v.gone[ref] {
		return nil, nil
	}
	base, ok := v.base[ref]
	if !ok {
		var err error
		if base, err = readLog(v.root, ref); err != nil {
			return nil, err
		}
		if v.base == nil {
			v.base = map[change.Ref][]change.Record{}
		}
		v.base[ref] = base
	}
	out := make([]change.Record, 0, len(base)+len(v.pending[ref]))
	for _, r := range base {
		out = append(out, cloneRecord(r))
	}
	for _, r := range v.pending[ref] {
		out = append(out, cloneRecord(r))
	}
	return out, nil
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
	ids = slices.DeleteFunc(ids, func(id change.ID) bool { return v.gone[change.Ref{Kind: kind, ID: id}] })
	slices.Sort(ids)
	return ids, nil
}

func (v *view) Workspaces() port.WorkspaceReader { return workspaces{v} }
func (v *view) Projects() port.ProjectReader     { return projects{v} }
func (v *view) Items() port.ItemReader           { return items{v} }
func (v *view) Members() port.MemberReader       { return members{v} }
func (v *view) Changes() port.ChangeReader       { return changes{v} }

// RetentionDays は、store.yaml の保持日数。ない(0 以下)ときは既定の 30。
func (v *view) RetentionDays() (int, error) {
	y, err := readStoreYAML(v.root)
	if err != nil {
		return 0, err
	}
	if y.RetentionDays <= 0 {
		return defaultRetentionDays, nil
	}
	return y.RetentionDays, nil
}

// description は、エンティティの説明を返す。ログがなければ port.ErrNotFound。同じ Tx で書いた説明(texts)を含む。
func (v *view) description(ref change.Ref) (string, error) {
	log, err := v.log(ref)
	if err != nil {
		return "", err
	}
	if len(log) == 0 {
		return "", port.ErrNotFound
	}
	if text, ok := v.texts[ref]; ok {
		return text, nil
	}
	return readDescription(v.root, ref)
}

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
		w, err := r.Get(id)
		if errors.Is(err, port.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func (r workspaces) Settings(id workspace.ID) (workspace.Settings, error) {
	return get(r.v, change.Ref{Kind: change.KindWorkspace, ID: id}, workspace.SettingsFromLog)
}

type projects struct{ v *view }

func (r projects) Get(id project.ID) (project.Project, error) {
	return get(r.v, change.Ref{Kind: change.KindProject, ID: id}, project.FromLog)
}

func (r projects) List(ws workspace.ID) ([]project.Project, error) {
	out, err := list(r.v, change.KindProject, r.inWorkspace(ws), func(p project.Project) bool { return p.Place.Workspace == ws })
	slices.SortFunc(out, func(a, b project.Project) int {
		return cmp.Or(strings.Compare(a.Rank, b.Rank), strings.Compare(string(a.ID), string(b.ID)))
	})
	return out, err
}

// inWorkspace は Get と同じだが、この view で ws と別のワークスペースにあると分かっているプロジェクトは、ログから
// 作らずに port.ErrNotFound(list は飛ばす)を返す。作ったプロジェクトのワークスペースは覚える。
func (r projects) inWorkspace(ws workspace.ID) func(project.ID) (project.Project, error) {
	return func(id project.ID) (project.Project, error) {
		if q, ok := r.v.projectPlaces[id]; ok && q != ws {
			return project.Project{}, port.ErrNotFound
		}
		p, err := r.Get(id)
		if err != nil {
			return p, err
		}
		if r.v.projectPlaces == nil {
			r.v.projectPlaces = map[project.ID]workspace.ID{}
		}
		r.v.projectPlaces[id] = p.Place.Workspace
		return p, nil
	}
}

type items struct{ v *view }

func (r items) Get(id item.ID) (item.Item, error) {
	return get(r.v, change.Ref{Kind: change.KindItem, ID: id}, item.FromLog)
}

func (r items) List(p project.ID) ([]item.Item, error) {
	out, err := list(r.v, change.KindItem, r.inProject(p), func(it item.Item) bool { return it.Place.Project == p })
	slices.SortFunc(out, func(a, b item.Item) int { return cmp.Compare(a.Place.Number, b.Place.Number) })
	return out, err
}

// inProject は Get と同じだが、この view で p と別のプロジェクトにあると分かっているアイテムは、ログから
// 作らずに port.ErrNotFound(list は飛ばす)を返す。作ったアイテムのプロジェクトは覚える。
func (r items) inProject(p project.ID) func(item.ID) (item.Item, error) {
	return func(id item.ID) (item.Item, error) {
		if q, ok := r.v.places[id]; ok && q != p {
			return item.Item{}, port.ErrNotFound
		}
		it, err := r.Get(id)
		if err != nil {
			return it, err
		}
		if r.v.places == nil {
			r.v.places = map[item.ID]project.ID{}
		}
		r.v.places[id] = it.Place.Project
		return it, nil
	}
}

// list は、kind のすべてのうち keep に合うものを返す。作成の行がないものは飛ばす。
func list[T any](v *view, kind change.Kind, get func(change.ID) (T, error), keep func(T) bool) ([]T, error) {
	ids, err := v.ids(kind)
	if err != nil {
		return nil, err
	}
	var out []T
	for _, id := range ids {
		x, err := get(id)
		if errors.Is(err, port.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if keep(x) {
			out = append(out, x)
		}
	}
	return out, nil
}

func (r projects) Description(id project.ID) (string, error) {
	return r.v.description(change.Ref{Kind: change.KindProject, ID: id})
}

func (r items) Description(id item.ID) (string, error) {
	return r.v.description(change.Ref{Kind: change.KindItem, ID: id})
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
