// Package noticing は、同期で受け取った保存から、あなた宛ての通知を判定する(06「通知」、決定 C4b1)。
package noticing

import (
	"context"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Service は、受け取った保存の判定。Judge が port.JudgeFunc になる。
type Service struct {
	Inbox    port.Inbox
	Identity port.Identity
	List     port.StoreList
}

// Judge は、saves(古い順)から割り当て・プロジェクトのオーナー・メンションの通知を作る。
// 「あなた」を決められない Store では何も作らない(C4b1-15)。1 回の read の中で前と後を作って通知を決め、
// 書くのは read のあと。判定できない中身(ないエンティティ、読めない行)は飛ばす。エラーを返すのは、このマシンの
// 読み書きの失敗(Store を読めない、受信箱に書けない)のときだけで、次の同期でやり直す(C4b1-5)。
func (s Service) Judge(ctx context.Context, store port.StoreID, saves []port.ReceivedSave, read func(fn func(r port.Reader) error) error) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil || me == "" {
		return nil
	}
	var notices []port.Notice
	if err := read(func(r port.Reader) error {
		b := newBatch(r, saves)
		notices = append(assigns(r, b, store, me), mentions(r, b, store, me)...)
		return nil
	}); err != nil {
		return err
	}
	for _, n := range notices {
		write := s.Inbox.Add
		if n.Kind == port.NoticeMention {
			write = s.Inbox.Mention
		}
		if err := write(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

// batch は、1 回の判定の前と後(C4b1-6)。後はいまのログ、前はいまのログから判定する行を除いたもの。
type batch struct {
	saves  []port.ReceivedSave
	judged map[change.ID]bool
	refs   []change.Ref // saves に出てくるエンティティ(最初に出た順)
	logs   map[change.Ref][]change.Record
}

// newBatch は、saves のエンティティのいまのログを読む。読めないエンティティ(もうない)はログを持たない。
func newBatch(r port.Reader, saves []port.ReceivedSave) batch {
	b := batch{saves: saves, judged: map[change.ID]bool{}, logs: map[change.Ref][]change.Record{}}
	for _, s := range saves {
		for _, e := range s.Entities {
			for _, row := range e.Rows {
				b.judged[row.ID] = true
			}
			if slices.Contains(b.refs, e.Ref) {
				continue
			}
			b.refs = append(b.refs, e.Ref)
			if log, err := r.Changes().Of(e.Ref); err == nil {
				b.logs[e.Ref] = log
			}
		}
	}
	return b
}

// after は、判定の後のログ。
func (b batch) after(ref change.Ref) []change.Record { return b.logs[ref] }

// before は、判定の前のログ(いまのログから判定する行を除いたもの)。
func (b batch) before(ref change.Ref) []change.Record {
	var out []change.Record
	for _, r := range b.logs[ref] {
		if !b.judged[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

// movedIn は、判定する行に moved_in の行があるか(別の Store から移ってきた。C4b1-19)。
func (b batch) movedIn(ref change.Ref) bool {
	return slices.ContainsFunc(b.logs[ref], func(r change.Record) bool { return r.T == change.MovedIn && b.judged[r.ID] })
}

// sender は、行の差出人。自動化の行は空(「hedgehug」。C4b1-8)。
func sender(r change.Record) string {
	if r.Actor.Kind == change.ActorAutomation {
		return ""
	}
	return r.By
}

func same(a, b string) bool { return strings.EqualFold(a, b) }
