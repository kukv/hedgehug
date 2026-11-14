// Package bulkchange は、選んだアイテムの項目を 1 つ、まとめて変える API(F2b2)。
// Store をまたぐので /api/stores/{store} の下に置かない(F2b2-24)。
package bulkchange

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appbulkchange "github.com/kukv/hedgehug/internal/app/bulkchange"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ChangeTarget は、一括変更で選んだアイテム 1 つ。
type ChangeTarget struct {
	Store string `json:"store" doc:"Store の ID"`
	Item  string `json:"item" doc:"アイテムの内部 ID"`
}

// ChoicesRequest は、値の選択肢を求める本文。
type ChoicesRequest struct {
	Items []ChangeTarget `json:"items" minItems:"1" nullable:"false" doc:"選んだアイテム。複数の Store にまたがってよい"`
}

// ChangeRequest は、一括変更の予告と実行の本文。
type ChangeRequest struct {
	Items    []ChangeTarget `json:"items" minItems:"1" nullable:"false" doc:"選んだアイテム。複数の Store にまたがってよい"`
	Field    string         `json:"field" enum:"status,assignee,priority,sprint,release,systemsAdd,systemsRemove,points,estimate,start,end" doc:"変える項目"`
	Value    string         `json:"value,omitempty" doc:"新しい値。空は値なし。ステータスを分類で変えるときは空"`
	Values   []string       `json:"values,omitempty" nullable:"false" doc:"関連システムの名前(systemsAdd・systemsRemove のとき)"`
	Category string         `json:"category,omitempty" enum:"\"\",todo,doing,done,canceled" doc:"ステータスを分類で変えるときの分類。名前で変えるときは空"`
}

// ChoiceMember は、担当者の選択肢。
type ChoiceMember struct {
	Login string `json:"login" doc:"GitHub のログイン名"`
	Name  string `json:"name" doc:"表示名"`
}

// ChangeChoices は、ダイアログの値の選択肢。
type ChangeChoices struct {
	Kinds      []string       `json:"kinds" nullable:"false" doc:"選んだアイテムの種類"`
	Statuses   []string       `json:"statuses" nullable:"false" doc:"ステータスの名前"`
	Priorities []string       `json:"priorities" nullable:"false" doc:"優先度の名前"`
	Sprints    []string       `json:"sprints" nullable:"false" doc:"スプリントの名前"`
	Releases   []string       `json:"releases" nullable:"false" doc:"リリースの名前"`
	Systems    []string       `json:"systems" nullable:"false" doc:"関連システムの名前"`
	Members    []ChoiceMember `json:"members" nullable:"false" doc:"担当者にできるメンバー"`
}

// ChangeKept は、変えずに残したアイテムと、その理由。
type ChangeKept struct {
	Store  string `json:"store" doc:"Store の ID"`
	Item   string `json:"item" doc:"アイテムの内部 ID"`
	Reason string `json:"reason" doc:"変えなかった理由"`
}

// ChangeFailed は、保存できなかった Store と、そこで変わるはずだった件数。
type ChangeFailed struct {
	Store string `json:"store" doc:"Store の ID"`
	Name  string `json:"name" doc:"Store の名前"`
	Count int    `json:"count" doc:"変わるはずだった件数"`
}

// ChangeOffBacklog は、変えたアイテムのうち、完了・中止のためバックログに表示されなくなったものの数。
type ChangeOffBacklog struct {
	Done     int `json:"done" doc:"分類が完了のもの"`
	Canceled int `json:"canceled" doc:"分類が中止のもの"`
}

// ChangeResult は、一括変更の予告と実行の結果。
type ChangeResult struct {
	Changed    int              `json:"changed" doc:"変えた(予告では変わる)件数"`
	Kept       []ChangeKept     `json:"kept" nullable:"false" doc:"変えずに残したアイテム。選んだ順"`
	Failed     []ChangeFailed   `json:"failed" nullable:"false" doc:"保存できなかった Store"`
	OffBacklog ChangeOffBacklog `json:"offBacklog" doc:"バックログに表示されなくなった件数"`
	ValueText  string           `json:"valueText" doc:"確認の本文に出す値の文字列"`
}

