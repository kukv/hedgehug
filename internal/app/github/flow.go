package github

import (
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// flow はサインインの途中の流れ。addFlow のあとは interval と granted だけが変わり、Session.mu の中で読み書きする。
type flow struct {
	target     account.Account // 登録・保存する値(登録では ID が空)
	previous   account.Account // 編集・再認証の前の値
	clientID   string
	deviceCode string
	expiresAt  time.Time
	interval   time.Duration
	granted    *account.DeviceToken // 許可されたが、持ち主をまだ確かめられていないトークン
}

func (s *Session) addFlow(id string, f *flow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flows[id] = f
}

func (s *Session) lookupFlow(id string) (*flow, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.flows[id]
	return f, ok
}

// slowDown は、流れの間隔を延ばして返す。
func (s *Session) slowDown(f *flow, step time.Duration) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	f.interval += step
	return f.interval
}

func (s *Session) intervalOf(f *flow) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return f.interval
}

// holdGrant は、許可されたトークンを、持ち主を確かめられるまで流れに持たせる。
func (s *Session) holdGrant(f *flow, tok account.DeviceToken) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f.granted = &tok
}

func (s *Session) grantOf(f *flow) (account.DeviceToken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.granted == nil {
		return account.DeviceToken{}, false
	}
	return *f.granted, true
}

// takeFlow は、流れを取り出して消す。取り出すのと消すのをロックの中で一度に行い、キャンセルと重ならないようにする。
func (s *Session) takeFlow(id string) (*flow, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.flows[id]
	delete(s.flows, id)
	return f, ok
}

func (s *Session) endFlow(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.flows, id)
}

// dropExpired は、期限を過ぎた流れを、許可されたトークンごと捨てる(閉じたダイアログの流れを残し続けないため)。
func (s *Session) dropExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, f := range s.flows {
		if !now.Before(f.expiresAt) {
			delete(s.flows, id)
		}
	}
}
