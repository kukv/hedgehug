package repair_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// issueLink は、アイテム id の、リンクした行 row(時刻 t0 の mins 分後)の Issue repo#n のリンクを作る。
func issueLink(id, repo string, n int, row string, mins int) repair.IssueLink {
	return repair.IssueLink{
		Item:    item.ID(id),
		Ref:     itemlink.Ref{Kind: itemlink.Issue, Repo: repo, Number: n},
		Linking: change.Record{ID: change.ID(row), At: t0.Add(time.Duration(mins) * time.Minute)},
	}
}

// assertUnlinks は、外すリンクが「アイテム=リポジトリ#番号」の文字列で want と並びまで同じことを確かめる。
func assertUnlinks(t *testing.T, got []repair.IssueLink, want ...string) {
	t.Helper()
	var have []string
	for _, l := range got {
		have = append(have, string(l.Item)+"="+l.Ref.Display())
	}
	if !slices.Equal(have, want) {
		t.Fatalf("unlinks = %v, want %v", have, want)
	}
}

func TestIssueUnlinksTwoItems(t *testing.T) {
	all := []repair.IssueLink{issueLink("HH-1", "kukv/web", 5, "r1", 0), issueLink("HH-2", "kukv/web", 5, "r2", 1)}
	// HH-2 の行が自分の側 → HH-2 のリンクだけを外す。
	assertUnlinks(t, repair.IssueUnlinks(all, facts([]change.ID{"r2"}, []change.ID{"r1"})), "HH-2=kukv/web#5")
	assertUnlinks(t, repair.IssueUnlinks(all, facts([]change.ID{"r1"}, []change.ID{"r2"})), "HH-1=kukv/web#5")
	// Facts が空 → at があとの方を外す。
	assertUnlinks(t, repair.IssueUnlinks(all, repair.Facts{}), "HH-2=kukv/web#5")
	// 重ならなければ空。
	assertUnlinks(t, repair.IssueUnlinks(all[:1], repair.Facts{}))
}

func TestIssueUnlinksOneItem(t *testing.T) {
	// HH-1 が #5(remote の側)と #9(自分の側)→ #9 だけ。
	all := []repair.IssueLink{issueLink("HH-1", "kukv/web", 5, "r1", 0), issueLink("HH-1", "kukv/web", 9, "r2", 1)}
	assertUnlinks(t, repair.IssueUnlinks(all, facts([]change.ID{"r2"}, []change.ID{"r1"})), "HH-1=kukv/web#9")
}

func TestIssueUnlinksChain(t *testing.T) {
	// HH-1 が X(自分の側)と Y(remote の側、at 1)、HH-2 が X(remote の側、at 2)→ HH-1 の X だけ。
	all := []repair.IssueLink{
		issueLink("HH-1", "kukv/web", 1, "rx1", 0),
		issueLink("HH-1", "kukv/web", 2, "ry1", 1),
		issueLink("HH-2", "kukv/web", 1, "rx2", 2),
	}
	assertUnlinks(t, repair.IssueUnlinks(all, facts([]change.ID{"rx1"}, []change.ID{"ry1", "rx2"})), "HH-1=kukv/web#1")
}

func TestIssueUnlinksSameIgnoresCase(t *testing.T) {
	all := []repair.IssueLink{issueLink("HH-1", "Kukv/Web", 5, "r1", 0), issueLink("HH-2", "kukv/web", 5, "r2", 1)}
	assertUnlinks(t, repair.IssueUnlinks(all, repair.Facts{}), "HH-2=kukv/web#5")
}

func TestIssueUnlinksIgnoresPR(t *testing.T) {
	a, b := issueLink("HH-1", "kukv/web", 7, "r1", 0), issueLink("HH-2", "kukv/web", 7, "r2", 1)
	a.Ref.Kind, b.Ref.Kind = itemlink.PR, itemlink.PR
	assertUnlinks(t, repair.IssueUnlinks([]repair.IssueLink{a, b}, repair.Facts{}))
}

func spelling(login, row string, mins int) repair.Spelling {
	return repair.Spelling{Login: login, Row: change.Record{ID: change.ID(row), At: t0.Add(time.Duration(mins) * time.Minute)}}
}

func assertMerges(t *testing.T, got, want []repair.MemberMerge) {
	t.Helper()
	if !slices.EqualFunc(got, want, func(a, b repair.MemberMerge) bool { return a.Keep == b.Keep && slices.Equal(a.Drop, b.Drop) }) {
		t.Fatalf("merges = %v, want %v", got, want)
	}
}

func TestMemberMerges(t *testing.T) {
	// 「Jiro」(remote の側)と「jiro」(自分の側)→ Jiro を残す。
	two := []repair.Spelling{spelling("Jiro", "r1", 0), spelling("jiro", "r2", 1)}
	assertMerges(t, repair.MemberMerges(two, facts([]change.ID{"r2"}, []change.ID{"r1"})), []repair.MemberMerge{{Keep: "Jiro", Drop: []string{"jiro"}}})
	// 3 つの綴り → Drop が 2 つ(辞書順)。at が最も古い「jIro」を残す。
	three := []repair.Spelling{spelling("JIRO", "r3", 2), spelling("jIro", "r1", 0), spelling("Jiro", "r2", 1)}
	assertMerges(t, repair.MemberMerges(three, repair.Facts{}), []repair.MemberMerge{{Keep: "jIro", Drop: []string{"JIRO", "Jiro"}}})
	// 2 組 → 小文字のログイン名の順。
	pairs := []repair.Spelling{spelling("Taro", "r1", 0), spelling("taro", "r2", 1), spelling("JIRO", "r3", 0), spelling("jiro", "r4", 1)}
	assertMerges(t, repair.MemberMerges(pairs, repair.Facts{}), []repair.MemberMerge{
		{Keep: "JIRO", Drop: []string{"jiro"}}, {Keep: "Taro", Drop: []string{"taro"}},
	})
}

func TestMemberMergesSameSpelling(t *testing.T) {
	assertMerges(t, repair.MemberMerges([]repair.Spelling{spelling("jiro", "r1", 0), spelling("taro", "r2", 1)}, repair.Facts{}), nil)
}
