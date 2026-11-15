package activity

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Kind は、種類の絞り込み(06「絞り込み」)。
type Kind string

const (
	KindAll        Kind = "all" // 空も「すべて」
	KindUser       Kind = "user"
	KindSync       Kind = "sync"
	KindAutomation Kind = "automation"
)

// matcher は、q の絞り込みと続きの印に合う行か、を返す関数。読めない値は port.ErrInvalidOperation。
func (q Query) matcher() (func(Row) bool, error) {
	kind, err := kindMatcher(q.Kind)
	if err != nil {
		return nil, err
	}
	if q.Workspace != "" && q.Store == "" {
		return nil, fmt.Errorf("%w: workspace %s needs its store", port.ErrInvalidOperation, q.Workspace)
	}
	after, err := cursorMatcher(q.Before)
	if err != nil {
		return nil, err
	}
	return func(r Row) bool { return after(r) && kind(r.Actor) && q.inPlace(r) }, nil
}

// kindMatcher は、種類の絞り込み(C4d-6)。実行者のない行は「すべて」にだけ出る。
func kindMatcher(k Kind) (func(ActorKind) bool, error) {
	switch k {
	case "", KindAll:
		return func(ActorKind) bool { return true }, nil
	case KindUser:
		return func(a ActorKind) bool { return a == ActorUser }, nil
	case KindSync:
		return func(a ActorKind) bool { return a == ActorSync }, nil // 同期の行と、同期での修復
	case KindAutomation:
		return func(a ActorKind) bool { return a == ActorAutomation || a == ActorGitHub }, nil
	}
	return nil, fmt.Errorf("%w: kind %q", port.ErrInvalidOperation, k)
}

// inPlace は、ワークスペースの絞り込み(C4d-9): 選んだワークスペースを含む行、同じ Store の同期の行、同じ Store の
// 「Store 全体」の行。Store だけなら、その Store の行。
func (q Query) inPlace(r Row) bool {
	switch {
	case q.Store == "":
		return true
	case r.Store.ID != q.Store:
		return false
	case q.Workspace == "":
		return true
	}
	return strings.HasPrefix(r.Key, syncKeyPrefix) || r.Storewide || slices.Contains(r.Workspaces, q.Workspace)
}

// cursorMatcher は、続きの印 before(前のページの最後の行の「{時刻(RFC 3339、ミリ秒まで)}|{鍵}」)より後ろに
// 並ぶ行か、を返す(C4d-14)。before が空なら、すべての行。
func cursorMatcher(before string) (func(Row) bool, error) {
	if before == "" {
		return func(Row) bool { return true }, nil
	}
	text, key, ok := strings.Cut(before, "|")
	at, err := time.Parse(time.RFC3339Nano, text)
	if !ok || err != nil || key == "" {
		return nil, fmt.Errorf("%w: cursor %q", port.ErrInvalidOperation, before)
	}
	return func(r Row) bool { return r.At.Before(at) || r.At.Equal(at) && r.Key < key }, nil
}

// Cursor は、r の続きの印(次のページの Before に渡す「{時刻(RFC 3339、ミリ秒まで)}|{鍵}」)。cursorMatcher が読む形。
func Cursor(r Row) string {
	return r.At.UTC().Format("2006-01-02T15:04:05.000Z07:00") + "|" + r.Key
}
