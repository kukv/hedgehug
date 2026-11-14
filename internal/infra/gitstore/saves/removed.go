package saves

import (
	"cmp"
	"errors"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// BotName は、人でない実行者の保存と、合わせの作者の名前(gitstore の保存もこれを使う)。
const BotName = change.BotLogin

// Pending は、判定の差し込み口に渡すもの(決定 C4b2-2)を古い順に返す。辿り方と並べ方は Unjudged と同じ。
// (a) remote の先端 theirs から辿れる、判定していない保存(Hedgehug-Op を持つもの)。
// (b) 自分の先端 head から辿れて theirs から辿れない、判定していない同期の修復の保存。ほかのこのマシンの保存は返さない。
// (c) theirs・head から辿れる、判定していない合わせのうち、消えたエンティティに共通の祖先にない行があるものと、
// 親と一緒の削除と人の変更が重なったエンティティがあるもの(決定 C4b2-17)と、説明の同じ箇所が重なったもの(決定 C4c1-12)。
func Pending(repo *git.Repository, theirs, head plumbing.Hash, known func(port.SaveKey) (port.Match, error)) ([]port.ReceivedSave, error) {
	received, err := walk(repo, theirs, known)
	if err != nil {
		return nil, err
	}
	all, err := walk(repo, head, known)
	if err != nil {
		return nil, err
	}
	maps.Copy(all, received)
	var out []port.ReceivedSave
	for _, n := range ordered(all) {
		if n.match != port.NotJudged {
			continue
		}
		_, ok := received[n.c.Hash]
		s, pass, err := pendingOf(n, ok)
		if err != nil {
			return nil, err
		}
		if pass {
			out = append(out, s)
		}
	}
	return out, nil
}

// pendingOf は、n を渡すなら、その中身と true。received は theirs から辿れるか。
func pendingOf(n node, received bool) (port.ReceivedSave, bool, error) {
	if n.key.Op != "" {
		s, err := saveOf(n)
		return s, err == nil && (received || s.Repair), err
	}
	if n.c.NumParents() != 2 {
		return port.ReceivedSave{}, false, nil // 最初の保存
	}
	base := lazyBase(n.c) // 2 つの判定で同じ共通の祖先を使い、探すのも 1 回にする
	removed, concurrent, err := mergeRemoved(n.c, base)
	if err != nil {
		return port.ReceivedSave{}, false, err
	}
	conflicts, err := mergeConflicts(n.c, base)
	if err != nil || (len(concurrent) == 0 && len(conflicts) == 0 && !slices.ContainsFunc(removed, func(e port.RemovedEntity) bool { return len(e.New) > 0 })) {
		return port.ReceivedSave{}, false, err
	}
	return port.ReceivedSave{Key: n.key, Merge: true, Removed: removed, Concurrent: concurrent, Conflicts: conflicts}, true, nil
}

// saveOf は、Hedgehug-Op を持つ保存 1 件の中身。同期の修復の保存なら、消えたエンティティと Sides も読む。
func saveOf(n node) (port.ReceivedSave, error) {
	es, err := entities(n.c)
	if err != nil {
		return port.ReceivedSave{}, err
	}
	s := port.ReceivedSave{Key: n.key, Entities: es, Repair: isRepair(n.c, es)}
	if !s.Repair || n.c.NumParents() == 0 {
		return s, nil
	}
	p, err := n.c.Parent(0)
	if err != nil {
		return port.ReceivedSave{}, err
	}
	if s.Removed, err = removedFrom(p, n.c, nil); err != nil {
		return port.ReceivedSave{}, err
	}
	if len(es) > 0 {
		s.Sides, err = sidesOf(n.c, es)
		return s, err
	}
	if s.Repair, err = underGone(p, s.Removed); !s.Repair {
		s.Removed = nil
	}
	return s, err
}

// isRepair は、同期の修復の保存の形か。作者が bot で、足した行がすべて実行者 sync か、行がない(決定 C4b2-2)。
// 行がない保存は、saveOf が消えたものの形(underGone)でも確かめる。
func isRepair(c *object.Commit, es []port.EntityRows) bool {
	if c.Author.Name != BotName {
		return false
	}
	for _, e := range es {
		for _, r := range e.Rows {
			if r.Actor.Kind != change.ActorSync {
				return false
			}
		}
	}
	return true
}

// mergeRemoved は、合わせ c の 2 つの親のそれぞれについて、親にあって c にないエンティティ。New は、その親の
// ログの行のうち、共通の祖先(1 つ目。同期の合わせと同じ)の同じエンティティのログにない行。あわせて、同じ共通の
// 祖先から、親と一緒の削除と人の変更が重なったエンティティ(concurrentIn)も返す。
func mergeRemoved(c *object.Commit, base baseFunc) ([]port.RemovedEntity, []port.ConcurrentRows, error) {
	ours, err := c.Parent(0)
	if err != nil {
		return nil, nil, err
	}
	theirs, err := c.Parent(1)
	if err != nil {
		return nil, nil, err
	}
	if need, err := needsBase(c); err != nil || !need {
		return nil, nil, err
	}
	baseTree, err := base()
	if err != nil || baseTree == nil {
		return nil, nil, err
	}
	var out []port.RemovedEntity
	for _, p := range []*object.Commit{ours, theirs} {
		es, err := removedFrom(p, c, baseTree)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, es...)
	}
	concurrent, err := concurrentIn(c, ours, theirs, baseTree)
	return out, concurrent, err
}

