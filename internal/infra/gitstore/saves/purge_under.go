package saves

import (
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// underGone は、行のない bot の保存で消えたエンティティ removed のすべてが、前の版 prev で、祖先(親のアイテム、
// プロジェクト、ワークスペースの順に辿る)のどれかのフォルダがもうないものか。それが修復 (1)(完全に削除された
// ものの下を消す)の形で、祖先と一緒に消した保存は修復でない(決定 C4b2-2)。修復 (1) は内部 ID の順に 1 件ずつ
// 消すので、すぐ上のものがまだあっても、その上がなければよい。
func underGone(prev *object.Commit, removed []port.RemovedEntity) (bool, error) {
	tree, err := prev.Tree()
	if err != nil {
		return false, err
	}
	for _, e := range removed {
		ok, err := ancestorGone(tree, e.Ref, e.Log)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// ancestorGone は、ref(ログ log)の祖先を tree で辿り、フォルダのないものに当たるか。
func ancestorGone(tree *object.Tree, ref change.Ref, log []change.Record) (bool, error) {
	seen := map[change.Ref]bool{ref: true}
	for {
		next, ok := upOf(ref, log)
		if !ok || seen[next] {
			return false, nil
		}
		seen[next] = true
		f, err := fileIn(tree, Dirs[next.Kind]+"/"+string(next.ID)+"/log.jsonl")
		if err != nil || f == nil {
			return err == nil, err
		}
		b, err := f.Contents()
		if err != nil {
			return false, err
		}
		ref, log = next, parseLog(b)
	}
}

// upOf は、アイテム・プロジェクト ref のすぐ上のもの(親のアイテム、プロジェクト、ワークスペース)。
func upOf(ref change.Ref, log []change.Record) (change.Ref, bool) {
	switch ref.Kind {
	case change.KindItem:
		it, ok := item.FromLog(log)
		switch {
		case !ok:
			return change.Ref{}, false
		case it.Place.Parent != nil:
			return change.Ref{Kind: change.KindItem, ID: *it.Place.Parent}, true
		case it.Place.Project != "":
			return change.Ref{Kind: change.KindProject, ID: it.Place.Project}, true
		}
		return change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace}, true
	case change.KindProject:
		if p, ok := project.FromLog(log); ok {
			return change.Ref{Kind: change.KindWorkspace, ID: p.Place.Workspace}, true
		}
	}
	return change.Ref{}, false
}
