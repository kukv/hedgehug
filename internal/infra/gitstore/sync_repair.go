package gitstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
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

// repair は、持っている Store のロックの中で、(1)(完全に削除されたものの下と、完全に削除された相手への関連)を
// 直してから、修復(Replicator.Repair)を呼ぶ(決定 C3a-5・C3a-17)。
func (s syncRun) repair(ctx context.Context, f repair.Facts) error {
	if err := s.r.stores.repairPurged(s.root); err != nil {
		return err
	}
	return s.r.Repair(ctx, repairWriter{stores: s.r.stores, root: s.root, facts: f})
}

// purgeState は、(1) のために読んだ Store の中身。
type purgeState struct {
	items            []item.Item
	display          map[item.ID]string
	projectWorkspace map[project.ID]workspace.ID
	purged           map[change.ID]bool
}

// repairPurged は、祖先が完全に削除されたアイテムを、内部 ID の順に 1 件ずつ完全に削除し、続けて、
// 相手が完全に削除された関連を、持ち主ごとに remove する(保存形式「同期の修復」の (1)。決定 C3a-17)。
// 削除済みかは見ない。どちらも 1 件を 1 回の保存にする。直すものがなければ何も書かない。
func (s *Stores) repairPurged(root string) error {
	st, err := readPurgeState(root)
	if err != nil {
		return err
	}
	for _, id := range repair.UnderPurged(st.items, st.projectWorkspace, st.purged) {
		display := st.display[id]
		op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: display + " を完全に削除(祖先が完全に削除)"}
		err := s.runTx(root, op, func(ptx port.Tx) error {
			x, ok := ptx.(*tx)
			if !ok {
				return fmt.Errorf("purge needs a gitstore tx, got %T", ptx)
			}
			return x.purge(change.Ref{Kind: change.KindItem, ID: id}, display)
		})
		if err != nil {
			return err
		}
	}
	// いま消したものも purged.jsonl に入ったので、読み直して、その相手への関連も外す。
	if st, err = readPurgeState(root); err != nil {
		return err
	}
	dangling := repair.PurgedRelations(st.items, st.purged)
	for len(dangling) > 0 {
		owner := dangling[0].Owner
		var drafts []change.Draft
		for len(dangling) > 0 && dangling[0].Owner == owner {
			drafts = append(drafts, change.NewRemove("relations", dangling[0].Relation))
			dangling = dangling[1:]
		}
		op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: st.display[owner] + " の関連を解除(相手が完全に削除)"}
		if err := s.runTx(root, op, func(ptx port.Tx) error { return ptx.WriteItems().Append(owner, drafts...) }); err != nil {
			return err
		}
	}
	return nil
}

// readPurgeState は、すべての items/*(プロジェクト・ワークスペースのフォルダがないものも)と、プロジェクトの
// ワークスペースと、purged.jsonl の target を読む。アイテムの表示の ID は、ワークスペースが読めればその接頭辞の
// ID、読めなければ内部 ID(決定 C3a-6)。
func readPurgeState(root string) (purgeState, error) {
	v := &view{root: root}
	st := purgeState{display: map[item.ID]string{}, projectWorkspace: map[project.ID]workspace.ID{}}
	ids, err := v.ids(change.KindItem)
	if err != nil {
		return st, err
	}
	for _, id := range ids {
		it, err := items{v}.Get(id)
		if errors.Is(err, port.ErrNotFound) {
			continue
		}
		if err != nil {
			return st, err
		}
		st.items = append(st.items, it)
		st.display[id] = string(id)
		w, err := workspaces{v}.Get(it.Place.Workspace)
		if err == nil {
			st.display[id] = it.DisplayID(w.Prefix)
		} else if !errors.Is(err, port.ErrNotFound) {
			return st, err
		}
	}
	ps, err := list(v, change.KindProject, projects{v}.Get, func(project.Project) bool { return true })
	if err != nil {
		return st, err
	}
	for _, p := range ps {
		st.projectWorkspace[p.ID] = p.Place.Workspace
	}
	st.purged, err = purgedTargets(root)
	return st, err
}

// purgedTargets は、purged.jsonl の行の target。ファイルがなければ空。形の違う行は読み飛ばす。
func purgedTargets(root string) (map[change.ID]bool, error) {
	out := map[change.ID]bool{}
	b, err := os.ReadFile(filepath.Join(root, "purged.jsonl")) //nolint:gosec // G304: パスは Store の根と決まったファイル名から作る
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, l := range bytes.Split(b, []byte("\n")) {
		var v struct {
			Target change.ID `json:"target"`
		}
		if json.Unmarshal(l, &v) == nil && v.Target != "" {
			out[v.Target] = true
		}
	}
	return out, nil
}

// repairWriter は port.RepairWriter。Sync がロックを持っているので、ロックを取り直さずに読み書きする。
type repairWriter struct {
	stores *Stores
	root   string
	facts  repair.Facts
}

func (w repairWriter) Facts() repair.Facts { return w.facts }

func (w repairWriter) Purged() (map[change.ID]bool, error) { return purgedTargets(w.root) }

func (w repairWriter) Read(fn func(r port.Reader) error) error { return fn(&view{root: w.root}) }

func (w repairWriter) Write(op port.Operation, fn func(tx port.Tx) error) error {
	return w.stores.runTx(w.root, op, fn)
}
