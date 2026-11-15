package apptest

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// CommitLog は、port.Stores を包み、Write 1 回を保存のコミット 1 つとして覚える偽物(port.StoreCommits。C4d-17)。
// Write の前後で、members・ワークスペース・プロジェクト・アイテムの行の ID を比べ、足された行・List で辿れなくなった
// エンティティ・保持日数の変化を、コミットにする。どれも変わらない Write は、本物と同じく記録しない。
type CommitLog struct {
	port.Stores
	Clock port.Clock

	mu     sync.Mutex // stores と seq だけを守る
	stores map[port.StoreID]*commitStore
	seq    int
}

// commitStore は、Store 1 つのコミット。mu は、Write の前後の比べ方がずれないよう、Write 1 回のあいだ持つ。
type commitStore struct {
	mu      sync.Mutex
	commits []port.Commit
}

func NewCommitLog(s port.Stores, c port.Clock) *CommitLog {
	return &CommitLog{Stores: s, Clock: c, stores: map[port.StoreID]*commitStore{}}
}

func (c *CommitLog) of(id port.StoreID) *commitStore {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stores[id] == nil {
		c.stores[id] = &commitStore{}
	}
	return c.stores[id]
}

// Commits は port.StoreCommits。
func (c *CommitLog) Commits(ctx context.Context, id port.StoreID) ([]port.Commit, error) {
	if err := c.Read(ctx, id, func(port.Reader) error { return nil }); err != nil {
		return nil, err
	}
	st := c.of(id)
	st.mu.Lock()
	defer st.mu.Unlock()
	out := slices.Clone(st.commits)
	slices.SortFunc(out, func(a, b port.Commit) int {
		return cmp.Or(b.At.Compare(a.At), cmp.Compare(a.SHA, b.SHA))
	})
	return out, nil
}

// Write は、内側の Write の前後を比べ、変わっていれば 1 件記録する。
func (c *CommitLog) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	st := c.of(id)
	st.mu.Lock()
	defer st.mu.Unlock()
	before, err := snapshotOf(ctx, c.Stores, id)
	if err != nil {
		return err
	}
	if err := c.Stores.Write(ctx, id, op, fn); err != nil {
		return err
	}
	after, err := snapshotOf(ctx, c.Stores, id)
	if err != nil {
		return err
	}
	if commit, ok := c.diff(op, before, after); ok {
		st.commits = append(st.commits, commit)
	}
	return nil
}

// snapshot は、ある時点の Store の、辿れるエンティティの行・転送記録・保持日数。
type snapshot struct {
	rows      map[change.Ref][]change.Record
	transfers []transfer.Record
	retention int
}

func snapshotOf(ctx context.Context, s port.Stores, id port.StoreID) (snapshot, error) {
	snap := snapshot{rows: map[change.Ref][]change.Record{}}
	err := s.Read(ctx, id, func(r port.Reader) error {
		refs, err := reachableRefs(r)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			log, err := r.Changes().Of(ref)
			if err != nil && !errors.Is(err, port.ErrNotFound) {
				return err
			}
			snap.rows[ref] = log
		}
		if snap.transfers, err = r.Transfers().List(); err != nil {
			return err
		}
		snap.retention, err = r.RetentionDays()
		return err
	})
	return snap, err
}

// reachableRefs は、members と、List で辿れるワークスペース・プロジェクト・アイテムの Ref。
func reachableRefs(r port.Reader) ([]change.Ref, error) {
	refs := []change.Ref{{Kind: change.KindMembers}}
	wss, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	for _, w := range wss {
		refs = append(refs, change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		ps, err := r.Projects().List(w.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			refs = append(refs, change.Ref{Kind: change.KindProject, ID: p.ID})
			its, err := r.Items().List(p.ID)
			if err != nil {
				return nil, err
			}
			for _, it := range its {
				refs = append(refs, change.Ref{Kind: change.KindItem, ID: it.ID})
			}
		}
	}
	return refs, nil
}

// diff は、before から after で変わったものをコミットにする。何も変わっていなければ false。
func (c *CommitLog) diff(op port.Operation, before, after snapshot) (port.Commit, bool) {
	commit := port.Commit{Summary: op.Summary, StoreLog: before.retention != after.retention}
	commit.Changes = addedRows(before, after)
	for ref := range before.rows {
		if _, ok := after.rows[ref]; !ok {
			commit.Removed = append(commit.Removed, ref)
		}
	}
	newTransfers := after.transfers[len(before.transfers):]
	if len(commit.Changes)+len(commit.Removed)+len(newTransfers) == 0 && !commit.StoreLog {
		return commit, false
	}
	byRef := func(a, b change.Ref) int { return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.ID, b.ID)) }
	slices.SortFunc(commit.Changes, func(a, b port.EntityRows) int { return byRef(a.Ref, b.Ref) })
	slices.SortFunc(commit.Removed, byRef)
	var added []change.Record
	for _, e := range commit.Changes {
		added = append(added, e.Rows...)
	}
	c.fill(&commit, op, added, newTransfers)
	return commit, true
}

// addedRows は、after にだけある行を、エンティティごとに返す。
func addedRows(before, after snapshot) []port.EntityRows {
	var out []port.EntityRows
	for ref, rows := range after.rows {
		known := map[change.ID]bool{}
		for _, r := range before.rows[ref] {
			known[r.ID] = true
		}
		var fresh []change.Record
		for _, r := range rows {
			if !known[r.ID] {
				fresh = append(fresh, r)
			}
		}
		if len(fresh) > 0 {
			out = append(out, port.EntityRows{Ref: ref, Rows: fresh})
		}
	}
	return out
}

// fill は、コミットの SHA・作者・時刻・op を決める。
func (c *CommitLog) fill(commit *port.Commit, op port.Operation, added []change.Record, newTransfers []transfer.Record) {
	c.mu.Lock()
	c.seq++
	commit.SHA = fmt.Sprintf("%040x", c.seq)
	c.mu.Unlock()
	commit.Author = change.BotLogin
	if op.Actor.Kind == change.ActorUser {
		commit.Author = op.Actor.Login
	}
	for _, r := range added {
		if commit.At.IsZero() || r.At.Before(commit.At) {
			commit.At = r.At
		}
		if commit.Op == "" && r.Actor == op.Actor {
			commit.Op = r.Op
		}
	}
	if commit.Op == "" && len(newTransfers) > 0 {
		commit.Op = newTransfers[0].Op
	}
	if commit.At.IsZero() {
		commit.At = c.Clock.Now()
	}
}
