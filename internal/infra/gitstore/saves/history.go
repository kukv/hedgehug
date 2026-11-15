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
// 含めない)を、作者の時刻の新しい順(同じなら SHA の辞書順)で返す(決定 C4d-2)。seen にあるコミットは
// 差分を取らずにその値を使い、新しく読んだものは seen に足す。コミットは書き換わらないので、SHA で覚えてよい。
func History(repo *git.Repository, from plumbing.Hash, seen map[plumbing.Hash]port.Commit) ([]port.Commit, error) {
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
		if c.NumParents() != 1 || KeyOf(c).Op == "" {
			return nil
		}
		pc, ok := seen[c.Hash]
		if !ok {
			var err error
			if pc, err = commitOf(c); err != nil {
				return err
			}
			seen[c.Hash] = pc
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

// commitOf は、保存のコミット c の、第 1 親との差分を port.Commit にする。
func commitOf(c *object.Commit) (port.Commit, error) {
	rows, err := entities(c)
	if err != nil {
		return port.Commit{}, err
	}
	removed, storeLog, err := removedOf(c)
	if err != nil {
		return port.Commit{}, err
	}
	k := KeyOf(c)
	summary, _, _ := strings.Cut(c.Message, "\n")
	return port.Commit{
		SHA: k.Version, Summary: summary, Op: k.Op, Author: k.Author, At: atOf(k, rows),
		Changes: rows, Removed: removed, StoreLog: storeLog,
	}, nil
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

// removedOf は、c が第 1 親から、フォルダごと消したエンティティ(log.jsonl がなくなったもの。Ref の順)と、
// Store 全体の記録を変えたかを返す。
func removedOf(c *object.Commit) (removed []change.Ref, storeLog bool, err error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, false, err
	}
	p, err := c.Parent(0)
	if err != nil {
		return nil, false, err
	}
	parent, err := p.Tree()
	if err != nil {
		return nil, false, err
	}
	changes, err := object.DiffTree(parent, tree)
	if err != nil {
		return nil, false, err
	}
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
	return removed, storeLog, nil
}
