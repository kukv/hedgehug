package noticing

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/member"
)

// membersReader は、メンバーの一覧の読み取りが err を返す Reader。Judge の中では、メンションの判定が先に
// メンバーを読むので、nameOf の読み取りの失敗はここで確かめる。
type membersReader struct {
	port.Reader
	err error
}

func (m membersReader) Members() port.MemberReader { return membersList(m) }

type membersList membersReader

func (m membersList) List() ([]member.Member, error) { return nil, m.err }

func TestNameOfReadError(t *testing.T) {
	boom := errors.New("boom")
	if _, err := nameOf(membersReader{err: boom}, "hanako"); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
	if got, err := nameOf(membersReader{err: port.ErrNotFound}, "hanako"); err != nil || got != "hanako" {
		t.Errorf("nameOf = %q, %v, want login", got, err)
	}
}
