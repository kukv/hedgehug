// Package github は、GitHub 連携の API(仕様 07)を持つ。
package github

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// Services は、GitHub 連携の API が使う usecase。
type Services struct {
	List    ghapp.ListAccounts
	Add     ghapp.AddAccount
	Edit    ghapp.EditAccount
	Delete  ghapp.DeleteAccount
	Reauth  ghapp.Reauth
	Recheck ghapp.RecheckOrgs
	Impact  ghapp.Impact
	SignIn  ghapp.SignIn
	Links   githublink.Links
}

type accountBody struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Login  string `json:"login"`
	Method string `json:"method" enum:"device,gh,env,keychain"`
}

func toBody(a account.Account) accountBody {
	return accountBody{ID: string(a.ID), Name: a.Name, Login: a.Login, Method: string(a.Method)}
}

// accountRow は一覧の 1 行(07「状態」)。reason は「認証エラー」の下の赤字(なければ空)。
// huma は埋め込みの構造体をスキーマに展開しないので、accountBody を埋め込まず欄を並べる。
type accountRow struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Login  string   `json:"login"`
	Method string   `json:"method" enum:"device,gh,env,keychain"`
	Status string   `json:"status" enum:"ok,auth_error,app_missing"`
	Reason string   `json:"reason"`
	Orgs   []string `json:"orgs" doc:"GitHub App が入っていない組織(認証エラーのときは空)"`
}

type listOutput struct {
	Body struct {
		Accounts []accountRow `json:"accounts"`
	}
}

type recheckOutput struct {
	Body struct {
		Cleared []string `json:"cleared" doc:"読めるようになった組織"`
		Still   []string `json:"still" doc:"まだ読めない組織"`
	}
}

type valuesBody struct {
	Name   string `json:"name"`
	Login  string `json:"login"`
	Method string `json:"method" enum:"gh,env,keychain" doc:"デバイスフローはサインインの API で登録・保存する"`
}

type addInput struct {
	Body valuesBody
}

type accountOutput struct {
	Body accountBody
}

type idInput struct {
	ID string `path:"id"`
}

type editInput struct {
	ID   string `path:"id"`
	Body valuesBody
}

type editImpactInput struct {
	ID    string `path:"id"`
	Login string `query:"login"`
}

type impactOutput struct {
	Body struct {
		Lines []string `json:"lines"`
	}
}

// Register は、アカウントとサインインの API を登録する。
func Register(api huma.API, s Services) {
	registerAccounts(api, s)
	registerManage(api, s)
	registerSignIn(api, s.SignIn)
	registerLinks(api, s.Links)
}

func registerAccounts(api huma.API, s Services) {
	huma.Register(api, huma.Operation{
		OperationID: "list-accounts", Method: http.MethodGet, Path: "/api/accounts",
		Summary: "登録したアカウントを、状態を付けて返す",
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		rows, err := s.List.Do(ctx)
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &listOutput{}
		out.Body.Accounts = make([]accountRow, 0, len(rows))
		for _, r := range rows {
			status := "ok"
			switch {
			case r.AuthError:
				status = "auth_error"
			case len(r.Orgs) > 0:
				status = "app_missing"
			}
			out.Body.Accounts = append(out.Body.Accounts, accountRow{
				ID: string(r.Account.ID), Name: r.Account.Name, Login: r.Account.Login, Method: string(r.Account.Method),
				Status: status, Reason: r.Reason, Orgs: append([]string{}, r.Orgs...),
			})
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "add-account", Method: http.MethodPost, Path: "/api/accounts",
		Summary: "GitHub で確かめてからアカウントを登録する", DefaultStatus: http.StatusCreated,
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *addInput) (*accountOutput, error) {
		a, err := s.Add.Do(ctx, ghapp.AddAccountInput{Name: in.Body.Name, Login: in.Body.Login, Method: account.Method(in.Body.Method)})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &accountOutput{Body: toBody(a)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "edit-account", Method: http.MethodPut, Path: "/api/accounts/{id}",
		Summary: "新しい値を GitHub で確かめてから、アカウントの値を入れ替える",
		Errors:  []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *editInput) (*accountOutput, error) {
		a, err := s.Edit.Do(ctx, ghapp.EditAccountInput{
			ID: account.ID(in.ID), Name: in.Body.Name, Login: in.Body.Login, Method: account.Method(in.Body.Method),
		})
		if err != nil {
			return nil, fromErr(err)
		}
		return &accountOutput{Body: toBody(a)}, nil
	})
}

func registerManage(api huma.API, s Services) {
	huma.Register(api, huma.Operation{
		OperationID: "delete-account", Method: http.MethodDelete, Path: "/api/accounts/{id}",
		Summary: "アカウントを削除し、割り当てた Store を「なし」にする", DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusNotFound},
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		return nil, apierr.From(s.Delete.Do(ctx, account.ID(in.ID)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "reauth-account", Method: http.MethodPost, Path: "/api/accounts/{id}/reauth",
		Summary: "gh CLI・環境変数・OS のキーチェーンのアカウントの認証の確認をやり直す",
		Errors:  []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *idInput) (*accountOutput, error) {
		a, err := s.Reauth.Do(ctx, account.ID(in.ID))
		if err != nil {
			return nil, fromErr(err)
		}
		return &accountOutput{Body: toBody(a)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "recheck-account", Method: http.MethodPost, Path: "/api/accounts/{id}/recheck",
		Summary: "GitHub App 未導入の組織を読み直し、読めたものの記録を消す",
		Errors:  []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *idInput) (*recheckOutput, error) {
		r, err := s.Recheck.Do(ctx, account.ID(in.ID))
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &recheckOutput{}
		out.Body.Cleared, out.Body.Still = r.Cleared, r.Still
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "account-delete-impact", Method: http.MethodGet, Path: "/api/accounts/{id}/delete-impact",
		Summary: "「アカウントを削除」の確認の本文の行", Errors: []int{http.StatusNotFound},
	}, func(ctx context.Context, in *idInput) (*impactOutput, error) {
		lines, err := s.Impact.Delete(ctx, account.ID(in.ID))
		return impact(lines, err)
	})
	huma.Register(api, huma.Operation{
		OperationID: "account-edit-impact", Method: http.MethodGet, Path: "/api/accounts/{id}/edit-impact",
		Summary: "ログイン名を変えるときの「アカウントを変更」の確認の本文の行(空なら確認を出さない)",
		Errors:  []int{http.StatusNotFound},
	}, func(ctx context.Context, in *editImpactInput) (*impactOutput, error) {
		lines, err := s.Impact.Edit(ctx, account.ID(in.ID), in.Login)
		return impact(lines, err)
	})
}

func impact(lines []string, err error) (*impactOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	out := &impactOutput{}
	out.Body.Lines = append([]string{}, lines...)
	return out, nil
}

// fromErr は、apierr.From に加えて、デバイスフローのアカウントをこの API で再認証・保存しようとしたときを 409 にする
// (デバイスフローは /api/sign-ins でサインインし直す。D2 画面の計画の決定 S6)。
func fromErr(err error) error {
	if errors.Is(err, ghapp.ErrDeviceFlow) {
		return huma.Error409Conflict("デバイスフローのアカウントは /api/sign-ins でサインインし直す")
	}
	return apierr.From(err)
}
