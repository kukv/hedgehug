package apptest

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// SyncState は port.SyncState の偽物。
type SyncState struct {
	mu      sync.Mutex
	records map[port.StoreID]port.SyncRecord
	seen    map[port.StoreID][2]string // remoteKey, seen
	mirrors map[port.StoreID]map[string]port.MirrorRecord
}

func NewSyncState() *SyncState {
	return &SyncState{
		records: map[port.StoreID]port.SyncRecord{},
		seen:    map[port.StoreID][2]string{},
		mirrors: map[port.StoreID]map[string]port.MirrorRecord{},
	}
}

func (s *SyncState) Records(context.Context) ([]port.SyncRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]port.SyncRecord, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b port.SyncRecord) int { return cmp.Compare(a.Store, b.Store) })
	return out, nil
}

func (s *SyncState) Failed(_ context.Context, id port.StoreID, at time.Time, reason syncstatus.Reason) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.records[id]
	r.Store, r.LastAttempt, r.Reason = id, at, reason
	s.records[id] = r
	return nil
}

func (s *SyncState) Succeeded(_ context.Context, id port.StoreID, at time.Time, remoteKey, seen string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[id] = port.SyncRecord{Store: id, LastAttempt: at, LastSuccess: at}
	s.seen[id] = [2]string{remoteKey, seen}
	return nil
}

func (s *SyncState) Seen(_ context.Context, id port.StoreID, remoteKey string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.seen[id]; ok && v[0] == remoteKey {
		return v[1], nil
	}
	return "", nil
}

func (s *SyncState) Mirrored(_ context.Context, id port.StoreID, mirror string, at time.Time, reason syncstatus.Reason) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mirrors[id] == nil {
		s.mirrors[id] = map[string]port.MirrorRecord{}
	}
	s.mirrors[id][mirror] = port.MirrorRecord{Mirror: mirror, At: at, Reason: reason}
	return nil
}

func (s *SyncState) Mirrors(_ context.Context, id port.StoreID) ([]port.MirrorRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []port.MirrorRecord{}
	for _, m := range s.mirrors[id] {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b port.MirrorRecord) int { return cmp.Compare(a.Mirror, b.Mirror) })
	return out, nil
}

// RunSyncStateContract は、port.SyncState が満たすべき振る舞いを確かめる。偽物と、本物の実装
// (infra/statedb)の両方のテストから呼ぶ。newState は、空の状態を返す。時刻は UTC で比べる。
func RunSyncStateContract(t *testing.T, newState func(t *testing.T) port.SyncState) {
	t.Run("empty at first", func(t *testing.T) { syncStateEmpty(t, newState(t)) })
	t.Run("failure keeps the last success and the seen position", func(t *testing.T) { syncStateFailure(t, newState(t)) })
	t.Run("success clears the reason and remembers the remote", func(t *testing.T) { syncStateSuccess(t, newState(t)) })
	t.Run("mirrors keep the last result per mirror", func(t *testing.T) { syncStateMirrors(t, newState(t)) })
}

var (
	syncT1  = time.Date(2026, 11, 13, 10, 0, 0, 123_000_000, time.UTC)
	syncT2  = syncT1.Add(time.Hour)
	syncCtx = context.Background()
)

func syncStateEmpty(t *testing.T, s port.SyncState) {
	if rs, err := s.Records(syncCtx); err != nil || len(rs) != 0 {
		t.Errorf("Records = %+v, %v", rs, err)
	}
	if seen, err := s.Seen(syncCtx, "S1", "github.com/o/r"); err != nil || seen != "" {
		t.Errorf("Seen = %q, %v", seen, err)
	}
}

func syncStateFailure(t *testing.T, s port.SyncState) {
	must(t, s.Succeeded(syncCtx, "S1", syncT1, "github.com/o/r", "v1"))
	must(t, s.Failed(syncCtx, "S1", syncT2, syncstatus.AuthFailed))
	rs, err := s.Records(syncCtx)
	if err != nil || len(rs) != 1 {
		t.Fatalf("Records = %+v, %v", rs, err)
	}
	r := rs[0]
	if r.Store != "S1" || !r.LastAttempt.Equal(syncT2) || !r.LastSuccess.Equal(syncT1) || r.Reason != syncstatus.AuthFailed {
		t.Errorf("record = %+v", r)
	}
	if seen, _ := s.Seen(syncCtx, "S1", "github.com/o/r"); seen != "v1" {
		t.Errorf("Seen = %q", seen)
	}
}

func syncStateSuccess(t *testing.T, s port.SyncState) {
	must(t, s.Failed(syncCtx, "S2", syncT1, syncstatus.Unreachable))
	must(t, s.Succeeded(syncCtx, "S2", syncT2, "github.com/o/r", "v2"))
	must(t, s.Failed(syncCtx, "S1", syncT1, syncstatus.NotStore))
	rs, _ := s.Records(syncCtx)
	if len(rs) != 2 || rs[0].Store != "S1" || rs[1].Store != "S2" {
		t.Fatalf("Records = %+v", rs)
	}
	if r := rs[1]; r.Reason != "" || !r.LastSuccess.Equal(syncT2) || !r.LastAttempt.Equal(syncT2) {
		t.Errorf("S2 = %+v", r)
	}
	if r := rs[0]; !r.LastSuccess.IsZero() || r.Reason != syncstatus.NotStore {
		t.Errorf("S1 = %+v", r)
	}
	if seen, _ := s.Seen(syncCtx, "S2", "github.com/o/other"); seen != "" {
		t.Errorf("Seen for another remote = %q", seen)
	}
}

func syncStateMirrors(t *testing.T, s port.SyncState) {
	must(t, s.Mirrored(syncCtx, "S1", "https://b.invalid/m", syncT1, syncstatus.MirrorAhead))
	must(t, s.Mirrored(syncCtx, "S1", "https://a.invalid/m", syncT1, ""))
	must(t, s.Mirrored(syncCtx, "S1", "https://b.invalid/m", syncT2, ""))
	ms, err := s.Mirrors(syncCtx, "S1")
	if err != nil || len(ms) != 2 || ms[0].Mirror != "https://a.invalid/m" || ms[1].Reason != "" || !ms[1].At.Equal(syncT2) {
		t.Errorf("Mirrors = %+v, %v", ms, err)
	}
	if ms, _ := s.Mirrors(syncCtx, "S2"); len(ms) != 0 {
		t.Errorf("Mirrors of another store = %+v", ms)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
