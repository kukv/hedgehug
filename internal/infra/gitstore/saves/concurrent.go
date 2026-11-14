package saves

import (
	"slices"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// concurrentIn は、合わせ c の結果に残るエンティティのうち、片側の base にない行に root: false の削除があり、
// もう片側の base にない行に人の変更(削除・取り消し以外)があるもの(決定 C4b2-17)。読むのは、base から両側で
// 変わったログだけ。Ref の順。
func concurrentIn(c, ours, theirs *object.Commit, base *object.Tree) ([]port.ConcurrentRows, error) {
	sides := make([]*object.Tree, 0, 2)
	changed := make([]map[string]change.Ref, 0, 2)
	for _, p := range []*object.Commit{ours, theirs} {
		t, err := p.Tree()
		if err != nil {
			return nil, err
		}
		logs, err := changedLogs(base, t)
		if err != nil {
			return nil, err
		}
		sides, changed = append(sides, t), append(changed, logs)
	}
	merged, err := c.Tree()
	if err != nil {
		return nil, err
	}
	var names []string
	for name := range changed[0] {
		if _, ok := changed[1][name]; ok {
			names = append(names, name)
		}
	}
	// Ref の順に読み、同じ Ref の中は concurrentOf の向きの順(ours が削除した側、theirs が削除した側)にする。
	slices.SortFunc(names, func(a, b string) int { return compareRef(changed[0][a], changed[0][b]) })
	var out []port.ConcurrentRows
	for _, name := range names {
		rows, err := concurrentOf(changed[0][name], name, merged, base, sides)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

// changedLogs は、from から to で変わったか足されたエンティティの log.jsonl のパスと、そのエンティティ。
func changedLogs(from, to *object.Tree) (map[string]change.Ref, error) {
	changes, err := object.DiffTree(from, to)
	if err != nil {
		return nil, err
	}
	out := map[string]change.Ref{}
	for _, ch := range changes {
		if ref, ok := logRef(ch.To.Name); ok {
			out[ch.To.Name] = ref
		}
	}
	return out, nil
}

// concurrentOf は、両側で変わったログ name について、合わせの結果に残っていれば、片側の削除ともう片側の変更の組。
func concurrentOf(ref change.Ref, name string, merged, base *object.Tree, sides []*object.Tree) ([]port.ConcurrentRows, error) {
	if f, err := fileIn(merged, name); f == nil || err != nil {
		return nil, err
	}
	old, err := fileIn(base, name)
	if err != nil {
		return nil, err
	}
	news := make([][]change.Record, 0, len(sides))
	for _, t := range sides {
		f, err := fileIn(t, name)
		if err != nil {
			return nil, err
		}
		rows, err := added(old, f)
		if err != nil {
			return nil, err
		}
		news = append(news, rows)
	}
	var out []port.ConcurrentRows
	for _, p := range [][2][]change.Record{{news[0], news[1]}, {news[1], news[0]}} {
		deleted := slices.DeleteFunc(slices.Clone(p[0]), func(r change.Record) bool { return !cascadeDelete(r) })
		changed := slices.DeleteFunc(slices.Clone(p[1]), func(r change.Record) bool { return !personChange(r) })
		if len(deleted) > 0 && len(changed) > 0 {
			out = append(out, port.ConcurrentRows{Ref: ref, Deleted: deleted, Changed: changed})
		}
	}
	return out, nil
}

// cascadeDelete は、親と一緒の削除(root: false のエンティティの削除)の行か。
func cascadeDelete(r change.Record) bool {
	return r.T == change.Delete && r.F == change.DeletedField && r.Root != nil && !*r.Root
}

// personChange は、人の変更(実行者が人で、削除・取り消し以外)の行か。
func personChange(r change.Record) bool {
	return r.Actor.Kind == change.ActorUser && r.T != change.Delete && r.T != change.Restore
}
