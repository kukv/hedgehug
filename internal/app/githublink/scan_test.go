package githublink_test

import (
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/ghread"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// scanWeb は、store の example-org/web を ScanRepo で探す。
func (f *readFixture) scanWeb(t *testing.T, store port.StoreID) {
	t.Helper()
	if err := f.links.ScanRepo(t.Context(), store, "example-org/web"); err != nil {
		t.Fatal(err)
	}
}

func (f *readFixture) repoFailures(t *testing.T) map[string]string {
	t.Helper()
	got, err := f.links.RepoFailures(t.Context(), linkStore, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// リポジトリを足して初めて探したときは、リンクを付けても、ルールは動かさない(D4b-7・9)。
func TestScanRepoFirstNoTrigger(t *testing.T) {
	f := newScanRead(t)
	f.withRules(t, statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone))
	f.addRepo(t, linkStore, "example-org/web")
	at := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Merged, "hh/WEB-1", at))
	f.scanWeb(t, linkStore)
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"pr example-org/web#5 Add login form merged"}) {
		t.Errorf("rows = %q", got)
	}
	if it := f.epicItem(t); it.Status != apptest.SampleStatusTodo {
		t.Errorf("status = %s, want unchanged", it.Status)
	}
	if rec := f.scanRecord(linkStore, "example-org/web"); rec != (port.ScanRecord{Read: true, Since: at}) {
		t.Errorf("record = %+v", rec)
	}
	if n := f.githubEvents(linkStore); n != 1 {
		t.Errorf("events = %d", n)
	}
}

// 間隔の中でも、読まない期限の前でも探し、取り直しの記録は変えない(D4b-9)。
func TestScanRepoIgnoresInterval(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/web")
	f.read(t, every, false)
	rec := f.reads.records[linkStore]["example-org/web"]
	rec.NotBefore = f.clock.now.Add(time.Hour)
	f.reads.records[linkStore]["example-org/web"] = rec
	before := maps.Clone(f.reads.records[linkStore])
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Open, "hh/WEB-1", f.clock.now))
	f.advance(time.Minute)
	f.scanWeb(t, linkStore)
	if n := f.gh.ReadPullsCalls("example-org/web"); n != 2 {
		t.Errorf("ReadPulls calls = %d, want a scan inside the interval", n)
	}
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"pr example-org/web#5 Add login form open"}) {
		t.Errorf("rows = %q", got)
	}
	if !maps.EqualFunc(f.reads.records[linkStore], before, func(a, b ghread.Record) bool { return a == b }) {
		t.Errorf("read records = %+v, want %+v", f.reads.records[linkStore], before)
	}
}

// 探すのは指定の Store だけ。
func TestScanRepoOnlyThatStore(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/web")
	f.addRepo(t, otherStore, "example-org/web")
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Open, "hh/HH-1", f.clock.now))
	f.scanWeb(t, otherStore)
	if rows := f.list(t, otherStore, linkEpic); len(rows) != 1 {
		t.Errorf("other store rows = %q", displays(rows))
	}
	if rows := f.list(t, linkStore, linkEpic); len(rows) != 0 {
		t.Errorf("link store rows = %q", displays(rows))
	}
}

// 閲覧のみ・非表示・アカウントのない Store では探さない(D4b-10)。
func TestScanRepoSkipsReadOnly(t *testing.T) {
	for name, edit := range map[string]func(*port.StoreGitHubInfo){
		"read only":  func(s *port.StoreGitHubInfo) { s.ReadOnly = true },
		"hidden":     func(s *port.StoreGitHubInfo) { s.Hidden = true },
		"no account": func(s *port.StoreGitHubInfo) { s.Account = "" },
	} {
		t.Run(name, func(t *testing.T) {
			f := newScanRead(t)
			f.addRepo(t, linkStore, "example-org/web")
			edit(&f.infos.Stores[0])
			f.scanWeb(t, linkStore)
			if n := f.gh.ReadPullsCalls("example-org/web"); n != 0 {
				t.Errorf("ReadPulls calls = %d", n)
			}
			if _, ok := f.scans.rows[linkStore]; ok {
				t.Errorf("records = %+v", f.scans.rows[linkStore])
			}
		})
	}
}

// 失敗の文言は記録から、アカウントのない Store ではその場で作り、閲覧のみの Store では出さない(D4b-10)。
func TestRepoFailures(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "Example-Org/Web")
	f.addRepo(t, linkStore, "example-org/api")
	const unreadable = "example-org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください"
	f.scans.rows[linkStore] = map[string]port.ScanRecord{"example-org/web": {Failure: unreadable}, "example-org/api": {Read: true}}
	if got := f.repoFailures(t); !maps.Equal(got, map[string]string{"Example-Org/Web": unreadable}) {
		t.Errorf("failures = %q", got)
	}
	f.infos.Stores[0].Account = ""
	want := map[string]string{
		"Example-Org/Web": "Example-Org/Web を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください",
		"example-org/api": "example-org/api を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください",
	}
	if got := f.repoFailures(t); !maps.Equal(got, want) {
		t.Errorf("no account: failures = %q", got)
	}
	f.infos.Stores[0].ReadOnly = true
	if got := f.repoFailures(t); len(got) != 0 {
		t.Errorf("read only: failures = %q", got)
	}
	// GitHub の設定の一覧にない Store は、ScanRepo と同じく何も出さない
	f.infos.Stores = f.infos.Stores[1:]
	if got := f.repoFailures(t); len(got) != 0 {
		t.Errorf("unlisted store: failures = %q", got)
	}
}

func TestRepoFailuresDeletedProject(t *testing.T) {
	f := newScanRead(t)
	if err := apptest.DeleteProject(t.Context(), f.stores.Stores, linkStore, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if _, err := f.links.RepoFailures(t.Context(), linkStore, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// 改名・移管したリポジトリの PR は、答えの名前が変わっても、問い合わせた名前で解除した PR を付け直さない
// (D4b 最終レビュー I-3)。
func TestScanRenamedRepoKeepsUnlinked(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/web")
	at := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Open, "hh/WEB-1", at))
	f.scanWeb(t, linkStore)
	ref := itemlink.Ref{Kind: itemlink.PR, Repo: "example-org/web", Number: 5}
	if err := f.links.Remove(t.Context(), linkStore, linkEpic, ref); err != nil {
		t.Fatal(err)
	}
	f.gh.MovePulls("example-org/web", "example-org/site")
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Open, "hh/WEB-1", at.Add(time.Hour)))
	f.scanWeb(t, linkStore)
	if rows := f.list(t, linkStore, linkEpic); len(rows) != 0 {
		t.Errorf("rows = %q, want none (unlinked under the old name)", displays(rows))
	}
}

// Merged のリンク(取り直さない)があるアイテムに、改名後の名前で同じ PR を重ねない(D4b 最終レビュー I-3)。
func TestScanRenamedRepoNoDoubleLink(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/web")
	at := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Merged, "hh/WEB-1", at))
	f.scanWeb(t, linkStore)
	f.gh.MovePulls("example-org/web", "example-org/site")
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Merged, "hh/WEB-1", at.Add(time.Hour)))
	f.scanWeb(t, linkStore)
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"pr example-org/web#5 Add login form merged"}) {
		t.Errorf("rows = %q, want the one link", got)
	}
}
