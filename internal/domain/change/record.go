// Package change は、変更の記録(log.jsonl の 1 行)と、記録からいまの値を決める規則を持つ。
package change

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// ID は内部 ID(ULID)。
type ID string

var idPattern = regexp.MustCompile(`^[0-9A-Za-z]+$`)

// ValidID は、エンティティの内部 ID に使えるか(空でなく、英数字だけ。Store ではフォルダの名前になる)。
func ValidID(id ID) bool { return idPattern.MatchString(string(id)) }

// Type は変更の種類(保存形式「変更記録」の t)。扱うのは下の種類だけ。
type Type string

const (
	Create Type = "create"
	Set    Type = "set"
	Add    Type = "add"
	Remove Type = "remove"
	// Delete は削除。f は消すもの(値なら "statuses/<ID>"、プロパティなら "properties/<ID>")。
	Delete Type = "delete"
	// Restore は、削除の取り消し。f は削除と同じ f、to は取り消す削除の op(C3a-7)。読む側は、
	// 取り消された delete の行を、なかったものとして扱う。
	Restore Type = "restore"
	// Desc は説明を保存した(本文は description.md。f も from も to も持たない)。
	Desc Type = "desc"
	// Comment・Worklog は、コメントと作業時間の追加・編集・削除。f は内部 ID、削除は to が null。
	Comment Type = "comment"
	Worklog Type = "worklog"
)

// TimeLayout は記録の時刻の書き方(ミリ秒までの RFC 3339、このマシンの時刻帯付き)。
const TimeLayout = "2006-01-02T15:04:05.000Z07:00"

// Record は log.jsonl の 1 行。
type Record struct {
	ID    ID
	Op    ID
	At    time.Time
	By    string
	Actor Actor
	T     Type
	F     string
	From  json.RawMessage
	To    json.RawMessage
	Prev  []ID
	Root  *bool
}

// line は log.jsonl に書く形。キーの並びは保存形式の例と同じ。
type line struct {
	ID    ID              `json:"id"`
	Op    ID              `json:"op"`
	At    string          `json:"at"`
	By    string          `json:"by"`
	Actor string          `json:"actor"`
	T     Type            `json:"t"`
	F     string          `json:"f,omitempty"`
	From  json.RawMessage `json:"from,omitempty"`
	To    json.RawMessage `json:"to,omitempty"`
	Prev  []ID            `json:"prev"`
	Root  *bool           `json:"root,omitempty"`
}

// MarshalJSON は log.jsonl の 1 行にする。prev が空でも [] で書く。
func (r Record) MarshalJSON() ([]byte, error) {
	prev := r.Prev
	if prev == nil {
		prev = []ID{}
	}
	return json.Marshal(line{
		ID: r.ID, Op: r.Op, At: r.At.Format(TimeLayout), By: r.By, Actor: r.Actor.Code(),
		T: r.T, F: r.F, From: r.From, To: r.To, Prev: prev, Root: r.Root,
	})
}

// UnmarshalJSON は log.jsonl の 1 行を読む。
func (r *Record) UnmarshalJSON(b []byte) error {
	var l line
	if err := json.Unmarshal(b, &l); err != nil {
		return fmt.Errorf("record: %w", err)
	}
	at, err := time.Parse(time.RFC3339, l.At)
	if err != nil {
		return fmt.Errorf("record %s: at: %w", l.ID, err)
	}
	actor, err := ParseActor(l.Actor, l.By)
	if err != nil {
		return fmt.Errorf("record %s: %w", l.ID, err)
	}
	*r = Record{
		ID: l.ID, Op: l.Op, At: at, By: l.By, Actor: actor,
		T: l.T, F: l.F, From: l.From, To: l.To, Prev: l.Prev, Root: l.Root,
	}
	return nil
}

// Kind はログを持つエンティティの種類。
type Kind string

const (
	KindMembers   Kind = "members"
	KindWorkspace Kind = "workspace"
	KindProject   Kind = "project"
	KindItem      Kind = "item"
)

// Ref は、どのエンティティのログかを指す。メンバーの一覧は Store に 1 つなので ID は空。
type Ref struct {
	Kind Kind
	ID   ID
}

// Valid は、ログを指せる Ref か。メンバーの一覧は ID が空のときだけ、ほかの種類は ValidID の ID のときだけ。
func (r Ref) Valid() bool {
	switch r.Kind {
	case KindMembers:
		return r.ID == ""
	case KindWorkspace, KindProject, KindItem:
		return ValidID(r.ID)
	}
	return false
}
