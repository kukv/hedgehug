package githublink_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/ghread"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/prlink"
)

// newScanRead は、newRead の組み立てで、サンプルのワークスペースの接頭辞を WEB にしたもの(HH-1 は WEB-1、HH-2 は WEB-2)。
func newScanRead(t *testing.T) *readFixture {
	t.Helper()
	f := newRead(t)
	f.writeAs(t, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "WEB"))
	})
	return f
}

// addRepo は、store のサンプルのプロジェクトにリポジトリ repo を結びつける(準備)。
func (f *readFixture) addRepo(t *testing.T, store port.StoreID, repo string) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	err := f.stores.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject, change.NewAdd("repos", repo))
	})
	if err != nil {
		t.Fatal(err)
	}
}

// scanPull は、番号 n の PR(状態 s、ブランチ branch、更新 at)。
func scanPull(n int, s itemlink.State, branch string, at time.Time) port.Pull {
	return port.Pull{Number: n, Text: prlink.Text{Branch: branch}, Values: itemlink.Values{Title: "Add login form", State: s, Review: itemlink.NoReview}, UpdatedAt: at}
}

// scanRecord は、store の repo(小文字)の、PR を探した記録。
func (f *readFixture) scanRecord(store port.StoreID, repo string) port.ScanRecord {
	return f.scans.rows[store][repo]
}

func TestReadOnceScansProjectRepo(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/web")
	at := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Open, "hh/WEB-1", at))
	f.read(t, every, false)
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"pr example-org/web#5 Add login form open"}) {
		t.Errorf("rows = %q", got)
	}
	if rec := f.scanRecord(linkStore, "example-org/web"); rec != (port.ScanRecord{Read: true, Since: at}) {
		t.Errorf("record = %+v", rec)
	}
	if n := f.gh.ReadManyCalls("example-org/web"); n != 0 {
		t.Errorf("ReadMany calls = %d, want none without numbers", n)
	}
	if n := f.gh.ReadPullsCalls("example-org/web"); n != 1 {
		t.Errorf("ReadPulls calls = %d", n)
	}
	if n := f.githubEvents(linkStore); n != 1 {
		t.Errorf("events = %d", n)
	}
}

func TestReadOnceScansLinkedRepoOfMergedLinks(t *testing.T) {
	f := newScanRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("other/api", 1, itemlink.Merged))
	f.gh.PutPull("other/api", scanPull(2, itemlink.Open, "hh/WEB-1", f.clock.now))
	f.read(t, every, false)
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"pr other/api#2 Add login form open"}) {
		t.Errorf("rows = %q", got)
	}
	if n := f.gh.ReadManyCalls("other/api"); n != 0 {
		t.Errorf("ReadMany calls = %d, want none for merged links", n)
	}
}

func TestReadOnceSecondScanFires(t *testing.T) {
	f := newScanRead(t)
	f.withRules(t, statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone))
	f.addRepo(t, linkStore, "example-org/web")
	t1 := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Merged, "hh/WEB-1", t1))
	f.read(t, every, false)
	if it := f.epicItem(t); it.Status != apptest.SampleStatusTodo {
		t.Fatalf("first scan: status = %s, want unchanged", it.Status)
	}
	f.gh.PutPull("example-org/web", scanPull(6, itemlink.Merged, "hh/WEB-1", t1.Add(time.Hour)))
	f.gh.PutPull("example-org/web", scanPull(4, itemlink.Open, "hh/WEB-2", t1.Add(-time.Hour))) // since より古い
	f.advance(every)
	f.read(t, every, false)
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{
		"pr example-org/web#5 Add login form merged", "pr example-org/web#6 Add login form merged",
	}) {
		t.Errorf("rows = %q", got)
	}
	if it := f.epicItem(t); it.Status != apptest.SampleStatusDone {
		t.Errorf("second scan: status = %s", it.Status)
	}
	if rows := f.list(t, linkStore, linkTask); len(rows) != 0 {
		t.Errorf("older PR linked: %q", displays(rows))
	}
	if rec := f.scanRecord(linkStore, "example-org/web"); rec != (port.ScanRecord{Read: true, Since: t1.Add(time.Hour)}) {
		t.Errorf("record = %+v", rec)
	}
}

