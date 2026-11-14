// Package inbox は、受信箱の API(仕様 06「受信箱」)。
package inbox

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appinbox "github.com/kukv/hedgehug/internal/app/inbox"
	"github.com/kukv/hedgehug/internal/app/port"
)

const base = "/api/inbox"

// tabs は、受け付けるタブ。
var tabs = []appinbox.Tab{
	appinbox.TabAll, appinbox.TabMention, appinbox.TabAssign, appinbox.TabWatch,
	appinbox.TabOverwrite, appinbox.TabAutomation, appinbox.TabRepair,
}

type listInput struct {
	Tab      string `query:"tab" doc:"all・mention・assign・watch・overwrite・automation・repair のどれか。ほかは 400"`
	BeforeAt string `query:"before_at" doc:"続きを読むとき、前の一覧の最後の行の at(RFC 3339、ナノ秒まで)。before_id と一緒に渡す"`
	BeforeID int64  `query:"before_id" doc:"続きを読むとき、前の一覧の最後の行の id"`
}

type noticeInput struct {
	Notice int64 `path:"notice"`
}

type readInput struct {
	Notice int64 `path:"notice"`
	Body   SetInboxRead
}

type readAllInput struct {
	Body ReadAllInbox
}

type (
	pageOutput   struct{ Body InboxPage }
	unreadOutput struct{ Body InboxUnread }
)

// Register は、受信箱の API を登録する。
func Register(api huma.API, svc appinbox.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-inbox", Method: http.MethodGet, Path: base,
		Summary: "受信箱のタブの通知を、新しい順に 300 件",
	},
		func(ctx context.Context, in *listInput) (*pageOutput, error) {
			tab, err := tabOf(in.Tab)
			if err != nil {
				return nil, err
			}
			before, err := cursorOf(in)
			if err != nil {
				return nil, err
			}
			p, err := svc.Page(ctx, tab, before)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &pageOutput{Body: toPage(p)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "inbox-unread", Method: http.MethodGet, Path: base + "/unread",
		Summary: "受信箱のタブごとの未読数",
	},
		func(ctx context.Context, _ *struct{}) (*unreadOutput, error) {
			c, err := svc.Counts(ctx)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &unreadOutput{Body: InboxUnread{Counts: toCounts(c)}}, nil
		})
	registerActions(api, svc)
}

func registerActions(api huma.API, svc appinbox.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "set-inbox-read", Method: http.MethodPut, Path: base + "/{notice}/read",
		Summary: "通知を既読・未読にする", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *readInput) (*struct{}, error) {
			return nil, apierr.From(svc.SetRead(ctx, port.NoticeID(in.Notice), in.Body.Read))
		})
	huma.Register(api, huma.Operation{
		OperationID: "read-all-inbox", Method: http.MethodPost, Path: base + "/read-all",
		Summary: "表示中の Store の、タブの通知をすべて既読にする", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *readAllInput) (*struct{}, error) {
			tab, err := tabOf(in.Body.Tab)
			if err != nil {
				return nil, err
			}
			return nil, apierr.From(svc.ReadAll(ctx, tab))
		})
	huma.Register(api, huma.Operation{
		OperationID: "apply-inbox-notice", Method: http.MethodPost, Path: base + "/{notice}/apply",
		Summary: "止まった自動化の変更を、制約を無視して適用", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *noticeInput) (*struct{}, error) {
			return nil, apierr.From(svc.Apply(ctx, port.NoticeID(in.Notice)))
		})
	huma.Register(api, huma.Operation{
		OperationID: "keep-inbox-notice", Method: http.MethodPost, Path: base + "/{notice}/keep",
		Summary: "止まった自動化の変更をしない", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *noticeInput) (*struct{}, error) {
			return nil, apierr.From(svc.Keep(ctx, port.NoticeID(in.Notice)))
		})
}

// tabOf は、7 つのタブのどれかを返す。ほかは 400(huma の検査は 422 になるため、手で確かめる)。
func tabOf(s string) (appinbox.Tab, error) {
	if t := appinbox.Tab(s); slices.Contains(tabs, t) {
		return t, nil
	}
	return "", huma.Error400BadRequest("unknown tab: " + s)
}

// cursorOf は、続きの位置。before_at がなければ先頭から(nil)。
func cursorOf(in *listInput) (*port.NoticeCursor, error) {
	if in.BeforeAt == "" {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339Nano, in.BeforeAt)
	if err != nil {
		return nil, huma.Error400BadRequest("not a time: " + in.BeforeAt)
	}
	return &port.NoticeCursor{At: at, ID: port.NoticeID(in.BeforeID)}, nil
}
