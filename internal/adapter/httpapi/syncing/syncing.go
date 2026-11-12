// Package syncing は、同期の API(仕様 06「手動で同期する」「同期の状態の表示」「自動で同期する」)を持つ。
package syncing

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appsyncing "github.com/kukv/hedgehug/internal/app/syncing"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// Services は、同期の API が使う usecase。どちらもサーバーに 1 つ。
type Services struct {
	Sync *appsyncing.Service
	Auto *appsyncing.Auto
}

type storeInput struct {
	Store string `path:"store"`
}

type statusOutput struct{ Body SyncStatusList }

type storeOutput struct{ Body SyncStoreResult }

type allOutput struct{ Body SyncAllResult }

type holdInput struct{ Body SyncHold }

type networkInput struct{ Body SyncNetwork }

func Register(api huma.API, s Services) {
	huma.Register(api, huma.Operation{
		OperationID: "list-sync-status", Method: http.MethodGet, Path: "/api/sync",
		Summary: "remote がある Store の同期の状態を返す",
	}, func(ctx context.Context, _ *struct{}) (*statusOutput, error) {
		st, err := s.Sync.Status(ctx)
		if err != nil {
			return nil, err
		}
		timing, err := s.Auto.Settings.Timing(ctx)
		if err != nil {
			return nil, err
		}
		return &statusOutput{Body: toStatusList(st, timing.Auto)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "sync-all", Method: http.MethodPost, Path: "/api/sync",
		Summary: "表示中で remote がある Store をすべて同期する",
	}, func(ctx context.Context, _ *struct{}) (*allOutput, error) {
		all, err := s.Sync.SyncAll(ctx)
		if err != nil {
			return nil, err
		}
		return &allOutput{Body: toAll(all)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "sync-store", Method: http.MethodPost, Path: "/api/stores/{store}/sync",
		Summary: "1 つの Store を同期する", Errors: []int{http.StatusNotFound},
	}, func(ctx context.Context, in *storeInput) (*storeOutput, error) {
		out, err := s.Sync.SyncStore(ctx, port.StoreID(in.Store))
		if err != nil {
			return nil, apierr.From(err)
		}
		return &storeOutput{Body: toStoreResult(out)}, nil
	})
	registerAuto(api, s.Auto)
}

func registerAuto(api huma.API, a *appsyncing.Auto) {
	huma.Register(api, huma.Operation{
		OperationID: "hold-sync", Method: http.MethodPut, Path: "/api/sync/hold",
		Summary: "見えているタブが、自動の同期を待たせる条件にあるかを知らせる", DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, in *holdInput) (*struct{}, error) {
		a.Hold(in.Body.Tab, in.Body.Hold)
		return nil, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "sync-network", Method: http.MethodPut, Path: "/api/sync/network",
		Summary: "タブがネットワークにつながっているかを知らせる(オフラインのあいだは自動で同期しない)", DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, in *networkInput) (*struct{}, error) {
		a.Network(in.Body.Tab, in.Body.Online)
		return nil, nil
	})
}

func toStatusList(st []appsyncing.StoreStatus, auto bool) SyncStatusList {
	out := SyncStatusList{Auto: auto, Stores: []SyncStoreStatus{}}
	for _, s := range st {
		last := ""
		if !s.LastSuccess.IsZero() {
			last = s.LastSuccess.UTC().Format(time.RFC3339)
		}
		out.Stores = append(out.Stores, SyncStoreStatus{
			Store: string(s.Store), Name: s.Name, Hidden: s.Hidden, ReadOnly: s.ReadOnly,
			Error: s.Reason.Short(), Unsent: s.Unsent, LastSuccess: last,
		})
	}
	return out
}

func toStoreResult(o appsyncing.Outcome) SyncStoreResult {
	r := SyncStoreResult{Result: "synced", Message: o.Message()}
	switch {
	case o.Skipped:
		r.Result = "skipped"
	case !o.OK():
		r.Result = "failed"
	}
	if o.MirrorFailures > 0 {
		r.MirrorWarning = syncstatus.MirrorsFailed(o.MirrorFailures)
	}
	return r
}

func toAll(a appsyncing.AllOutcome) SyncAllResult {
	r := SyncAllResult{Result: "synced", Lines: a.Lines}
	switch {
	case a.Nothing:
		r.Result = "nothing"
	case slices.ContainsFunc(a.Outcomes, func(o appsyncing.Outcome) bool { return !o.OK() }):
		r.Result = "failed"
	}
	return r
}
