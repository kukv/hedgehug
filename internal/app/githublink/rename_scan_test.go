package githublink_test

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// 問い合わせた名前と答えの名前が違えば(改名・移管)、Store ごとに対応を記録する(D4b 最終レビュー R-1)。
func TestCallRecordsRename(t *testing.T) {
	f := appMissingFixture(t)
	moved := pr41
	moved.Repo = "Org/Site"
	f.gh.Put("org/web", moved)
	f.refresh(t)
	if got := f.scans.renames[linkStore]; !maps.Equal(got, map[string]string{"org/web": "org/site"}) {
		t.Errorf("renames = %v", got)
	}
}

// renamedEpic は、HH-1 に example-org/web#5 を状態 s で置き(off なら解除し)、web#7 の取り直しで
// example-org/web → example-org/site の改名を知る(R-1 の例)。そのあと site に #5 の PR を置く。
func (f *readFixture) renamedEpic(t *testing.T, s itemlink.State, off bool) {
	t.Helper()
	f.linkIn(t, linkStore, linkEpic, readPR("example-org/web", 5, s))
	if off {
		if err := f.links.Remove(t.Context(), linkStore, linkEpic, itemlink.Ref{Kind: itemlink.PR, Repo: "example-org/web", Number: 5}); err != nil {
			t.Fatal(err)
		}
	}
	f.linkIn(t, linkStore, linkEpic, readPR("example-org/web", 7, itemlink.Open))
	f.gh.Put("example-org/web", readPR("example-org/site", 7, itemlink.Open))
	f.read(t, every, false)
	f.gh.PutPull("example-org/site", scanPull(5, s, "hh/WEB-1", time.Date(2026, 11, 13, 9, 30, 0, 0, time.UTC)))
	f.advance(every)
}

// 改名の前に解除した PR・Merged のリンクの PR を、改名後の名前で問い合わせた探索(ReadOnce・ScanRepo)で
// 付け直さない。改名を知らなければ、今までどおり付ける(D4b 最終レビュー R-1)。
func TestScanByNewNameKeepsOldLinks(t *testing.T) {
	cases := []struct {
		name string
		s    itemlink.State
		off  bool
		want []string
	}{
		{"unlinked", itemlink.Open, true, []string{"pr example-org/site#7 PR open"}},
		{"merged", itemlink.Merged, false, []string{"pr example-org/web#5 PR merged", "pr example-org/site#7 PR open"}},
	}
	scans := map[string]func(f *readFixture, t *testing.T){
		"ReadOnce": func(f *readFixture, t *testing.T) { f.read(t, every, false) },
		"ScanRepo": func(f *readFixture, t *testing.T) {
			if err := f.links.ScanRepo(t.Context(), linkStore, "example-org/site"); err != nil {
				t.Fatal(err)
			}
		},
	}
	for _, c := range cases {
		for how, scan := range scans {
			t.Run(c.name+"/"+how, func(t *testing.T) {
				f := newScanRead(t)
				f.renamedEpic(t, c.s, c.off)
				scan(f, t)
				if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, c.want) {
					t.Errorf("rows = %q, want %q", got, c.want)
				}
			})
		}
	}
}

// 同じ読み取りで知った改名も使う: 取り直し(web#7 → site#7)と site の探索が同じ ReadOnce でも、解除した web#5 を
// site#5 として付けない(改名の対応は、読み取りのあとで読む)。
func TestReadOnceUsesRenameLearnedInSamePass(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/site")
	f.linkIn(t, linkStore, linkEpic, readPR("example-org/web", 5, itemlink.Open))
	if err := f.links.Remove(t.Context(), linkStore, linkEpic, itemlink.Ref{Kind: itemlink.PR, Repo: "example-org/web", Number: 5}); err != nil {
		t.Fatal(err)
	}
	f.linkIn(t, linkStore, linkEpic, readPR("example-org/web", 7, itemlink.Open))
	f.gh.Put("example-org/web", readPR("example-org/site", 7, itemlink.Open))
	f.gh.PutPull("example-org/site", scanPull(5, itemlink.Open, "hh/WEB-1", time.Date(2026, 11, 13, 8, 30, 0, 0, time.UTC)))
	f.read(t, every, false)
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"pr example-org/site#7 PR open"}) {
		t.Errorf("rows = %q", got)
	}
}

// 改名を知らないとき(対応がない)は、解除した名前と違う名前の PR として付ける。
func TestScanWithoutRenameLinks(t *testing.T) {
	f := newScanRead(t)
	f.linkIn(t, linkStore, linkEpic, readPR("example-org/web", 5, itemlink.Open))
	if err := f.links.Remove(t.Context(), linkStore, linkEpic, itemlink.Ref{Kind: itemlink.PR, Repo: "example-org/web", Number: 5}); err != nil {
		t.Fatal(err)
	}
	f.gh.PutPull("example-org/site", scanPull(5, itemlink.Open, "hh/WEB-1", time.Date(2026, 11, 13, 9, 30, 0, 0, time.UTC)))
	if err := f.links.ScanRepo(t.Context(), linkStore, "example-org/site"); err != nil {
		t.Fatal(err)
	}
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"pr example-org/site#5 Add login form open"}) {
		t.Errorf("rows = %q", got)
	}
}
