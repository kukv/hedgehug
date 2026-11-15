package stores

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// LoginSetting は、マシンの設定「ログイン名」(08)。
type LoginSetting struct {
	Login string `json:"login"`
}

// LoginPreview は、ログイン名を変えたときの確認の本文(Store ごとに 1 行)。空なら確認なしで保存する。
type LoginPreview struct {
	Lines []string `json:"lines" nullable:"false"`
}

type loginOutput struct{ Body LoginSetting }

type loginInput struct{ Body LoginSetting }

type previewOutput struct{ Body LoginPreview }

// RegisterLogin は、ログイン名の API を登録する。使えない名前は 400 で、detail に断りの文言を返す。
func RegisterLogin(api huma.API, svc appstores.Login) {
	huma.Register(api, huma.Operation{
		OperationID: "get-login", Method: http.MethodGet, Path: "/api/settings/login",
		Summary: "マシンの設定「ログイン名」を返す",
	}, func(ctx context.Context, _ *struct{}) (*loginOutput, error) {
		login, err := svc.Get(ctx)
		if err != nil {
			return nil, err
		}
		return &loginOutput{Body: LoginSetting{Login: login}}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "preview-login", Method: http.MethodPost, Path: "/api/settings/login/preview",
		Summary: "ログイン名を変えたときの確認の本文を返す", Errors: []int{http.StatusBadRequest},
	}, func(ctx context.Context, in *loginInput) (*previewOutput, error) {
		lines, err := svc.Preview(ctx, in.Body.Login)
		if err != nil {
			return nil, loginErr(err)
		}
		return &previewOutput{Body: LoginPreview{Lines: append([]string{}, lines...)}}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-login", Method: http.MethodPut, Path: "/api/settings/login",
		Summary: "マシンの設定「ログイン名」を保存する", DefaultStatus: http.StatusNoContent, Errors: []int{http.StatusBadRequest},
	}, func(ctx context.Context, in *loginInput) (*struct{}, error) {
		return nil, loginErr(svc.Set(ctx, in.Body.Login))
	})
}

// loginErr は、断りを 400(detail に文言)にする。ほかは apierr のとおり。
func loginErr(err error) error {
	if r := (*refusal.Refusal)(nil); errors.As(err, &r) {
		return huma.Error400BadRequest(r.Message)
	}
	return apierr.From(err)
}
