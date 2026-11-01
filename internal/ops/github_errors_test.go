package ops

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/github"
)

// The Kind and Code of each way GitHub can refuse (spec: the table of errors).
func TestGitHubErrKindsAndCodes(t *testing.T) {
	t.Parallel()
	acct := config.Account{Name: "work", Host: "github.com", Login: "alice"}
	for _, c := range []struct {
		err  error
		kind Kind
		code string
	}{
		{github.ErrUnauthorized, KindInvalid, "auth.unauthorized"},
		{github.ErrForbidden, KindInvalid, "github.forbidden"},
		{github.ErrNotFound, KindInvalid, "github.not_found"},
		{&github.RateLimitError{Reset: time.Now().Add(time.Minute)}, KindInvalid, "github.rate_limit"},
	} {
		err := githubErr(c.err, acct, "list the repositories")
		wantErr(t, err, c.kind, c.code)
		if e := AsError(err); e.Args["account"] != "work" {
			t.Errorf("%s: account arg = %v", c.code, e.Args["account"])
		}
	}
	if err := githubErr(errors.New("boom"), acct, "x"); KindOf(err) != KindUnknown {
		t.Errorf("an unclassified error got a code: %v", err)
	}
}
