package repair

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Cascade は、削除済みの親の下に残ったアイテム Child と、削除に使う op(親の削除の op)。
type Cascade struct {
	Child item.ID
	Op    change.ID
}

// ProjectCascade は、削除済みのワークスペースの下に残ったプロジェクト Project と、削除に使う op
// (ワークスペースの削除の op)。
type ProjectCascade struct {
	Project project.ID
	Op      change.ID
}

// ProjectCascades は、削除されていないプロジェクトで、ワークスペースが削除済みのものを、プロジェクトの
// 内部 ID の順で返す(保存形式「同期の修復」の (2)。決定 B5b-6)。その下のアイテムは、プロジェクトを
// 削除済みにしたあとの次の回で、Cascades が見つける。
func ProjectCascades(wss []workspace.Workspace, projects []project.Project) []ProjectCascade {
	dead := map[workspace.ID]*change.Deletion{}
	for _, w := range wss {
		if w.Deleted != nil {
			dead[w.ID] = w.Deleted
		}
	}
	var out []ProjectCascade
	for _, p := range projects {
		if d := dead[p.Place.Workspace]; p.Deleted == nil && d != nil {
			out = append(out, ProjectCascade{Project: p.ID, Op: d.Op})
		}
	}
	slices.SortFunc(out, func(a, b ProjectCascade) int { return strings.Compare(string(a.Project), string(b.Project)) })
	return out
}

// Cascades は、削除されていないアイテムで、親のアイテムが削除済みのもの(op は親の削除の op)と、
// 親が削除済みでなくプロジェクトが削除済みのもの(op はプロジェクトの削除の op)を、内部 ID の順で返す
// (保存形式「同期の修復」の (2)。決定 C3a-10・C3a-12)。親が両方に当たるときは、親のアイテムの op を使う。
// 子の子は、子を削除済みにしたあとの次の回で見つかる。
func Cascades(items []item.Item, projects []project.Project) []Cascade {
	byID := make(map[item.ID]item.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	deadProject := map[project.ID]*change.Deletion{}
	for _, p := range projects {
		if p.Deleted != nil {
			deadProject[p.ID] = p.Deleted
		}
	}
	var out []Cascade
	for _, it := range items {
		if it.Deleted != nil {
			continue
		}
		if d := deletedAbove(it, byID, deadProject); d != nil {
			out = append(out, Cascade{Child: it.ID, Op: d.Op})
		}
	}
	slices.SortFunc(out, func(a, b Cascade) int { return strings.Compare(string(a.Child), string(b.Child)) })
	return out
}

// deletedAbove は、it の親のアイテムの削除、なければ it のプロジェクトの削除を返す。
func deletedAbove(it item.Item, byID map[item.ID]item.Item, deadProject map[project.ID]*change.Deletion) *change.Deletion {
	if it.Place.Parent != nil {
		if p, ok := byID[*it.Place.Parent]; ok && p.Deleted != nil {
			return p.Deleted
		}
	}
	return deadProject[it.Place.Project]
}
