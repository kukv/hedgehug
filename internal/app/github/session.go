package github

import (
	"crypto/sha256"
	"sync"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Session は、アプリを開いているあいだだけ持つアカウントの状態。再起動すると消え、最初の接続で照合し直す
// (この小計画の決定 D2-2・D2-3)。トークンは持たず、指紋(SHA-256)だけを持つ。
type Session struct {
	mu      sync.Mutex
	fps     map[account.ID][32]byte // 最後に確かめたトークンの指紋(07「接続のたびの照合」)
	broken  map[account.ID]string   // 認証エラーと、行に出す理由(空なら理由の文言なし)
	refresh sync.Mutex              // 期限付きトークンの取り直しを 1 つずつ行う(決定 D2-6)
}

func NewSession() *Session {
	return &Session{fps: map[account.ID][32]byte{}, broken: map[account.ID]string{}}
}

func fingerprint(token string) [32]byte { return sha256.Sum256([]byte(token)) }

// markOK は、本人を確かめたことを記録し、認証エラーを外す。fp がゼロ値なら指紋は覚えない(デバイスフロー)。
func (s *Session) markOK(id account.ID, fp [32]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.broken, id)
	if fp != ([32]byte{}) {
		s.fps[id] = fp
	}
}

func (s *Session) markBroken(id account.ID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.broken[id] = reason
	delete(s.fps, id)
}

// forget は、削除したアカウントの状態を捨てる。
func (s *Session) forget(id account.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.fps, id)
	delete(s.broken, id)
}

func (s *Session) state(id account.ID) (broken bool, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reason, broken = s.broken[id]
	return broken, reason
}

func (s *Session) checked(id account.ID, fp [32]byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	got, ok := s.fps[id]
	return ok && got == fp
}
