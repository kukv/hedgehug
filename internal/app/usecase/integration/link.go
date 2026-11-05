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

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

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

func (in LinkInput) sync(s *store.Store) (string, error) {
	invalid := func(err error) (string, error) { return "", usecase.NewErr(usecase.KindUsage, "link.invalid", err) }
	switch {
	case in.PR != "" && in.Sync != "":
		return invalid(errors.New("--sync is for an issue: GitHub is always the source of a pull request"))
	case in.PR != "":
		return model.SyncMirror, nil
	case in.Sync == "":
		return defaultIssueSync(s)
	case slices.Contains([]string{model.SyncMirror, model.SyncPush, model.SyncLink}, in.Sync):
		return in.Sync, nil
	}
	return invalid(fmt.Errorf("--sync: %q is not mirror, push or link", in.Sync))
}

// defaultIssueSync is the mode of a new link to an issue that names none:
// issue_fields.default_sync of automations.yaml. The mode stays in the task,
// so it is not guessed when the file cannot be used.
func defaultIssueSync(s *store.Store) (string, error) {
	f, err := s.Automations()
	if err != nil {
		return "", usecase.NewErr(usecase.KindInvalid, "automation.invalid", err)
	}
	mode := f.IssueFields.DefaultSync
	if !slices.Contains(automation.SyncModes, mode) {
		return "", usecase.NewErr(usecase.KindInvalid, "automation.invalid",
			fmt.Errorf("%s: issue_fields.default_sync: %q is not mirror, push or link", model.AutomationsPath, mode))
	}
	return mode, nil
}

// dropGH takes what gh holds of ref off t. A link that comes in or goes away
// starts over: undo leaves gh as it is, so the item of an old link may be there.
func dropGH(t *model.Task, ref model.GitHubRef) error {
	g, err := t.GitHub()
	if err != nil {
		return err
	}
	g.PRs = slices.DeleteFunc(g.PRs, func(p model.PRState) bool { return model.CanonRef(p.Ref) == ref.String() })
	g.Issues = slices.DeleteFunc(g.Issues, func(x model.IssueState) bool { return model.CanonRef(x.Ref) == ref.String() })
	return t.SetGitHub(g)
}

// link adds the link to ref to t. Linking again what is unlinked brings it
// back; linking an issue again with another Sync changes its mode.
func (in LinkInput) link(s *store.Store, t *model.Task, ref model.GitHubRef) error {
	sync, err := in.sync(s)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(t.Links, func(l model.Link) bool { return model.CanonRef(l.Ref) == ref.String() })
	if i < 0 {
		t.Links = append(slices.Clone(t.Links), model.Link{Type: ref.Kind, Ref: ref.String(), Sync: sync})
		return dropGH(t, ref)
	}
	live := t.Links[i].Live()
	if live && (in.Sync == "" || t.Links[i].Sync == sync) {
		return usecase.NewErr(usecase.KindInvalid, "link.exists", fmt.Errorf("%s is linked already", ref.Label()))
	}
	t.Links = slices.Clone(t.Links)
	t.Links[i].Sync = sync
	if !live {
		t.Links[i].Via = "" // this link is of a person now
	}
	if live {
		return dropSynced(t, ref) // what the old mode compared is no base for the new one
	}
	return dropGH(t, ref)
}

// dropSynced takes the base of ref off t. What GitHub says of it stays.
func dropSynced(t *model.Task, ref model.GitHubRef) error {
	g, err := t.GitHub()
	if err != nil {
		return err
	}
	for i := range g.Issues {
		if model.CanonRef(g.Issues[i].Ref) == ref.String() {
			g.Issues[i].Synced = nil
		}
	}
	return t.SetGitHub(g)
}

// Link links the task arg names.
func Link(ss *usecase.Session, arg string, in LinkInput) (usecase.TaskResult, error) {
	return change(ss, arg, "link", in, in.link)
}

// Unlink turns a link off. It stays in the file, so that the automatic
// linking does not link the pull request again; what gh holds of it goes, so
// that linking it again is a new item.
func Unlink(ss *usecase.Session, arg string, in LinkInput) (usecase.TaskResult, error) {
	return change(ss, arg, "unlink", in, func(_ *store.Store, t *model.Task, ref model.GitHubRef) error {
		i := slices.IndexFunc(t.Links, func(l model.Link) bool { return model.CanonRef(l.Ref) == ref.String() && l.Live() })
		if i < 0 {
			return usecase.NewErr(usecase.KindInvalid, "link.not_found", fmt.Errorf("%s is not linked", ref.Label()))
		}
		t.Links = slices.Clone(t.Links)
		t.Links[i].Sync = model.SyncOff
		return dropGH(t, ref)
	})
}

func change(ss *usecase.Session, arg, op string, in LinkInput, fn func(*store.Store, *model.Task, model.GitHubRef) error) (usecase.TaskResult, error) {
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
	return writeLink(ss, s, ix, e, op, ref, fn)
}

// writeLink runs fn on the task e, as one commit.
func writeLink(ss *usecase.Session, s *store.Store, ix *index.Index, e *index.Entry, op string, ref model.GitHubRef, fn func(*store.Store, *model.Task, model.GitHubRef) error) (usecase.TaskResult, error) {
	hash, err := ss.UpdateCommit(s, store.Op{Name: op, Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		if err := fn(s, t, ref); err != nil {
			return err
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}
