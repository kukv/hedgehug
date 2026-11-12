package github

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/domain/account"
)

type signInInput struct {
	Body struct {
		Account string `json:"account,omitempty" doc:"編集・再認証するアカウントの ID。登録では空"`
		Name    string `json:"name"`
		Login   string `json:"login"`
	}
}

type signInCode struct {
	ID              string `json:"id"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in" doc:"秒"`
	Interval        int    `json:"interval" doc:"秒"`
}

type signInCodeOutput struct {
	Body signInCode
}

type signInIDInput struct {
	ID string `path:"id"`
}

type signInResult struct {
	State    string       `json:"state" enum:"waiting,expired,denied,mismatch,failed,done"`
	Message  string       `json:"message"`
	Interval int          `json:"interval" doc:"次に問い合わせるまでの秒"`
	Account  *accountBody `json:"account,omitempty"`
}

type signInResultOutput struct {
	Body signInResult
}

func registerSignIn(api huma.API, s ghapp.SignIn) {
	huma.Register(api, huma.Operation{
		OperationID: "start-sign-in", Method: http.MethodPost, Path: "/api/sign-ins",
		Summary: "デバイスフローのサインインを始め、GitHub で入力するコードを返す", DefaultStatus: http.StatusCreated,
		Errors: []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *signInInput) (*signInCodeOutput, error) {
		c, err := s.Start(ctx, ghapp.SignInInput{Account: account.ID(in.Body.Account), Name: in.Body.Name, Login: in.Body.Login})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &signInCodeOutput{Body: signInCode{
			ID: c.ID, UserCode: c.UserCode, VerificationURI: c.VerificationURI,
			ExpiresIn: int(c.ExpiresIn.Seconds()), Interval: int(c.Interval.Seconds()),
		}}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "poll-sign-in", Method: http.MethodGet, Path: "/api/sign-ins/{id}",
		Summary: "サインインが許可されたかを GitHub に 1 回聞く", Errors: []int{http.StatusNotFound},
	}, func(ctx context.Context, in *signInIDInput) (*signInResultOutput, error) {
		r, err := s.Poll(ctx, in.ID)
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &signInResultOutput{Body: signInResult{State: string(r.State), Message: r.Message, Interval: int(r.Interval.Seconds())}}
		if r.State == ghapp.SignInDone {
			b := toBody(r.Account)
			out.Body.Account = &b
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "cancel-sign-in", Method: http.MethodDelete, Path: "/api/sign-ins/{id}",
		Summary: "サインインをやめる(何も登録しない)", DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, in *signInIDInput) (*struct{}, error) {
		s.Cancel(in.ID)
		return nil, nil
	})
}
