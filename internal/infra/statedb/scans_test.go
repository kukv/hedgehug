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
	save("S1", "Org/Web", rec, true)
	save("S1", "org/web", rec, false)
	got, err := s.Scans(ctx, "S1")
	if err != nil || len(got) != 1 || !reflect.DeepEqual(got["org/web"], rec) {
		t.Fatalf("Scans = %+v, %v", got, err)
	}
	if other, _ := s.Scans(ctx, "S2"); len(other) != 0 {
		t.Errorf("other store: %+v", other)
	}
	rec.Failure = ""
	save("S1", "org/web", rec, true)
	// 失敗の文言だけ置いた記録(まだ探していない)も戻る。
	save("S1", "org/api", port.ScanRecord{Failure: "x"}, true)
	got, _ = s.Scans(ctx, "S1")
	if want := (port.ScanRecord{Failure: "x"}); !reflect.DeepEqual(got["org/api"], want) || !reflect.DeepEqual(got["org/web"], rec) {
		t.Errorf("after update: %+v", got)
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
	put("A1", "Example-Org", "Example-Org/web", true)
	put("A1", "Other", "Other/api", true)
	put("A2", "Example-Org", "Example-Org/web", true)
	put("A1", "example-org", "example-org/docs", false)
	got, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[account.ID][]port.MissingOrg{
		"A1": {{Org: "Example-Org", Repo: "example-org/docs"}, {Org: "Other", Repo: "Other/api"}},
		"A2": {{Org: "Example-Org", Repo: "Example-Org/web"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %+v, want %+v", got, want)
	}
	clearOrg := func(a account.ID, org string, want bool) {
		t.Helper()
		changed, err := m.Clear(ctx, a, org)
		if err != nil || changed != want {
			t.Fatalf("Clear(%s, %s) = %v, %v; want %v", a, org, changed, err, want)
		}
	}
	clearOrg("A1", "EXAMPLE-ORG", true)
	clearOrg("A1", "example-org", false)
	got, _ = m.List(ctx)
	if len(got["A1"]) != 1 || got["A1"][0].Org != "Other" || len(got["A2"]) != 1 {
		t.Errorf("after clear: %+v", got)
	}
}
