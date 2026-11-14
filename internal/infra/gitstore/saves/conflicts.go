package saves

import (
	"path"
	"slices"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/textmerge"
)

const descFile = "description.md"

// mergeConflicts は、合わせ c の 2 つの親で説明(description.md)の同じ箇所が重なったエンティティ(決定 C4c1-12)。
// 共通の祖先は mergeRemoved と同じく 1 つ目。説明は同期と同じ textmerge.Merge で合わせ直し、conflict が真のものを
// 返す。Sides は、親ごとの、共通の祖先のログにない desc の行。Ref の順。説明が親どうしで違わなければ祖先を探さない。
func mergeConflicts(c *object.Commit, base baseFunc) ([]port.DescConflict, error) {
	if base == nil {
		base = lazyBase(c)
	}
	trees, err := parentTrees(c)
	if err != nil {
		return nil, err
	}
	names, err := differingDescs(trees[0], trees[1])
	if err != nil || len(names) == 0 {
		return nil, err
	}
	baseTree, err := base()
	if err != nil || baseTree == nil {
		return nil, err
	}
	merged, err := c.Tree()
	if err != nil {
		return nil, err
	}
	var out []port.DescConflict
	for _, name := range names {
		dc, ok, err := keptConflict(name, merged, baseTree, trees)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, dc)
		}
	}
	slices.SortFunc(out, func(a, b port.DescConflict) int { return compareRef(a.Ref, b.Ref) })
	return out, nil
}

// keptConflict は、説明 name のエンティティが合わせの結果 merged に残っていれば、descConflict。
// 同期(gitstore の mergeTrees)は、祖先にあって片側で消えたエンティティ(removedEntities)と、別の Store へ
// 移ったものの下のエンティティ(underTransferred)を結果に残さない。その規則を写さず、結果にログが
// ないエンティティは重なりに数えない(同期が結果に残したものだけを見る)。
func keptConflict(name string, merged, base *object.Tree, sides []*object.Tree) (port.DescConflict, bool, error) {
	kept, err := fileIn(merged, path.Join(path.Dir(name), "log.jsonl"))
	if err != nil || kept == nil {
		return port.DescConflict{}, false, err
	}
	return descConflict(name, base, sides)
}

// parentTrees は、合わせ c の 2 つの親のツリー。
func parentTrees(c *object.Commit) ([]*object.Tree, error) {
	var out []*object.Tree
	for i := range 2 {
		p, err := c.Parent(i)
		if err != nil {
			return nil, err
		}
		t, err := p.Tree()
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// differingDescs は、a と b で違う(片方にだけあるものも含む)エンティティの description.md のパス。
func differingDescs(a, b *object.Tree) ([]string, error) {
	changes, err := object.DiffTree(a, b)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, ch := range changes {
		for _, name := range []string{ch.From.Name, ch.To.Name} {
			if _, ok := descRef(name); ok && !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	}
	return out, nil
}

// descRef は、エンティティの description.md のパス name のエンティティ。ほかのファイルなら false。
func descRef(name string) (change.Ref, bool) {
	dir, file := path.Split(name)
	if file != descFile {
		return change.Ref{}, false
	}
	return logRef(dir + "log.jsonl")
}

// descConflict は、説明 name を base から合わせ直して重なるなら、その中身。
func descConflict(name string, base *object.Tree, sides []*object.Tree) (port.DescConflict, bool, error) {
	var texts [2]string
	var rows [2][]change.Record
	logName := path.Join(path.Dir(name), "log.jsonl")
	oldText, err := textIn(base, name)
	if err != nil {
		return port.DescConflict{}, false, err
	}
	oldLog, err := fileIn(base, logName)
	if err != nil {
		return port.DescConflict{}, false, err
	}
	for i, t := range sides {
		if texts[i], err = textIn(t, name); err != nil {
			return port.DescConflict{}, false, err
		}
		f, err := fileIn(t, logName)
		if err != nil {
			return port.DescConflict{}, false, err
		}
		all, err := added(oldLog, f)
		if err != nil {
			return port.DescConflict{}, false, err
		}
		rows[i] = slices.DeleteFunc(all, func(r change.Record) bool { return r.T != change.Desc })
	}
	if texts[0] == oldText || texts[1] == oldText || texts[0] == texts[1] {
		return port.DescConflict{}, false, nil
	}
	older, newer := texts[0], texts[1]
	if last(rows[1]).Before(last(rows[0])) || (last(rows[0]).Equal(last(rows[1])) && texts[1] <= texts[0]) {
		older, newer = newer, older // 同期と同じく、保存の古い版を先にする
	}
	if _, conflict := textmerge.Merge(oldText, older, newer); !conflict {
		return port.DescConflict{}, false, nil
	}
	ref, _ := descRef(name)
	return port.DescConflict{Ref: ref, Sides: rows}, true, nil
}

// textIn は、t の name の本文。なければ空(同期の mergeDescription と同じ)。
func textIn(t *object.Tree, name string) (string, error) {
	f, err := fileIn(t, name)
	if err != nil || f == nil {
		return "", err
	}
	return f.Contents()
}

// last は、rows のうち最大の at。
func last(rows []change.Record) time.Time {
	var out time.Time
	for _, r := range rows {
		if r.At.After(out) {
			out = r.At
		}
	}
	return out
}
