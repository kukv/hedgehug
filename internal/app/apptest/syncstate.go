package apptest

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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

// JudgedSaves は port.JudgedSaves の偽物。
type JudgedSaves struct {
	mu    sync.Mutex
	saves map[port.StoreID][]port.SaveKey
}

func NewJudgedSaves() *JudgedSaves { return &JudgedSaves{saves: map[port.StoreID][]port.SaveKey{}} }

func (j *JudgedSaves) Started(_ context.Context, id port.StoreID) (bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.saves[id]) > 0, nil
}

func (j *JudgedSaves) Known(_ context.Context, id port.StoreID, k port.SaveKey) (port.Match, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	m := port.NotJudged
	for _, s := range j.saves[id] {
		if s.Version == k.Version {
			return port.SameVersion, nil
		}
		if k.Op != "" && s.Op == k.Op && s.Author == k.Author && s.At.Equal(k.At) {
			m = port.SameSave
		}
	}
	return m, nil
}

func (j *JudgedSaves) Mark(_ context.Context, id port.StoreID, keys []port.SaveKey) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, k := range keys {
		if !slices.ContainsFunc(j.saves[id], func(s port.SaveKey) bool { return s.Version == k.Version }) {
			j.saves[id] = append(j.saves[id], k)
		}
	}
	return nil
}

func (j *JudgedSaves) ForgetStore(_ context.Context, id port.StoreID) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.saves, id)
	return nil
}

// RunJudgedSavesContract は、port.JudgedSaves が満たすべき振る舞いを確かめる。偽物と、本物の実装
// (infra/statedb)の両方のテストから呼ぶ。ケースごとに Store 名を変えて、1 つの記録を使う。
func RunJudgedSavesContract(t *testing.T, newSaves func() port.JudgedSaves) {
	s := newSaves()
	t.Run("started follows marks per store", func(t *testing.T) { judgedStarted(t, s) })
	t.Run("known matches version then save", func(t *testing.T) { judgedKnown(t, s) })
	t.Run("forget only that store", func(t *testing.T) { judgedForget(t, s) })
}

func judgedKey(version, op string) port.SaveKey {
	return port.SaveKey{Version: version, Author: "alice", At: syncT1, Op: change.ID(op)}
}

func judgedStarted(t *testing.T, s port.JudgedSaves) {
	if ok, _ := s.Started(syncCtx, "J1"); ok {
		t.Error("Started before Mark")
	}
	must(t, s.Mark(syncCtx, "J1", []port.SaveKey{judgedKey("v1", "")}))
	if ok, err := s.Started(syncCtx, "J1"); err != nil || !ok {
		t.Errorf("Started after Mark = %v, %v", ok, err)
	}
	if ok, _ := s.Started(syncCtx, "J2"); ok {
		t.Error("Started of another store")
	}
}

func judgedKnown(t *testing.T, s port.JudgedSaves) {
	keys := []port.SaveKey{judgedKey("v1", "op1"), judgedKey("v2", "")}
	must(t, s.Mark(syncCtx, "K1", keys))
	must(t, s.Mark(syncCtx, "K1", keys)) // 同じ鍵をもう一度書いてもエラーにならない
	cases := []struct {
		name  string
		store port.StoreID
		key   port.SaveKey
		want  port.Match
	}{
		{"same version", "K1", judgedKey("v1", "op1"), port.SameVersion},
		{"same version without op", "K1", judgedKey("v2", ""), port.SameVersion},
		{"same op author at, other version", "K1", judgedKey("v9", "op1"), port.SameSave},
		{"other op", "K1", judgedKey("v9", "op2"), port.NotJudged},
		{"other author", "K1", port.SaveKey{Version: "v9", Author: "bob", At: syncT1, Op: "op1"}, port.NotJudged},
		{"other time", "K1", port.SaveKey{Version: "v9", Author: "alice", At: syncT2, Op: "op1"}, port.NotJudged},
		{"empty op matches by version only", "K1", judgedKey("v9", ""), port.NotJudged},
		{"other store", "K2", judgedKey("v1", "op1"), port.NotJudged},
	}
	for _, c := range cases {
		if got, err := s.Known(syncCtx, c.store, c.key); err != nil || got != c.want {
			t.Errorf("%s: Known = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}

func judgedForget(t *testing.T, s port.JudgedSaves) {
	must(t, s.Mark(syncCtx, "F1", []port.SaveKey{judgedKey("v1", "")}))
	must(t, s.Mark(syncCtx, "F2", []port.SaveKey{judgedKey("v1", "")}))
	must(t, s.ForgetStore(syncCtx, "F1"))
	if ok, _ := s.Started(syncCtx, "F1"); ok {
		t.Error("F1 still started")
	}
	if ok, _ := s.Started(syncCtx, "F2"); !ok {
		t.Error("F2 was forgotten")
	}
}

// StoreVersions は port.StoreVersions の偽物。
type StoreVersions struct {
	mu    sync.Mutex
	Value map[port.StoreID]string
}

func (s *StoreVersions) Versions(context.Context) (map[port.StoreID]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.Value), nil
}

// Set は、版を入れ替える。
func (s *StoreVersions) Set(v map[port.StoreID]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Value = v
}

// Events は port.Events の偽物。知らされた出来事を覚える。
type Events struct {
	mu   sync.Mutex
	list []port.Event
}

func (e *Events) Publish(ev port.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, ev)
}

// List は、知らされた出来事を順に返す。
func (e *Events) List() []port.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.list)
}
