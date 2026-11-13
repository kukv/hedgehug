// Package itemlink は、アイテムの PR・Issue のリンク(07「PR と Issue のリンク」)を、アイテムのログから読み、
// 書く行を作る。リンクの値は「種別・リポジトリ・番号」(Ref)で、タイトル・状態・レビューは
// set の項目 links/<種別:リポジトリ#番号>/<title|state|review> で持つ(保存形式「変更記録」)。
// 解除した PR の記録は links_off に Ref を持つ。domain/item.Item には足さない(トラック D の決定 6)。
package itemlink

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Kind はリンクの種別。
type Kind string

const (
	PR    Kind = "pr"
	Issue Kind = "issue"
)

// State はリンクの状態。Issue は Open・Closed だけ。
type State string

const (
	Open   State = "open"
	Draft  State = "draft"
	Merged State = "merged"
	Closed State = "closed"
)

// Review は PR のレビューの状態。Issue は NoReview。
type Review string

const (
	NoReview         Review = "none"
	Pending          Review = "pending"
	Approved         Review = "approved"
	ChangesRequested Review = "changes_requested"
)

// Checks は PR のチェックの結果(このマシンだけが持つ。Store に保存しない)。
type Checks string

const (
	NoChecks Checks = ""
	Success  Checks = "success"
	Failure  Checks = "failure"
	Running  Checks = "pending"
)

const (
	// LinksField はリンクの項目、OffField は解除した PR の記録の項目。
	LinksField = "links"
	OffField   = "links_off"
	valuesHead = "links/"
)

// Ref はリンクの値(add・remove の値)。
type Ref struct {
	Kind   Kind   `json:"kind"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

// Key は、set の項目名に使う「種別:リポジトリ#番号」。
func (r Ref) Key() string { return fmt.Sprintf("%s:%s#%d", r.Kind, r.Repo, r.Number) }

// Display は「owner/name#番号」。
func (r Ref) Display() string { return fmt.Sprintf("%s#%d", r.Repo, r.Number) }

// Same は、同じ PR・Issue を指すか。リポジトリの大文字小文字と種別は問わない(07「追加」の
// 「もうリンクしています」。GitHub の PR と Issue は番号を共有する)。
func (r Ref) Same(o Ref) bool { return r.Number == o.Number && strings.EqualFold(r.Repo, o.Repo) }

// ValueField は、リンクの値の項目名(links/<種別:リポジトリ#番号>/<name>)。name は title・state・review。
func ValueField(r Ref, name string) string { return valuesHead + r.Key() + "/" + name }

// Values は、GitHub から読んで Store に保存する値。
type Values struct {
	Title  string
	State  State
	Review Review
}

// Link は、いまのリンク 1 本。
type Link struct {
	Ref
	Values
}

// Set は、アイテムのいまのリンクと、解除した PR の記録。Links は「リンクした順」(D3-2)。
type Set struct {
	Links []Link
	Off   []Ref
	// known は、リンクの値の項目のいまの値(外したリンクのものを含む。付け直したときの from に使う)。
	known map[string]Values
}

// FromLog は、アイテムのログからリンクを読む。
func FromLog(log []change.Record) Set {
	st := change.Resolve(log, change.Options{Multi: func(f string) bool { return f == LinksField || f == OffField }})
	s := Set{known: knownValues(st.Values)}
	for _, raw := range st.Lists[LinksField] {
		if r, ok := refOf(raw); ok {
			s.Links = append(s.Links, Link{Ref: r, Values: s.valuesOf(r)})
		}
	}
	for _, raw := range st.Lists[OffField] {
		if r, ok := refOf(raw); ok {
			s.Off = append(s.Off, r)
		}
	}
	places := placesOf(log)
	slices.SortStableFunc(s.Links, func(a, b Link) int { return places[a.Ref].compare(places[b.Ref]) })
	return s
}

// knownValues は、set の項目 links/<key>/<name> の値を、key ごとにまとめる。
func knownValues(values map[string]json.RawMessage) map[string]Values {
	out := map[string]Values{}
	for f, raw := range values {
		rest, ok := strings.CutPrefix(f, valuesHead)
		if !ok {
			continue
		}
		i := strings.LastIndex(rest, "/")
		if i < 0 {
			continue
		}
		key, name := rest[:i], rest[i+1:]
		var v string
		_ = json.Unmarshal(raw, &v)
		cur := out[key]
		switch name {
		case "title":
			cur.Title = v
		case "state":
			cur.State = knownState(v)
		case "review":
			cur.Review = knownReview(v)
		}
		out[key] = cur
	}
	return out
}

// knownState は、v が状態の値でなければ空。
func knownState(v string) State {
	if s := State(v); slices.Contains([]State{Open, Draft, Merged, Closed}, s) {
		return s
	}
	return ""
}

// knownReview は、v がレビューの状態の値でなければ空(valuesOf が NoReview にする)。
func knownReview(v string) Review {
	if r := Review(v); slices.Contains([]Review{NoReview, Pending, Approved, ChangesRequested}, r) {
		return r
	}
	return ""
}

// valuesOf は、r の値。レビューの値がなければ NoReview。
func (s Set) valuesOf(r Ref) Values {
	v := s.known[r.Key()]
	if v.Review == "" {
		v.Review = NoReview
	}
	return v
}

// Find は、r と同じ PR・Issue(Same)のリンクを探す。
func (s Set) Find(r Ref) (Link, bool) {
	for _, l := range s.Links {
		if l.Same(r) {
			return l, true
		}
	}
	return Link{}, false
}

// HasIssue は、Issue のリンクがあれば、その 1 本を返す。
func (s Set) HasIssue() (Link, bool) {
	for _, l := range s.Links {
		if l.Kind == Issue {
			return l, true
		}
	}
	return Link{}, false
}
