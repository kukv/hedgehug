// Package syncing は、Store の同期の usecase(06「同期とは」〜「同期の状態の表示」)。
// パッケージ名は標準ライブラリの sync と重ならないようにした(決定 C2-23)。
package syncing

import (
	"context"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// Service は Store の同期。サーバーに 1 つだけ作り、API と自動の同期(Auto)で共用する。
type Service struct {
	Targets    port.SyncTargets
	Tokens     port.SyncTokens
	Replicator port.Replicator
	State      port.SyncState
	Events     port.Events
	Clock      port.Clock
	// AfterSuccess は、書き込めて、表示中の Store の同期に成功した直後に呼ぶ(06「同期のあとの完全な削除」。中身は
	// トラック G が差し込む。決定 C2-22)。nil なら呼ばない。
	AfterSuccess func(ctx context.Context, store port.StoreID)

	mu      sync.Mutex
	running map[port.StoreID]*call
}

// call は、動いている 1 つの同期。終わると done を閉じる。
type call struct {
	done chan struct{}
	out  Outcome
	err  error
}

// Outcome は、1 つの Store の同期の結果。
type Outcome struct {
	Store          port.StoreID
	Name           string
	Skipped        bool              // remote がないので何もしなかった
	Reason         syncstatus.Reason // 失敗なら理由
	Sent           int
	Received       int
	MirrorFailures int // 送れなかったミラーの数
}

// OK は、同期に成功したか。
func (o Outcome) OK() bool { return !o.Skipped && o.Reason == "" }

// Message は、手動で 1 つの Store を同期したときのトースト(06「同期の結果」「同期のエラー」)。
func (o Outcome) Message() string {
	switch {
	case o.Skipped:
		return syncstatus.NoRemote(o.Name)
	case o.Reason != "":
		return syncstatus.Failed(o.Name, o.Reason)
	}
	return syncstatus.Synced(o.Name, o.Sent, o.Received)
}

// SyncStore は、1 つの Store を同期する(Store メニューと設定の「今すぐ同期」、「Store ごとの同期状態」)。
// 非表示の Store も同期する。Store がなければ port.ErrStoreNotFound。失敗した同期はエラーでなく Outcome で返す。
func (s *Service) SyncStore(ctx context.Context, id port.StoreID) (Outcome, error) {
	targets, err := s.Targets.List(ctx)
	if err != nil {
		return Outcome{}, err
	}
	for _, t := range targets {
		if t.ID == id {
			return s.once(ctx, t)
		}
	}
	return Outcome{}, port.ErrStoreNotFound
}

// once は、t を同期する。同じ Store の同期が動いていれば、新しく始めずに、その結果を待って返す。
// 同期は、始めた要求が切れても最後まで続ける(決定 C2-17)。
func (s *Service) once(ctx context.Context, t port.SyncTarget) (Outcome, error) {
	s.mu.Lock()
	if c, ok := s.running[t.ID]; ok {
		s.mu.Unlock()
		<-c.done
		return c.out, c.err
	}
	if s.running == nil {
		s.running = map[port.StoreID]*call{}
	}
	c := &call{done: make(chan struct{})}
	s.running[t.ID] = c
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, t.ID)
		s.mu.Unlock()
		close(c.done)
	}()
	c.out, c.err = s.run(context.WithoutCancel(ctx), t)
	return c.out, c.err
}

func (s *Service) publish(id port.StoreID) {
	if s.Events != nil {
		s.Events.Publish(port.Event{Kind: port.EventSync, Store: id})
	}
}
