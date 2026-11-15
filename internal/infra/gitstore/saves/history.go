package saves

import (
	"cmp"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// storeLogPath は、Store 全体の記録(保持日数の変更など)のファイル。
const storeLogPath = ".hedgehug/log.jsonl"

// History は、from から辿れる保存のコミット(親が 1 つで Hedgehug-Op を持つもの。マージと最初のコミットは
// 含めない)を、保存の時刻(port.Commit.At。atOf)の新しい順(同じなら SHA の辞書順)で返す(決定 C4d-2)。recall が返すコミットは
// 差分を取らずにその値を使い、新しく読んだものは remember で覚えさせる。コミットは書き換わらないので、
// SHA で覚えてよい。
func History(repo *git.Repository, from plumbing.Hash, recall func(plumbing.Hash) (port.Commit, bool), remember func(plumbing.Hash, port.Commit)) ([]port.Commit, error) {
	if from.IsZero() {
		return nil, nil
	}
	iter, err := repo.Log(&git.LogOptions{From: from})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	var out []port.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		k := KeyOf(c)
		if c.NumParents() != 1 || k.Op == "" {
			return nil
		}
		pc, ok := recall(c.Hash)
		if !ok {
			var err error
			if pc, err = commitOf(c, k); err != nil {
				return err
			}
			remember(c.Hash, pc)
		}
		out = append(out, pc)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b port.Commit) int {
		return cmp.Or(b.At.Compare(a.At), strings.Compare(a.SHA, b.SHA))
	})
	return out, nil
}

// commitOf は、保存のコミット c(印 k)の、第 1 親との差分を port.Commit にする。差分は 1 回だけ取る。
func commitOf(c *object.Commit, k port.SaveKey) (port.Commit, error) {
	tree, changes, err := diffOf(c)
	if err != nil {
		return port.Commit{}, err
	}
	rows, err := entitiesOf(tree, changes)
	if err != nil {
		return port.Commit{}, err
	}
	removed, storeLog := removedOf(changes)
	summary, _, _ := strings.Cut(c.Message, "\n")
	return port.Commit{
		SHA: k.Version, Summary: summary, Op: k.Op, Author: k.Author, At: atOf(k, rows),
		Changes: withoutDescriptions(rows), Removed: removed, StoreLog: storeLog,
	}, nil
}

// withoutDescriptions は、rows から説明の全文(Description)を除き、行のないエンティティを落とす。コミットは覚えて
// 使い回すが、アクティビティは説明を使わないので、全文を覚えない。
func withoutDescriptions(rows []port.EntityRows) []port.EntityRows {
	out := rows[:0]
	for _, e := range rows {
		if len(e.Rows) > 0 {
			e.Description = nil
			out = append(out, e)
		}
	}
	return out
}

// diffOf は、c の木と、最初の親(なければ空)との差分。
func diffOf(c *object.Commit) (*object.Tree, object.Changes, error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, nil, err
	}
	var parent *object.Tree
	if c.NumParents() > 0 {
		p, err := c.Parent(0)
		if err != nil {
			return nil, nil, err
		}
		if parent, err = p.Tree(); err != nil {
			return nil, nil, err
		}
	}
	changes, err := object.DiffTree(parent, tree)
	return tree, changes, err
}

// atOf は、保存の時刻。git の作者の時刻は秒までなので、そのコミットの op と同じ op の行があれば、その行の
// At(ミリ秒まで。1 回の保存の行はすべて同じ値)を使う。なければ(修復・のせ直し)作者の時刻。
func atOf(k port.SaveKey, rows []port.EntityRows) time.Time {
	for _, e := range rows {
		for _, r := range e.Rows {
			if r.Op == k.Op {
				return r.At
			}
		}
	}
	return k.At
}

// removedOf は、changes のうち、フォルダごと消したエンティティ(log.jsonl がなくなったもの。Ref の順)と、
// Store 全体の記録を変えたか。
func removedOf(changes object.Changes) (removed []change.Ref, storeLog bool) {
	for _, ch := range changes {
		if ch.To.Name == storeLogPath {
			storeLog = true
		}
		if ch.To.Name != "" || path.Base(ch.From.Name) != "log.jsonl" {
			continue
		}
		if ref, ok := RefOf(path.Dir(ch.From.Name)); ok && ref.Valid() {
			removed = append(removed, ref)
		}
	}
	slices.SortFunc(removed, func(a, b change.Ref) int {
		return cmp.Or(strings.Compare(string(a.Kind), string(b.Kind)), strings.Compare(string(a.ID), string(b.ID)))
	})
	return removed, storeLog
}
