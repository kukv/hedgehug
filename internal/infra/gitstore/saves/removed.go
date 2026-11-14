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
// (c) theirs・head から辿れる、判定していない合わせのうち、消えたエンティティに共通の祖先にない行があるもの。
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
	removed, err := mergeRemoved(n.c)
	if err != nil || !slices.ContainsFunc(removed, func(e port.RemovedEntity) bool { return len(e.New) > 0 }) {
		return port.ReceivedSave{}, false, err
	}
	return port.ReceivedSave{Key: n.key, Merge: true, Removed: removed}, true, nil
}

// saveOf は、Hedgehug-Op を持つ保存 1 件の中身。同期の修復の保存なら、消えたエンティティも読む。
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
	s.Removed, err = removedFrom(p, n.c, nil)
	return s, err
}

// isRepair は、同期の修復の保存か。作者が bot で、足した行がすべて実行者 sync か、行がない(決定 C4b2-2)。
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
// ログの行のうち、共通の祖先(1 つ目。同期の合わせと同じ)の同じエンティティのログにない行。
func mergeRemoved(c *object.Commit) ([]port.RemovedEntity, error) {
	ours, err := c.Parent(0)
	if err != nil {
		return nil, err
	}
	theirs, err := c.Parent(1)
	if err != nil {
		return nil, err
	}
	bases, err := ours.MergeBase(theirs)
	if err != nil {
		return nil, err
	}
	if len(bases) == 0 {
		return nil, nil // 同期は、共通の祖先のない合わせを作らない
	}
	base, err := bases[0].Tree()
	if err != nil {
		return nil, err
	}
	var out []port.RemovedEntity
	for _, p := range []*object.Commit{ours, theirs} {
		es, err := removedFrom(p, c, base)
		if err != nil {
			return nil, err
		}
		out = append(out, es...)
	}
	return out, nil
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
		dir, name := path.Split(ch.From.Name)
		ref, ok := RefOf(strings.TrimSuffix(dir, "/"))
		if ch.To.Name != "" || name != "log.jsonl" || !ok || !ref.Valid() {
			continue
		}
		e, err := removedEntity(ref, ch, base)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b port.RemovedEntity) int {
		return cmp.Or(strings.Compare(string(a.Ref.Kind), string(b.Ref.Kind)), strings.Compare(string(a.Ref.ID), string(b.Ref.ID)))
	})
	return out, nil
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
	old, err := base.File(ch.From.Name)
	if errors.Is(err, object.ErrFileNotFound) {
		old, err = nil, nil
	}
	if err != nil {
		return port.RemovedEntity{}, err
	}
	e.New, err = added(old, log)
	return e, err
}
