package statedb

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestPRScans(t *testing.T) {
	s := openTemp(t).PRScans()
	ctx := t.Context()
	rec := port.ScanRecord{Read: true, Since: time.Date(2026, 11, 14, 9, 0, 0, 123456789, time.UTC), Failure: "PR を探せませんでした"}
	save := func(store port.StoreID, repo string, r port.ScanRecord, want bool) {
		t.Helper()
		changed, err := s.SaveScan(ctx, store, repo, r)
		if err != nil || changed != want {
			t.Fatalf("SaveScan(%s, %s, %+v) = %v, %v; want %v", store, repo, r, changed, err, want)
		}
	}
	scans := func(store port.StoreID) map[string]port.ScanRecord {
		t.Helper()
		got, err := s.Scans(ctx, store)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	save("S1", "Org/Web", rec, true)
	save("S1", "org/web", rec, false)
	if got := scans("S1"); len(got) != 1 || !reflect.DeepEqual(got["org/web"], rec) {
		t.Fatalf("Scans = %+v", got)
	}
	if other := scans("S2"); len(other) != 0 {
		t.Errorf("other store: %+v", other)
	}
	rec.Failure = ""
	save("S1", "org/web", rec, true)
	// 失敗の文言だけ置いた記録(まだ探していない)も戻る。
	save("S1", "org/api", port.ScanRecord{Failure: "x"}, true)
	got := scans("S1")
	if want := (port.ScanRecord{Failure: "x"}); !reflect.DeepEqual(got["org/api"], want) || !reflect.DeepEqual(got["org/web"], rec) {
		t.Errorf("after update: %+v", got)
	}
}

// 同時に動いた探索(ScanRepo と ReadOnce)が、古い読み取りをもとに書いても、「読んだか」は偽に戻らず、どこまで
// 見たかは大きい方が残る。失敗の文言は書く側のもの(D4b 最終レビュー I-4)。
func TestPRScansMergesWithStoredRow(t *testing.T) {
	s := openTemp(t).PRScans()
	ctx := t.Context()
	later := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	earlier := later.Add(-time.Hour)
	if _, err := s.SaveScan(ctx, "S1", "org/web", port.ScanRecord{Read: true, Since: later}); err != nil {
		t.Fatal(err)
	}
	changed, err := s.SaveScan(ctx, "S1", "org/web", port.ScanRecord{Read: false, Since: earlier})
	if err != nil || changed {
		t.Errorf("stale save = %v, %v; want no change", changed, err)
	}
	changed, err = s.SaveScan(ctx, "S1", "org/web", port.ScanRecord{Since: earlier, Failure: "x"})
	if err != nil || !changed {
		t.Errorf("failure save = %v, %v; want changed", changed, err)
	}
	got, err := s.Scans(ctx, "S1")
	if err != nil {
		t.Fatal(err)
	}
	if want := (port.ScanRecord{Read: true, Since: later, Failure: "x"}); !reflect.DeepEqual(got["org/web"], want) {
		t.Errorf("merged = %+v, want %+v", got["org/web"], want)
	}
}

func TestAppMissing(t *testing.T) {
	m := openTemp(t).AppMissing()
	ctx := t.Context()
	put := func(a account.ID, org, repo string, want bool) {
		t.Helper()
		changed, err := m.Put(ctx, a, port.MissingOrg{Org: org, Repo: repo})
		if err != nil || changed != want {
			t.Fatalf("Put(%s, %s) = %v, %v; want %v", a, org, changed, err, want)
		}
	}
	list := func() map[account.ID][]port.MissingOrg {
		t.Helper()
		got, err := m.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	put("A1", "Example-Org", "Example-Org/web", true)
	put("A1", "Other", "Other/api", true)
	put("A2", "Example-Org", "Example-Org/web", true)
	put("A1", "example-org", "example-org/docs", false)
	want := map[account.ID][]port.MissingOrg{
		"A1": {{Org: "Example-Org", Repo: "example-org/docs"}, {Org: "Other", Repo: "Other/api"}},
		"A2": {{Org: "Example-Org", Repo: "Example-Org/web"}},
	}
	if got := list(); !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %+v, want %+v", got, want)
	}
	clearOrg := func(a account.ID, org, repo string, want bool) {
		t.Helper()
		changed, err := m.Clear(ctx, a, org, repo)
		if err != nil || changed != want {
			t.Fatalf("Clear(%s, %s, %s) = %v, %v; want %v", a, org, repo, changed, err, want)
		}
	}
	// 記録していないリポジトリが読めても消さない(I-1)。
	clearOrg("A1", "example-org", "example-org/web", false)
	clearOrg("A1", "EXAMPLE-ORG", "Example-Org/Docs", true)
	clearOrg("A1", "example-org", "example-org/docs", false)
	want = map[account.ID][]port.MissingOrg{
		"A1": {{Org: "Other", Repo: "Other/api"}},
		"A2": {{Org: "Example-Org", Repo: "Example-Org/web"}},
	}
	if got := list(); !reflect.DeepEqual(got, want) {
		t.Errorf("after clear: %+v, want %+v", got, want)
	}
	// アカウントを削除したら、そのアカウントの記録をすべて消す(I-2)。
	if err := m.Forget(ctx, "A1"); err != nil {
		t.Fatal(err)
	}
	want = map[account.ID][]port.MissingOrg{"A2": {{Org: "Example-Org", Repo: "Example-Org/web"}}}
	if got := list(); !reflect.DeepEqual(got, want) {
		t.Errorf("after forget: %+v, want %+v", got, want)
	}
}