// needsBase は、合わせ c に、結果から消えた log.jsonl か、両方の親の版と違う log.jsonl があるか。どちらもなければ、
// 消えたものも、同時の削除と変更(両側で変わったログ)もないので、共通の祖先を探さない(長い履歴を辿らないため)。
// 結果にあって親にないログも、その親の版と違うものに数える。
func needsBase(c *object.Commit) (bool, error) {
	merged, err := c.Tree()
	if err != nil {
		return false, err
	}
	seen := map[string]bool{}
	for i := range c.NumParents() {
		p, err := c.Parent(i)
		if err != nil {
			return false, err
		}
		t, err := p.Tree()
		if err != nil {
			return false, err
		}
		if need, err := logsDiffer(t, merged, seen); err != nil || need {
			return need, err
		}
	}
	return false, nil
}

// logsDiffer は、親の版 parent と合わせの結果 merged で違う log.jsonl を seen に足す。結果から消えたログか、
// 前の親でも違ったログ(seen にある)があれば true。
func logsDiffer(parent, merged *object.Tree, seen map[string]bool) (bool, error) {
	changes, err := object.DiffTree(parent, merged)
	if err != nil {
		return false, err
	}
	for _, ch := range changes {
		name := cmp.Or(ch.To.Name, ch.From.Name)
		if _, ok := logRef(name); !ok {
			continue
		}
		if ch.To.Name == "" || seen[name] {
			return true, nil
		}
		seen[name] = true
	}
	return false, nil
}

// removedFrom は、from にあって to にない log.jsonl のエンティティと、from でのログ。base が nil でなければ
// (合わせ)、New に base の同じログにない行を入れる。Ref の順。
func removedFrom(from, to *object.Commit, base *object.Tree) ([]port.RemovedEntity, error) {
	ft, err := from.Tree()
	if err != nil {
		return nil, err
	}
	tt, err := to.Tree()
	if err != nil {
		return nil, err
	}
	changes, err := object.DiffTree(ft, tt)
	if err != nil {
		return nil, err
	}
	var out []port.RemovedEntity
	for _, ch := range changes {
		ref, ok := logRef(ch.From.Name)
		if ch.To.Name != "" || !ok {
			continue
		}
		e, err := removedEntity(ref, ch, base)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b port.RemovedEntity) int { return compareRef(a.Ref, b.Ref) })
	return out, nil
}

// logRef は、エンティティの log.jsonl のパス name のエンティティ。ほかのファイルなら false。
func logRef(name string) (change.Ref, bool) {
	dir, file := path.Split(name)
	ref, ok := RefOf(strings.TrimSuffix(dir, "/"))
	return ref, ok && file == "log.jsonl" && ref.Valid()
}

// compareRef は、Ref の順(種類、ID)。
func compareRef(a, b change.Ref) int {
	return cmp.Or(strings.Compare(string(a.Kind), string(b.Kind)), strings.Compare(string(a.ID), string(b.ID)))
}

// fileIn は、t の name のファイル。t が nil か、ファイルがなければ nil。
func fileIn(t *object.Tree, name string) (*object.File, error) {
	if t == nil {
		return nil, nil
	}
	f, err := t.File(name)
	if errors.Is(err, object.ErrFileNotFound) {
		return nil, nil
	}
	return f, err
}

// removedEntity は、消えた log.jsonl の変更 ch から、消える前のログと、base があれば New を読む。
func removedEntity(ref change.Ref, ch *object.Change, base *object.Tree) (port.RemovedEntity, error) {
	log, _, err := ch.Files()
	if err != nil {
		return port.RemovedEntity{}, err
	}
	b, err := log.Contents()
	if err != nil {
		return port.RemovedEntity{}, err
	}
	e := port.RemovedEntity{Ref: ref, Log: parseLog(b)}
	if base == nil {
		return e, nil
	}
	old, err := fileIn(base, ch.From.Name)
	if err != nil {
		return port.RemovedEntity{}, err
	}
	e.New, err = added(old, log)
	return e, err
}
