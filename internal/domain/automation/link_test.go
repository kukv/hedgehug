package automation_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func pr(s itemlink.State, r itemlink.Review) itemlink.Link {
	return itemlink.Link{
		Ref:    itemlink.Ref{Kind: itemlink.PR, Repo: "o/r", Number: 1},
		Values: itemlink.Values{State: s, Review: r},
	}
}

func issue(s itemlink.State) itemlink.Link {
	return itemlink.Link{
		Ref:    itemlink.Ref{Kind: itemlink.Issue, Repo: "o/r", Number: 2},
		Values: itemlink.Values{State: s, Review: itemlink.NoReview},
	}
}

func ptr(l itemlink.Link) *itemlink.Link { return &l }

func TestLinkEvents(t *testing.T) {
	const (
		open    = itemlink.Open
		draft   = itemlink.Draft
		merged  = itemlink.Merged
		closed  = itemlink.Closed
		none    = itemlink.NoReview
		pending = itemlink.Pending
	)
	tests := []struct {
		name   string
		before *itemlink.Link
		after  itemlink.Link
		want   []automation.Trigger
	}{
		{"リンク: Open", nil, pr(open, none), []automation.Trigger{automation.TriggerPROpen}},
		{"リンク: Open でレビュー待ち", nil, pr(open, pending), []automation.Trigger{automation.TriggerPROpen, automation.TriggerReview}},
		{"リンク: Draft でレビュー待ち", nil, pr(draft, pending), []automation.Trigger{automation.TriggerReview}},
		{"リンク: Draft", nil, pr(draft, none), nil},
		{"リンク: Merged", nil, pr(merged, pending), []automation.Trigger{automation.TriggerPRMerged}},
		{"リンク: Closed の PR", nil, pr(closed, none), nil},
		{"リンク: Closed の Issue", nil, issue(closed), []automation.Trigger{automation.TriggerIssueClosed}},
		{"リンク: Open の Issue", nil, issue(open), nil},
		{"Draft から Open", ptr(pr(draft, none)), pr(open, none), []automation.Trigger{automation.TriggerPROpen}},
		{"Closed から Open", ptr(pr(closed, none)), pr(open, none), []automation.Trigger{automation.TriggerPROpen}},
		{"Open のまま", ptr(pr(open, none)), pr(open, none), nil},
		{"Open でレビュー待ちになる", ptr(pr(open, none)), pr(open, pending), []automation.Trigger{automation.TriggerReview}},
		{"Draft でレビュー待ちになる", ptr(pr(draft, none)), pr(draft, pending), []automation.Trigger{automation.TriggerReview}},
		{"承認済みから再依頼", ptr(pr(open, itemlink.Approved)), pr(open, pending), nil},
		{"修正依頼から再依頼", ptr(pr(open, itemlink.ChangesRequested)), pr(open, pending), nil},
		{"Merged でレビュー待ちになる", ptr(pr(merged, none)), pr(merged, pending), nil},
		{"Closed になりレビュー待ち", ptr(pr(open, none)), pr(closed, pending), nil},
		{"Merged に変わる", ptr(pr(open, itemlink.Approved)), pr(merged, itemlink.Approved), []automation.Trigger{automation.TriggerPRMerged}},
		{"Merged のまま", ptr(pr(merged, itemlink.Approved)), pr(merged, itemlink.Approved), nil},
		{"Issue が閉じる", ptr(issue(open)), issue(closed), []automation.Trigger{automation.TriggerIssueClosed}},
		{"Issue は閉じたまま", ptr(issue(closed)), issue(closed), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := automation.LinkEvents(tt.before, tt.after)
			if !slices.Equal(got, tt.want) {
				t.Errorf("LinkEvents = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllMerged(t *testing.T) {
	m, o, c := pr(itemlink.Merged, itemlink.NoReview), pr(itemlink.Open, itemlink.NoReview), pr(itemlink.Closed, itemlink.NoReview)
	tests := []struct {
		name  string
		links []itemlink.Link
		want  bool
	}{
		{"なし", nil, false},
		{"Merged 1 件", []itemlink.Link{m}, true},
		{"Merged と Open", []itemlink.Link{m, o}, false},
		{"Merged と Closed", []itemlink.Link{m, c}, true},
		{"Closed だけ", []itemlink.Link{c}, false},
		{"Merged と Open の Issue", []itemlink.Link{m, issue(itemlink.Open)}, true},
		{"Closed の Issue だけ", []itemlink.Link{issue(itemlink.Closed)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := automation.AllMerged(tt.links); got != tt.want {
				t.Errorf("AllMerged = %v, want %v", got, tt.want)
			}
		})
	}
}
