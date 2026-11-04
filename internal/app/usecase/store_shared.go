package usecase

import (
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// GithubErr describes err of doing, as acct, and gives it its Kind and Code.
func GithubErr(err error, acct config.Account, doing string) error {
	d := github.Describe(err, acct.ID(), acct.Name, doing)
	var rl *github.RateLimitError
	switch {
	case errors.Is(err, github.ErrUnauthorized):
		return NewErr(KindInvalid, "auth.unauthorized", d, "account", acct.Name)
	case errors.Is(err, github.ErrForbidden):
		return NewErr(KindInvalid, "github.forbidden", d, "account", acct.Name)
	case errors.Is(err, github.ErrNotFound):
		return NewErr(KindInvalid, "github.not_found", d, "account", acct.Name)
	case errors.As(err, &rl):
		return NewErr(KindInvalid, "github.rate_limit", d, "account", acct.Name)
	}
	return d
}

// UpdateConfig is c.Update, after the BeforeUpdate hook of the session (a
// test writes config.yaml there, as another process could). A change makes
// the session forget the stores it mounted.
func (ss *Session) UpdateConfig(c *config.Config, fn func(*config.Config) error) error {
	if ss.BeforeUpdate != nil {
		ss.BeforeUpdate()
	}
	if err := c.Update(fn); err != nil {
		return err
	}
	// the stores and the prefixes they share may have changed
	ss.DropMount()
	return nil
}

func StoreExists(c *config.Config, name string) bool {
	for _, e := range c.Stores {
		if e.Name == name {
			return true
		}
	}
	return false
}

// CheckPrefixElsewhere refuses an id prefix that a workspace of another store
// of the config has; self is the store that is not looked at. A store that
// cannot be opened is skipped with a warning.
func (ss *Session) CheckPrefixElsewhere(prefix, self string) error {
	c, _, err := ss.Config()
	if err != nil {
		return err
	}
	var others []store.NamedStore
	for _, e := range c.Stores {
		if e.Name != self {
			others = append(others, store.NamedStore{Name: e.Name, Path: e.Path})
		}
	}
	uses := store.PrefixUses(others, []string{prefix}, func(name string, err error) {
		ss.AddWarning(fmt.Sprintf("store %s is not checked for id prefixes: %v", name, err))
	})
	if len(uses) > 0 {
		return NewErr(KindConflict, "workspace.prefix_taken",
			fmt.Errorf("id prefix %s is used by workspace %s of store %s", prefix, uses[0].Workspace.Name, uses[0].Store), "prefix", prefix)
	}
	return nil
}

func BadWorkspaceName(name string) error {
	if err := model.ValidWorkspaceName(name); err != nil {
		return NewErr(KindUsage, "workspace.bad_name", err, "value", name)
	}
	return nil
}

func BadIDPrefix(prefix string) error {
	if err := model.ValidIDPrefix(prefix); err != nil {
		return NewErr(KindUsage, "workspace.bad_prefix", err, "value", prefix)
	}
	return nil
}
