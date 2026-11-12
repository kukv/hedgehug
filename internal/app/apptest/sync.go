package apptest

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
)

// ReplicatorCall は、Replicator の偽物が受けた 1 回の呼び出し。
type ReplicatorCall struct {
	Store  port.StoreID
	Remote port.Remote
	Mirror bool // Mirror の呼び出しなら true
}

// Replicator は port.Replicator の偽物。Sync は Results・Errs に置いた値を返す。Gate が nil でなければ、
// Sync はそこから 1 つ受け取るまで待つ(同期が動いているあいだを作るため)。
type Replicator struct {
	mu         sync.Mutex
	Results    map[port.StoreID]port.SyncResult
	Errs       map[port.StoreID]error
	MirrorErrs map[string]error // ミラーの URL → エラー
	UnsentN    map[port.StoreID]int
	UnsentErrs map[port.StoreID]error // Unsent が返すエラー
	Gate       chan struct{}
	calls      []ReplicatorCall
}

func (r *Replicator) Sync(ctx context.Context, id port.StoreID, remote port.Remote) (port.SyncResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, ReplicatorCall{Store: id, Remote: remote})
	gate := r.Gate
	r.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return port.SyncResult{}, ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Errs[id]; err != nil {
		return port.SyncResult{}, err
	}
	return r.Results[id], nil
}

func (r *Replicator) Mirror(_ context.Context, id port.StoreID, mirror port.Remote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, ReplicatorCall{Store: id, Remote: mirror, Mirror: true})
	return r.MirrorErrs[mirror.URL]
}

func (r *Replicator) Unsent(_ context.Context, id port.StoreID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.UnsentErrs[id]; err != nil {
		return 0, err
	}
	return r.UnsentN[id], nil
}

// Calls は、受けた呼び出しを順に返す。
func (r *Replicator) Calls() []ReplicatorCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

// SyncTargets は port.SyncTargets の偽物。
type SyncTargets struct {
	mu      sync.Mutex
	Targets []port.SyncTarget
}

func (s *SyncTargets) List(context.Context) ([]port.SyncTarget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.Targets), nil
}

// Set は、一覧を入れ替える(テストの途中で設定を変えるため)。
func (s *SyncTargets) Set(ts []port.SyncTarget) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Targets = ts
}

// SyncSettings は port.SyncSettings の偽物。
type SyncSettings struct {
	mu    sync.Mutex
	Value port.SyncTiming
}

func (s *SyncSettings) Timing(context.Context) (port.SyncTiming, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Value, nil
}

// SyncTokens は port.SyncTokens の偽物。Errs にあればそのエラーを、なければ Tokens の値を返す。
type SyncTokens struct {
	mu       sync.Mutex
	Tokens   map[port.StoreID]string
	Errs     map[port.StoreID]error
	rejected []port.StoreID
}

func (s *SyncTokens) StoreToken(_ context.Context, id port.StoreID) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Errs[id]; err != nil {
		return "", err
	}
	return s.Tokens[id], nil
}

func (s *SyncTokens) StoreRejected(_ context.Context, id port.StoreID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejected = append(s.rejected, id)
}

// Rejected は、StoreRejected で知らされた Store を順に返す。
func (s *SyncTokens) Rejected() []port.StoreID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.rejected)
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
