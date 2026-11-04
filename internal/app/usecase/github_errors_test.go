package usecase_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// The Kind and Code of each way GitHub can refuse (spec: the table of errors).
func TestGitHubErrKindsAndCodes(t *testing.T) {
	t.Parallel()
	acct := config.Account{Name: "work", Host: "github.com", Login: "alice"}
	for _, c := range []struct {
		err  error
		kind usecase.Kind
		code string
	}{
		{github.ErrUnauthorized, usecase.KindInvalid, "auth.unauthorized"},
		{github.ErrForbidden, usecase.KindInvalid, "github.forbidden"},
		{github.ErrNotFound, usecase.KindInvalid, "github.not_found"},
		{&github.RateLimitError{Reset: time.Now().Add(time.Minute)}, usecase.KindInvalid, "github.rate_limit"},
	} {
		err := usecase.GithubErr(c.err, acct, "list the repositories")
		usecasetest.WantErr(t, err, c.kind, c.code)
		if e := usecase.AsError(err); e.Args["account"] != "work" {
			t.Errorf("%s: account arg = %v", c.code, e.Args["account"])
		}
	}
	if err := usecase.GithubErr(errors.New("boom"), acct, "x"); usecase.KindOf(err) != usecase.KindUnknown {
		t.Errorf("an unclassified error got a code: %v", err)
	}
}
