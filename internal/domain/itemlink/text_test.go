package itemlink_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestTexts(t *testing.T) {
	cases := []struct{ got, want string }{
		{itemlink.MsgWrongKind(itemlink.Ref{Repo: "org/web", Number: 3}, itemlink.PR), "org/web#3 は PR ではなく Issue です"},
		{itemlink.MsgWrongKind(itemlink.Ref{Repo: "org/web", Number: 3}, itemlink.Issue), "org/web#3 は Issue ではなく PR です"},
		{itemlink.MsgIssueTaken(itemlink.Ref{Repo: "org/web", Number: 3}, "HH-7"), "org/web#3 は HH-7 にリンクされています"},
		{itemlink.AddSummary("HH-2", pr41, false), "HH-2 に org/web#41 をリンク"},
		{itemlink.AddSummary("HH-2", pr41, true), "HH-2 に org/web#41 をリンク(プロジェクトのリポジトリにも追加)"},
		{itemlink.RemoveSummary("HH-2"), "HH-2 のリンクを解除"},
		{itemlink.StateText(itemlink.Merged), "Merged"},
		{itemlink.ReviewText(itemlink.ChangesRequested), "修正依頼"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%q, want %q", c.got, c.want)
		}
	}
}

func TestRefreshSummary(t *testing.T) {
	old := itemlink.Link{Ref: pr41, Values: itemlink.Values{Title: "A", State: itemlink.Open, Review: itemlink.Pending}}
	cases := []struct {
		name string
		repo string
		v    itemlink.Values
		want string
	}{
		{"state", "org/web", itemlink.Values{Title: "A", State: itemlink.Merged, Review: itemlink.Pending}, "HH-2 の org/web#41 の状態を Merged に変更"},
		{
			"review and title", "org/web",
			itemlink.Values{Title: "B", State: itemlink.Open, Review: itemlink.Approved},
			"HH-2 の org/web#41 のレビューを 承認済み に変更、HH-2 の org/web#41 のタイトルを「B」に変更",
		},
		{
			"rename then state", "org/web2",
			itemlink.Values{Title: "A", State: itemlink.Closed, Review: itemlink.Pending},
			"HH-2 のリンクを org/web#41 から org/web2#41 に更新、HH-2 の org/web2#41 の状態を Closed に変更",
		},
	}
	for _, c := range cases {
		if got := itemlink.RefreshSummary("HH-2", old, c.repo, c.v); got != c.want {
			t.Errorf("%s: %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestRefreshSummaryIsOneLine(t *testing.T) {
	old := itemlink.Link{Ref: pr41, Values: itemlink.Values{Title: "A", State: itemlink.Open, Review: itemlink.Pending}}
	got := itemlink.RefreshSummary("HH-2", old, "org/web", itemlink.Values{Title: "B\nC", State: itemlink.Open, Review: itemlink.Pending})
	if got != "HH-2 の org/web#41 のタイトルを「B C」に変更" {
		t.Errorf("%q", got)
	}
}
