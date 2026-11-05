package usecase

import (
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
)

// SharesTokensWith reports whether ss and o use one token cache.
func (ss *Session) SharesTokensWith(o *Session) bool {
	return ss.tokens != nil && ss.tokens == o.tokens
}

// WriterCached reports whether ss has read the writer and keeps it.
func (ss *Session) WriterCached() bool { return ss.writerDone }

type DupPlace = dupPlace

func (ss *Session) GithubRepoOf(a0 *config.Config, a1 config.StoreEntry) *replicate.GitHubRepo {
	return ss.githubRepoOf(a0, a1)
}

var ProjectOf = projectOf

// ResultIndexHas reports whether the index the result holds, as it is before
// View refreshes it, has the task u.
func ResultIndexHas(r TaskResult, u model.ULID) bool {
	_, ok := r.index.Task(u)
	return ok
}

// AutoLinks lets the external test call autoLinks.
var AutoLinks = autoLinks
