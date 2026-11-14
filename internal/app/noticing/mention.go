package noticing

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/mention"
)

// written は、1 つの保存で書かれた本文 1 つ(説明かコメント 1 件)。row はその本文を保存した行。
type written struct {
	source, body string
	row          change.Record
}

// mentions は、判定する保存を古い順に見て、あなたへのメンションのある本文の通知を作る(C4b1-10)。
// 同じ本文の鍵は受信箱の Mention が書き換えるので、あとの保存の本文が残る。
func mentions(r port.Reader, b batch, store port.StoreID, me string) []port.Notice {
	if !isMember(r, me) {
		return nil
	}
	var out []port.Notice
	for _, s := range b.saves {
		for _, e := range s.Entities {
			out = append(out, mentionsIn(r, b, e, me, s.Key.At)...)
		}
	}
	for i := range out {
		out[i].Store = store
	}
	return out
}

// mentionsIn は、1 つの保存がエンティティ e に書いた本文のうち、あなたへのメンションのあるものの通知。
// 時刻は本文を保存した時刻 at(C4b1-11)。
func mentionsIn(r port.Reader, b batch, e port.EntityRows, me string, at time.Time) []port.Notice {
	if b.movedIn(e.Ref) {
		return nil
	}
	base, ok := target(r, b, e.Ref)
	if !ok {
		return nil
	}
	var out []port.Notice
	for _, w := range texts(e) {
		if !mention.Has(w.body, me) || same(w.row.By, me) {
			continue
		}
		n := base
		n.Kind, n.Text, n.Sender, n.Source, n.At = port.NoticeMention, w.body, sender(w.row), w.source, at
		out = append(out, n)
	}
	return out
}

// isMember は、あなたが Store のメンバーにいるか。読めなければ false。
func isMember(r port.Reader, me string) bool {
	ms, err := r.Members().List()
	return err == nil && slices.ContainsFunc(ms, func(m member.Member) bool { return same(m.Login, me) })
}

// target は、アイテムかプロジェクトを指す通知の下地。ほかの種類と、ないものは false。
func target(r port.Reader, b batch, ref change.Ref) (port.Notice, bool) {
	switch ref.Kind {
	case change.KindItem:
		_, n, ok := itemTarget(r, b, ref)
		return n, ok
	case change.KindProject:
		_, n, ok := projectTarget(r, b, ref)
		return n, ok
	}
	return port.Notice{}, false
}

// texts は、保存 e で書かれた本文。説明は、本文が変わり、作成か desc の行があるとき(作成・複製・テンプレートは
// desc の行を書かない。A4a-4)。コメントは、本文のある comment の行。
func texts(e port.EntityRows) []written {
	var out []written
	if e.Description != nil {
		if row, ok := descRow(e.Rows); ok {
			out = append(out, written{source: string(e.Ref.Kind) + ":" + string(e.Ref.ID) + ":desc", body: *e.Description, row: row})
		}
	}
	for _, row := range e.Rows {
		if row.T != change.Comment {
			continue
		}
		if body, ok := commentBody(row.To); ok {
			out = append(out, written{source: "comment:" + row.F, body: body, row: row})
		}
	}
	return out
}

// descRow は、説明を書いた人の行。作成の行があればそれ、なければ desc の行。
func descRow(rows []change.Record) (change.Record, bool) {
	for _, t := range []change.Type{change.Create, change.Desc} {
		if k := slices.IndexFunc(rows, func(r change.Record) bool { return r.T == t }); k >= 0 {
			return rows[k], true
		}
	}
	return change.Record{}, false
}

// commentBody は、comment の行の to の本文。削除の行と読めない行は false。
func commentBody(to json.RawMessage) (string, bool) {
	var v struct {
		Body *string `json:"body"`
	}
	if change.Absent(to) || json.Unmarshal(to, &v) != nil || v.Body == nil {
		return "", false
	}
	return *v.Body, true
}
