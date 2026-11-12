// Package github は、GitHub 連携の API(仕様 07)を持つ。
package github

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/domain/account"
)

type accountBody struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Login  string `json:"login"`
	Method string `json:"method" enum:"device,gh,env,keychain"`
}

func toBody(a account.Account) accountBody {
	return accountBody{ID: string(a.ID), Name: a.Name, Login: a.Login, Method: string(a.Method)}
}

type listOutput struct {
	Body struct {
		Accounts []accountBody `json:"accounts"`
	}
}

type addInput struct {
	Body struct {
		Name   string `json:"name"`
		Login  string `json:"login"`
		Method string `json:"method" enum:"gh,env,keychain" doc:"デバイスフローはサインインの API で登録する"`
	}
}

type addOutput struct {
	Body accountBody
}

// Register は、アカウントの API を登録する。
func Register(api huma.API, list ghapp.ListAccounts, add ghapp.AddAccount) {
	huma.Register(api, huma.Operation{
		OperationID: "list-accounts",
		Method:      http.MethodGet,
		Path:        "/api/accounts",
		Summary:     "登録したアカウントを返す",
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		accs, err := list.Do(ctx)
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &listOutput{}
		out.Body.Accounts = make([]accountBody, 0, len(accs))
		for _, a := range accs {
			out.Body.Accounts = append(out.Body.Accounts, toBody(a))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID:   "add-account",
		Method:        http.MethodPost,
		Path:          "/api/accounts",
		Summary:       "GitHub で確かめてからアカウントを登録する",
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *addInput) (*addOutput, error) {
		a, err := add.Do(ctx, ghapp.AddAccountInput{Name: in.Body.Name, Login: in.Body.Login, Method: account.Method(in.Body.Method)})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &addOutput{Body: toBody(a)}, nil
	})
}
