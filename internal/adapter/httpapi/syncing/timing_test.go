package syncing_test

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/syncing"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func newTimingAPI(t *testing.T) (humatest.TestAPI, *apptest.SyncSettings, *apptest.Events) {
	t.Helper()
	settings := &apptest.SyncSettings{Value: port.SyncTiming{
		Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second, GitHubEvery: 15 * time.Minute,
	}}
	ev := &apptest.Events{}
	_, api := humatest.New(t)
	syncing.RegisterTiming(api, settings, ev)
	return api, settings, ev
}

func TestGetTiming(t *testing.T) {
	api, _, _ := newTimingAPI(t)
	res := api.Get("/api/sync/timing")
	want := `{"auto":true,"intervalMinutes":5,"afterWriteSeconds":30,"githubMinutes":15}` + "\n"
	if res.Code != http.StatusOK || res.Body.String() != want {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
}

func TestPutTiming(t *testing.T) {
	api, settings, ev := newTimingAPI(t)
	res := api.Put("/api/sync/timing", map[string]any{"auto": false, "intervalMinutes": 60, "afterWriteSeconds": 10, "githubMinutes": 30})
	if res.Code != http.StatusNoContent {
		t.Fatalf("put: %d %s", res.Code, res.Body)
	}
	got, _ := settings.Timing(context.Background())
	want := port.SyncTiming{Auto: false, Interval: time.Hour, AfterWrite: 10 * time.Second, GitHubEvery: 30 * time.Minute}
	if got != want {
		t.Errorf("timing = %+v", got)
	}
	if evs := ev.List(); !slices.Equal(evs, []port.Event{{Kind: port.EventSync}}) {
		t.Errorf("events = %+v", evs)
	}
}

// 選択肢にない値は 422「選べない値です」で、保存しない。
func TestPutTimingRefusesValuesOutsideTheChoices(t *testing.T) {
	bodies := []map[string]any{
		{"auto": true, "intervalMinutes": 2, "afterWriteSeconds": 30, "githubMinutes": 5},
		{"auto": true, "intervalMinutes": 5, "afterWriteSeconds": 20, "githubMinutes": 5},
		{"auto": true, "intervalMinutes": 5, "afterWriteSeconds": 30, "githubMinutes": 1},
		{"auto": true, "intervalMinutes": 0, "afterWriteSeconds": 0, "githubMinutes": 0},
	}
	for _, b := range bodies {
		api, settings, ev := newTimingAPI(t)
		res := api.Put("/api/sync/timing", b)
		if res.Code != http.StatusUnprocessableEntity || detailOf(t, res.Body.Bytes()) != "選べない値です" {
			t.Errorf("%v: %d %s", b, res.Code, res.Body)
		}
		if got, _ := settings.Timing(context.Background()); got.GitHubEvery != 15*time.Minute || len(ev.List()) != 0 {
			t.Errorf("%v: saved %+v, events %+v", b, got, ev.List())
		}
	}
}

func detailOf(t *testing.T, body []byte) string {
	t.Helper()
	return decode[struct{ Detail string }](t, body).Detail
}
