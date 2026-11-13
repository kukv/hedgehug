package itemlink_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

var base = time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)

// logOf は、作成の行の後ろに下書きを 1 行ずつ、1 秒ずつずらした時刻で並べたログにする(prev は前の行)。
// 作成の行がないログは、いまの値がないと決まるため(change.Resolve)、作成の行を先頭に置く。
func logOf(ds ...change.Draft) []change.Record {
	var out []change.Record
	ds = append([]change.Draft{change.NewCreate(map[string]any{})}, ds...)
	for i, d := range ds {
		id := change.ID("01TESTREC" + string(rune('A'+i)) + "0000000000000000")
		r := change.Record{
			ID: id, Op: id, At: base.Add(time.Duration(i) * time.Second), By: "taro",
			Actor: change.User("taro"), T: d.T, F: d.F, From: d.From, To: d.To,
		}
		if len(out) > 0 {
			r.Prev = []change.ID{out[len(out)-1].ID}
		}
		out = append(out, r)
	}
	return out
}

var (
	pr41  = itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	pr40  = itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 40}
	iss12 = itemlink.Ref{Kind: itemlink.Issue, Repo: "org/api", Number: 12}
)

func TestRefNames(t *testing.T) {
	if pr41.Key() != "pr:org/web#41" || pr41.Display() != "org/web#41" {
		t.Errorf("Key = %q, Display = %q", pr41.Key(), pr41.Display())
	}
	if itemlink.ValueField(pr41, "state") != "links/pr:org/web#41/state" {
		t.Errorf("ValueField = %q", itemlink.ValueField(pr41, "state"))
	}
	if !pr41.Same(itemlink.Ref{Kind: itemlink.Issue, Repo: "Org/Web", Number: 41}) || pr41.Same(pr40) {
		t.Error("Same must ignore kind and case, and compare numbers")
	}
}

func TestFromLogReadsLinksValuesAndOrder(t *testing.T) {
	log := logOf(
		change.NewAdd("links", iss12),
		change.NewSet(itemlink.ValueField(iss12, "title"), nil, "ログインが落ちる"),
		change.NewSet(itemlink.ValueField(iss12, "state"), nil, "open"),
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "Add login form"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
		change.NewSet(itemlink.ValueField(pr41, "review"), nil, "pending"),
		change.NewSet(itemlink.ValueField(pr41, "state"), "open", "merged"),
		change.NewAdd("links_off", pr40),
	)
	s := itemlink.FromLog(log)
	if len(s.Links) != 2 || s.Links[0].Ref != iss12 || s.Links[1].Ref != pr41 {
		t.Fatalf("Links = %+v, want issue then PR (the order they were linked)", s.Links)
	}
	if got := s.Links[1].Values; got != (itemlink.Values{Title: "Add login form", State: itemlink.Merged, Review: itemlink.Pending}) {
		t.Errorf("PR values = %+v", got)
	}
	if got := s.Links[0].Values; got != (itemlink.Values{Title: "ログインが落ちる", State: itemlink.Open, Review: itemlink.NoReview}) {
		t.Errorf("issue values = %+v (an issue has no review)", got)
	}
	if len(s.Off) != 1 || s.Off[0] != pr40 {
		t.Errorf("Off = %+v", s.Off)
	}
	if l, ok := s.Find(itemlink.Ref{Kind: itemlink.Issue, Repo: "ORG/WEB", Number: 41}); !ok || l.Ref != pr41 {
		t.Errorf("Find = %+v, %v", l, ok)
	}
	if l, ok := s.HasIssue(); !ok || l.Ref != iss12 {
		t.Errorf("HasIssue = %+v, %v", l, ok)
	}
}

func TestFromLogDropsRemovedAndOrdersRelinkLast(t *testing.T) {
	log := logOf(
		change.NewAdd("links", pr41),
		change.NewAdd("links", pr40),
		change.NewRemove("links", pr41),
		change.NewAdd("links", pr41),
	)
	s := itemlink.FromLog(log)
	if len(s.Links) != 2 || s.Links[0].Ref != pr40 || s.Links[1].Ref != pr41 {
		t.Errorf("Links = %+v, want 40 then the relinked 41", s.Links)
	}
	if _, ok := itemlink.FromLog(logOf(change.NewAdd("links", pr41), change.NewRemove("links", pr41))).Find(pr41); ok {
		t.Error("a removed link is still found")
	}
}