func TestReadOnceScanFailure(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/web")
	f.gh.PutPull("example-org/web", scanPull(5, itemlink.Open, "hh/WEB-1", f.clock.now))
	f.gh.Fail("example-org/web", 0, &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: "example-org/web"})
	f.read(t, every, false)
	want := port.ScanRecord{Failure: "example-org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください"}
	if rec := f.scanRecord(linkStore, "example-org/web"); rec != want {
		t.Errorf("record = %+v", rec)
	}
	if n := f.githubEvents(linkStore); n != 1 {
		t.Errorf("events = %d", n)
	}
	if rows := f.list(t, linkStore, linkEpic); len(rows) != 0 {
		t.Errorf("rows = %q", displays(rows))
	}
	f.gh.Fail("example-org/web", 0, nil)
	f.advance(every)
	f.read(t, every, false)
	if rec := f.scanRecord(linkStore, "example-org/web"); rec.Failure != "" || !rec.Read {
		t.Errorf("record after success = %+v", rec)
	}
	if rows := f.list(t, linkStore, linkEpic); len(rows) != 1 {
		t.Errorf("rows = %q", displays(rows))
	}
}

func TestReadOnceScanRateLimited(t *testing.T) {
	f := newScanRead(t)
	// プロジェクトのリポジトリは大文字小文字を区別しない辞書順なので、api を先に探す
	f.addRepo(t, linkStore, "example-org/api")
	f.addRepo(t, linkStore, "example-org/web")
	f.addRepo(t, otherStore, "example-org/web") // 同じアカウントの別の Store
	reset := f.clock.now.Add(23 * time.Minute)
	f.gh.Fail("example-org/api", 0, &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: reset})
	f.read(t, every, false)
	if n := f.gh.ReadPullsCalls("example-org/web"); n != 0 {
		t.Errorf("ReadPulls calls = %d, want example-org/web not asked", n)
	}
	want := "GitHub の利用上限に達しました。" + resetText(reset, f.clock.now) + " 以降にもう一度お試しください"
	for _, c := range []struct {
		store port.StoreID
		repo  string
	}{{linkStore, "example-org/api"}, {linkStore, "example-org/web"}, {otherStore, "example-org/web"}} {
		if rec := f.scanRecord(c.store, c.repo); rec != (port.ScanRecord{Failure: want}) {
			t.Errorf("%s %s record = %+v", c.store, c.repo, rec)
		}
		if rec := f.reads.records[c.store][c.repo]; !rec.NotBefore.Equal(reset) {
			t.Errorf("%s %s read record = %+v, want not before the reset", c.store, c.repo, rec)
		}
	}
}

func TestReadOnceRefreshThenScanOnePass(t *testing.T) {
	f := newScanRead(t)
	f.withRules(t, statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone))
	f.linkIn(t, linkStore, linkEpic, readPR("example-org/web", 5, itemlink.Open))
	f.gh.Put("example-org/web", readPR("example-org/web", 5, itemlink.Merged))
	t0 := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	f.scans.rows[linkStore] = map[string]port.ScanRecord{"example-org/web": {Read: true, Since: t0}} // 2 回目の探索
	f.gh.PutPull("example-org/web", scanPull(6, itemlink.Merged, "hh/WEB-1", t0.Add(time.Hour)))
	f.read(t, every, false)
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{
		"pr example-org/web#5 PR merged", "pr example-org/web#6 Add login form merged",
	}) {
		t.Errorf("rows = %q", got)
	}
	if rows := ruleRows(f.epicLog(t)); len(rows) != 1 || len(f.inbox.Notices()) != 1 {
		t.Errorf("rule rows = %+v, notices = %+v", rows, f.inbox.Notices())
	}
}

func TestReadOnceSkipsDeletedProjectRepo(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/web")
	if err := apptest.DeleteProject(t.Context(), f.stores.Stores, linkStore, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	f.read(t, every, false)
	if n := f.gh.ReadPullsCalls("example-org/web"); n != 0 {
		t.Errorf("ReadPulls calls = %d", n)
	}
}

func TestReadOnceNotDueNoScan(t *testing.T) {
	f := newScanRead(t)
	f.addRepo(t, linkStore, "example-org/web")
	f.reads.records[linkStore] = map[string]ghread.Record{"example-org/web": {Attempt: f.clock.now.Add(-time.Minute)}}
	f.read(t, every, false)
	if n := f.gh.ReadPullsCalls("example-org/web"); n != 0 {
		t.Errorf("ReadPulls calls = %d, want none inside the interval", n)
	}
}
