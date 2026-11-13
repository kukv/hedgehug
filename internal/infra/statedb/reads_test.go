package statedb

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghread"
)

func TestReadsRecordRoundTrip(t *testing.T) {
	r := openTemp(t).GitHubReads()
	ctx := t.Context()
	rec := ghread.Record{
		Attempt:   time.Date(2026, 11, 13, 9, 0, 0, 123456789, time.UTC),
		Failed:    true,
		Limit:     5000,
		Remaining: 12,
		ResetAt:   time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
		NotBefore: time.Date(2026, 11, 13, 9, 30, 0, 987654321, time.UTC),
	}
	if err := r.SaveRecord(ctx, "S1", "Org/Web", rec); err != nil {
		t.Fatal(err)
	}
	got, err := r.Records(ctx, "S1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got["org/web"], rec) || len(got) != 1 {
		t.Errorf("got %+v, want org/web = %+v", got, rec)
	}
	if other, _ := r.Records(ctx, "S2"); len(other) != 0 {
		t.Errorf("other store: %+v", other)
	}
	// 2 回目は上書き。ゼロ値の時刻はゼロのまま戻る。
	next := ghread.Record{Attempt: time.Date(2026, 11, 13, 11, 0, 0, 0, time.UTC)}
	if err := r.SaveRecord(ctx, "S1", "org/web", next); err != nil {
		t.Fatal(err)
	}
	got, _ = r.Records(ctx, "S1")
	if !reflect.DeepEqual(got["org/web"], next) || len(got) != 1 {
		t.Errorf("after overwrite: %+v", got)
	}
}

func TestReadsFailures(t *testing.T) {
	r := openTemp(t).GitHubReads()
	ctx := t.Context()
	const key, text = "pr:org/web#1", "状態を更新できませんでした: 接続できません"
	step := func(want bool, set string) {
		t.Helper()
		changed, err := r.SetFailure(ctx, "S1", key, set)
		if err != nil || changed != want {
			t.Fatalf("SetFailure(%q) = %v, %v; want %v", set, changed, err, want)
		}
	}
	step(true, text)
	step(false, text)
	if got, _ := r.Failures(ctx, "S1"); !reflect.DeepEqual(got, map[string]string{key: text}) {
		t.Errorf("failures: %v", got)
	}
	if got, _ := r.Failures(ctx, "S2"); len(got) != 0 {
		t.Errorf("other store: %v", got)
	}
	step(true, "")
	step(false, "")
	if got, _ := r.Failures(ctx, "S1"); len(got) != 0 {
		t.Errorf("after clear: %v", got)
	}
}
