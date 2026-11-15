package syncing

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/port"
)

// SyncTimingSetting は、設定の「一般」の「同期のタイミング」(08。このマシンだけの設定)。
type SyncTimingSetting struct {
	Auto              bool `json:"auto" doc:"裏で自動的に同期する"`
	IntervalMinutes   int  `json:"intervalMinutes" doc:"間隔(分)。1・5・15・30・60 のどれか。ほかは 400"`
	AfterWriteSeconds int  `json:"afterWriteSeconds" doc:"書き込みのあと同期するまで(秒)。10・30・60 のどれか。ほかは 400"`
	GitHubMinutes     int  `json:"githubMinutes" doc:"GitHub を読む間隔(分)。5・15・30・60 のどれか。ほかは 400"`
}

// 08 の選択肢。値の検査は 400 にするため enum にしない(huma の検査は 422 になる)。
var (
	intervalMinutes   = []int{1, 5, 15, 30, 60}
	afterWriteSeconds = []int{10, 30, 60}
	githubMinutes     = []int{5, 15, 30, 60}
)

type timingOutput struct{ Body SyncTimingSetting }

type timingInput struct{ Body SyncTimingSetting }

// RegisterTiming は、同期のタイミングの API を登録する。保存したら EventSync を知らせる(G1-17)。
func RegisterTiming(api huma.API, settings port.SyncSettings, events port.Events) {
	huma.Register(api, huma.Operation{
		OperationID: "get-sync-timing", Method: http.MethodGet, Path: "/api/sync/timing",
		Summary: "同期のタイミングを返す",
	}, func(ctx context.Context, _ *struct{}) (*timingOutput, error) {
		t, err := settings.Timing(ctx)
		if err != nil {
			return nil, err
		}
		return &timingOutput{Body: SyncTimingSetting{
			Auto: t.Auto, IntervalMinutes: int(t.Interval / time.Minute),
			AfterWriteSeconds: int(t.AfterWrite / time.Second), GitHubMinutes: int(t.GitHubEvery / time.Minute),
		}}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-sync-timing", Method: http.MethodPut, Path: "/api/sync/timing",
		Summary: "同期のタイミングを保存する", DefaultStatus: http.StatusNoContent, Errors: []int{http.StatusBadRequest},
	}, func(ctx context.Context, in *timingInput) (*struct{}, error) {
		b := in.Body
		if !slices.Contains(intervalMinutes, b.IntervalMinutes) || !slices.Contains(afterWriteSeconds, b.AfterWriteSeconds) ||
			!slices.Contains(githubMinutes, b.GitHubMinutes) {
			return nil, huma.Error400BadRequest("選べない値です")
		}
		err := settings.SetTiming(ctx, port.SyncTiming{
			Auto: b.Auto, Interval: time.Duration(b.IntervalMinutes) * time.Minute,
			AfterWrite: time.Duration(b.AfterWriteSeconds) * time.Second, GitHubEvery: time.Duration(b.GitHubMinutes) * time.Minute,
		})
		if err != nil {
			return nil, err
		}
		events.Publish(port.Event{Kind: port.EventSync})
		return nil, nil
	})
}
