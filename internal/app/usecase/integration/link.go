// Package integration ties tasks to pull requests and issues on GitHub: it
// writes and drops the links. Reading GitHub's values into gh is the sync's,
// in package usecase (FetchGitHub).
package integration

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// defaultIssueSync is the mode of a new link to an issue. Phase 5c reads it
// from issue_fields.default_sync of automations.yaml.
const defaultIssueSync = model.SyncLink

// LinkInput names what to link: PR or Issue, as [<host>/]owner/repo#n, and
// for an issue how it syncs.
type LinkInput struct {
	PR, Issue string
	Sync      string
}

var written = regexp.MustCompile(`^(?:([^/#]+)/)?([^/#]+)/([^/#]+)#([1-9][0-9]*)$`)

// ref reads in into a ref. host is the writer's, for a ref without one.
func (in LinkInput) ref(host string) (model.GitHubRef, error) {
	invalid := func(err error) (model.GitHubRef, error) {
		return model.GitHubRef{}, usecase.NewErr(usecase.KindUsage, "link.invalid", err)
	}
	s, kind := in.PR, model.LinkPR
	switch {
	case in.PR == "" && in.Issue == "":
		return invalid(errors.New("say what to link: --pr or --issue"))
	case in.PR != "" && in.Issue != "":
		return invalid(errors.New("say one of --pr and --issue"))
	case in.Issue != "":
		s, kind = in.Issue, model.LinkIssue
	}
	m := written.FindStringSubmatch(s)
	if m == nil {
		return invalid(fmt.Errorf("%q: want owner/repo#number", s))
	}
	if m[1] != "" {
		host = m[1]
	}
	if host == "" {
		host = "github.com"
	}
	n, err := strconv.Atoi(m[4])
	if err != nil {
		return invalid(fmt.Errorf("%q: %w", s, err))
	}
	return model.GitHubRef{Repo: strings.ToLower(host + "/" + m[2] + "/" + m[3]), Kind: kind, Number: n}, nil
}

func (in LinkInput) sync() (string, error) {
	invalid := func(err error) (string, error) { return "", usecase.NewErr(usecase.KindUsage, "link.invalid", err) }
	switch {
	case in.PR != "" && in.Sync != "":
		return invalid(errors.New("--sync is for an issue: GitHub is always the source of a pull request"))
	case in.PR != "":
		return model.SyncMirror, nil
	case in.Sync == "":
		return defaultIssueSync, nil
	case slices.Contains([]string{model.SyncMirror, model.SyncPush, model.SyncLink}, in.Sync):
		return in.Sync, nil
	}
	return invalid(fmt.Errorf("--sync: %q is not mirror, push or link", in.Sync))
}

// Link links the task arg names. Linking again what is unlinked brings it
// back; linking an issue again with another Sync changes its mode.
func Link(ss *usecase.Session, arg string, in LinkInput) (usecase.TaskResult, error) {
	return change(ss, arg, "link", in, func(links []model.Link, ref model.GitHubRef) ([]model.Link, error) {
		sync, err := in.sync()
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(links, func(l model.Link) bool { return model.CanonRef(l.Ref) == ref.String() })
		if i < 0 {
			return append(slices.Clone(links), model.Link{Type: ref.Kind, Ref: ref.String(), Sync: sync}), nil
		}
		if links[i].Live() && (in.Sync == "" || links[i].Sync == sync) {
			return nil, usecase.NewErr(usecase.KindInvalid, "link.exists", fmt.Errorf("%s is linked already", ref.Label()))
		}
		out := slices.Clone(links)
		out[i].Sync = sync
		return out, nil
	})
}

// Unlink turns a link off. It stays in the file, so that the automatic
// linking does not link the pull request again.
func Unlink(ss *usecase.Session, arg string, in LinkInput) (usecase.TaskResult, error) {
	return change(ss, arg, "unlink", in, func(links []model.Link, ref model.GitHubRef) ([]model.Link, error) {
		i := slices.IndexFunc(links, func(l model.Link) bool { return model.CanonRef(l.Ref) == ref.String() && l.Live() })
		if i < 0 {
			return nil, usecase.NewErr(usecase.KindInvalid, "link.not_found", fmt.Errorf("%s is not linked", ref.Label()))
		}
		out := slices.Clone(links)
		out[i].Sync = model.SyncOff
		return out, nil
	})
}

func change(ss *usecase.Session, arg, op string, in LinkInput, fn func([]model.Link, model.GitHubRef) ([]model.Link, error)) (usecase.TaskResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if ts != ss {
		return change(ts, arg, op, in, fn)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return usecase.TaskResult{}, err
	}
	host, err := ss.WriterHost()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	ref, err := in.ref(host)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: op, Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		if t.Links, err = fn(t.Links, ref); err != nil {
			return err
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}