type choicesInput struct{ Body ChoicesRequest }

type choicesOutput struct{ Body ChangeChoices }

type changeInput struct{ Body ChangeRequest }

type changeOutput struct{ Body ChangeResult }

func toTargets(in []ChangeTarget) []appbulkchange.Target {
	out := make([]appbulkchange.Target, 0, len(in))
	for _, t := range in {
		out = append(out, appbulkchange.Target{Store: port.StoreID(t.Store), ID: item.ID(t.Item)})
	}
	return out
}

// fail は、型に合わない値(日付の形でないなど)を 422 にし、ほかは apierr.From に任せる。
func fail(err error) error {
	if errors.Is(err, item.ErrBadInput) {
		return huma.Error422UnprocessableEntity(err.Error())
	}
	return apierr.From(err)
}

func toResult(r appbulkchange.Result) ChangeResult {
	out := ChangeResult{
		Changed: r.Changed, Kept: []ChangeKept{}, Failed: []ChangeFailed{},
		OffBacklog: ChangeOffBacklog{Done: r.OffBacklog.Done, Canceled: r.OffBacklog.Canceled}, ValueText: r.ValueText,
	}
	for _, k := range r.Kept {
		out.Kept = append(out.Kept, ChangeKept{Store: string(k.Store), Item: string(k.ID), Reason: k.Reason})
	}
	for _, f := range r.Failed {
		out.Failed = append(out.Failed, ChangeFailed{Store: string(f.Store), Name: f.Name, Count: f.Count})
	}
	return out
}

func toInput(b ChangeRequest) appbulkchange.Input {
	values := b.Values
	if values == nil {
		values = []string{}
	}
	return appbulkchange.Input{Field: appbulkchange.Field(b.Field), Value: b.Value, Values: values, Category: workspace.Category(b.Category)}
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// Register は、一括変更の 3 本の API を登録する。
func Register(api huma.API, svc appbulkchange.Service) {
	huma.Register(api, huma.Operation{OperationID: "bulk-change-choices", Method: http.MethodPost, Path: "/api/bulk-change/choices", Summary: "選んだアイテムを一括変更するときの値の選択肢を返す"},
		func(ctx context.Context, in *choicesInput) (*choicesOutput, error) {
			c, err := svc.Choices(ctx, toTargets(in.Body.Items))
			if err != nil {
				return nil, fail(err)
			}
			members := make([]ChoiceMember, 0, len(c.Members))
			for _, m := range c.Members {
				members = append(members, ChoiceMember{Login: m.Login, Name: m.Name})
			}
			return &choicesOutput{Body: ChangeChoices{
				Kinds: nonNil(c.Kinds), Statuses: nonNil(c.Statuses), Priorities: nonNil(c.Priorities),
				Sprints: nonNil(c.Sprints), Releases: nonNil(c.Releases), Systems: nonNil(c.Systems), Members: members,
			}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "preview-bulk-change", Method: http.MethodPost, Path: "/api/bulk-change/preview", Summary: "選んだアイテムの項目を一括変更するときの確認の中身を返す(保存しない)"},
		func(ctx context.Context, in *changeInput) (*changeOutput, error) {
			r, err := svc.Preview(ctx, toTargets(in.Body.Items), toInput(in.Body))
			if err != nil {
				return nil, fail(err)
			}
			return &changeOutput{Body: toResult(r)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "bulk-change", Method: http.MethodPost, Path: "/api/bulk-change", Summary: "選んだアイテムの項目を、Store ごとに一括変更する"},
		func(ctx context.Context, in *changeInput) (*changeOutput, error) {
			r, err := svc.Run(ctx, toTargets(in.Body.Items), toInput(in.Body))
			if err != nil {
				return nil, fail(err)
			}
			return &changeOutput{Body: toResult(r)}, nil
		})
}
