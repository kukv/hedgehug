package gitstore

import (
	"context"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// unsentFacts は、自分の先端から辿れて remote の先端 theirs から辿れない、親が 2 つのコミット(まだ送っていない
// 合わせ)ごとに、共通の祖先にない行の ID を、1 つ目の親の側(Ours)と 2 つ目の親の側(Theirs)に集める(決定 C3a-4)。
// theirs がゼロなら、自分の先端から辿れるすべてのコミットを見る。まだ送っていない合わせがなければ ok は false。
func (s syncRun) unsentFacts(theirs plumbing.Hash) (repair.Facts, bool, error) {
	head, err := s.repo.Head()
	if err != nil {
		return repair.Facts{}, false, err
	}
	sent := map[plumbing.Hash]bool{}
	if !theirs.IsZero() {
		if err := walk(s.repo, theirs, func(c *object.Commit) { sent[c.Hash] = true }); err != nil {
			return repair.Facts{}, false, err
		}
	}
	var merges []*object.Commit
	err = walk(s.repo, head.Hash(), func(c *object.Commit) {
		if !sent[c.Hash] && c.NumParents() == 2 {
			merges = append(merges, c)
		}
	})
	if err != nil || len(merges) == 0 {
		return repair.Facts{}, false, err
	}
	f := repair.Facts{Ours: map[change.ID]bool{}, Theirs: map[change.ID]bool{}}
	for _, m := range merges {
		if err := addFacts(f, m); err != nil {
			return repair.Facts{}, false, err
		}
	}
	return f, true, nil
}

// addFacts は、合わせのコミット m の、共通の祖先にない行の ID を f に足す。
func addFacts(f repair.Facts, m *object.Commit) error {
	ours, err := m.Parent(0)
	if err != nil {
		return err
	}
	theirs, err := m.Parent(1)
	if err != nil {
		return err
	}
	bases, err := ours.MergeBase(theirs)
	if err != nil {
		return err
	}
	// 共通の祖先が複数あっても、1 つ目だけを使う。merge(integrate)も 1 つ目を祖先にして合わせたので、
	// Facts はその合わせが「共通の祖先にない」とした行と同じになる。
	var base map[change.ID]bool
	if len(bases) > 0 {
		if base, err = logIDs(bases[0]); err != nil {
			return err
		}
	}
	for _, side := range []struct {
		c   *object.Commit
		out map[change.ID]bool
	}{{ours, f.Ours}, {theirs, f.Theirs}} {
		ids, err := logIDs(side.c)
		if err != nil {
			return err
		}
		for id := range ids {
			if !base[id] {
				side.out[id] = true
			}
		}
	}
	return nil
}

// logIDs は、コミットのすべての log.jsonl の行の ID。
func logIDs(c *object.Commit) (map[change.ID]bool, error) {
	files, err := c.Files()
	if err != nil {
		return nil, err
	}
	out := map[change.ID]bool{}
	err = files.ForEach(func(f *object.File) error {
		if !strings.HasSuffix(f.Name, "log.jsonl") {
			return nil
		}
		b, err := f.Contents()
		if err != nil {
			return err
		}
		for _, r := range parseLog([]byte(b)) {
			out[r.ID] = true
		}
		return nil
	})
	return out, err
}

// repairUnsent は、修復(Replicator.Repair)があり、まだ送っていない合わせがあれば、修復する。
// 修復がなければ、修復の材料(unsentFacts)も作らない。
func (s syncRun) repairUnsent(ctx context.Context, theirs plumbing.Hash) error {
	if s.r.Repair == nil {
		return nil
	}
	f, ok, err := s.unsentFacts(theirs)
	if err != nil || !ok {
		return err
	}
	return s.repair(ctx, f)
}

// repair は、修復(Replicator.Repair)を、持っている Store のロックの中で呼ぶ。
func (s syncRun) repair(ctx context.Context, f repair.Facts) error {
	return s.r.Repair(ctx, repairWriter{stores: s.r.stores, root: s.root, facts: f})
}

// repairWriter は port.RepairWriter。Sync がロックを持っているので、ロックを取り直さずに読み書きする。
type repairWriter struct {
	stores *Stores
	root   string
	facts  repair.Facts
}

func (w repairWriter) Facts() repair.Facts { return w.facts }

func (w repairWriter) Read(fn func(r port.Reader) error) error { return fn(&view{root: w.root}) }

func (w repairWriter) Write(op port.Operation, fn func(tx port.Tx) error) error {
	return w.stores.runTx(w.root, op, fn)
}
