package planning

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// releaseStates are the states a release takes.
var releaseStates = []string{model.ReleaseUnreleased, model.ReleaseReleased, model.ReleaseArchived}

// ReleaseView is a release with what its tasks add up to.
type ReleaseView struct {
	ULID      string               `json:"ulid"`
	Workspace usecase.WorkspaceRef `json:"workspace"`
	Name      string               `json:"name"`
	Date      string               `json:"date,omitempty"`
	State     string               `json:"state"`
	Tasks     int                  `json:"tasks" doc:"The tasks in the release, not deleted"`
	Done      int                  `json:"done" doc:"Those of them in the done category"`
}

// ReleaseWrite is what a write to a release answers.
type ReleaseWrite struct {
	Release ReleaseView `json:"release"`
	Commit  *string     `json:"commit"`
}

func inRelease(e *index.Entry, r model.ULID) bool {
	return slices.Contains(e.Releases, r)
}

func releaseViewOf(ix *index.Index, r *model.Release) ReleaseView {
	v := ReleaseView{ULID: r.ULID.String(), Workspace: workspaceRef(ix, r.Workspace), Name: r.Name, Date: dateText(r.Date), State: r.State}
	sc := ix.Schema()
	for _, e := range ix.Tasks() {
		if !inRelease(e, r.ULID) || e.DeletedAt != nil {
			continue
		}
		v.Tasks++
		if st, ok := sc.Status(e.Status); ok && st.Category == schema.CategoryDone {
			v.Done++
		}
	}
	return v
}

func writtenRelease(ix *index.Index, r *model.Release, hash string) ReleaseWrite {
	_ = ix.Refresh() // the commit is made; see written
	return ReleaseWrite{Release: releaseViewOf(ix, r), Commit: usecase.CommitPtr(hash)}
}

func checkReleaseName(ix *index.Index, ws, self model.ULID, name string) error {
	if r, ok := ix.ReleaseNamed(ws, name); ok && r.ULID != self {
		return usecase.NewErr(usecase.KindConflict, "release.name_taken", fmt.Errorf("the workspace has a release called %s", r.Name), "name", name)
	}
	return nil
}

// ReleaseAdd adds an unreleased release.
func ReleaseAdd(ss *usecase.Session, name, date, workspace string) (ReleaseWrite, error) {
	if err := badName("release", name); err != nil {
		return ReleaseWrite{}, err
	}
	d, err := parseDate("date", date)
	if err != nil {
		return ReleaseWrite{}, err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return ReleaseWrite{}, err
	}
	ws, err := writeWorkspace(ss, ix, workspace)
	if err != nil {
		return ReleaseWrite{}, err
	}
	if err := checkReleaseName(ix, ws, model.ULID{}, name); err != nil {
		return ReleaseWrite{}, err
	}
	r := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: name, Date: d, State: model.ReleaseUnreleased}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "release-add", Workspace: ws, Detail: name}, func(tx *store.Tx) error {
		return tx.PutRelease(r)
	})
	if err != nil {
		return ReleaseWrite{}, err
	}
	return writtenRelease(ix, r, usecase.CommitHex(hash)), nil
}

// ReleaseList is SprintList for releases.
func ReleaseList(ss *usecase.Session, workspace, state string) ([]ReleaseView, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return nil, err
	}
	ws, err := pickWorkspace(ss, ix, workspace)
	if err != nil {
		return nil, err
	}
	out := []ReleaseView{}
	for _, r := range ix.Releases(ws) {
		if state == "" || r.State == state {
			out = append(out, releaseViewOf(ix, r))
		}
	}
	return out, nil
}

// ReleaseSet changes the name, the date or the state of a release.
func ReleaseSet(ss *usecase.Session, name, workspace string, assigns []usecase.Assign) (ReleaseWrite, error) {
	if len(assigns) == 0 {
		return ReleaseWrite{}, usecase.NewErr(usecase.KindUsage, "release.nothing_to_set", errors.New("nothing to change"))
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return ReleaseWrite{}, err
	}
	found, err := findRelease(ss, ix, name, workspace)
	if err != nil {
		return ReleaseWrite{}, err
	}
	var r *model.Release
	hash, err := ss.UpdateCommit(s, store.Op{Name: "release-set", Workspace: found.Workspace, Detail: found.Name}, func(tx *store.Tx) error {
		var err error
		if r, err = tx.Release(found.ULID); err != nil {
			return err
		}
		for _, a := range assigns {
			if err := applyRelease(r, a); err != nil {
				return err
			}
		}
		if err := checkReleaseName(ix, r.Workspace, r.ULID, r.Name); err != nil {
			return err
		}
		return tx.PutRelease(r)
	})
	if err != nil {
		return ReleaseWrite{}, err
	}
	return writtenRelease(ix, r, usecase.CommitHex(hash)), nil
}

func applyRelease(r *model.Release, a usecase.Assign) error {
	if a.Op != "=" {
		return usecase.NewErr(usecase.KindInvalid, "assign.single", fmt.Errorf("%s holds one value; use %s=<value>", a.Key, a.Key), "key", a.Key)
	}
	var err error
	switch a.Key {
	case "name":
		if err = badName("release", a.Value); err == nil {
			r.Name = a.Value
		}
	case "date":
		r.Date, err = parseDate(a.Key, a.Value)
	case "state":
		if !slices.Contains(releaseStates, a.Value) {
			return usecase.NewErr(usecase.KindInvalid, "assign.bad_value", fmt.Errorf("state: %q is not one of %s", a.Value, strings.Join(releaseStates, ", ")), "key", a.Key, "value", a.Value)
		}
		r.State = a.Value
	default:
		err = usecase.NewErr(usecase.KindInvalid, "assign.unknown_key", fmt.Errorf("%s is not name, date or state", a.Key), "key", a.Key)
	}
	return err
}

// ReleaseNotes writes the done tasks of a release in Markdown, a section for
// each kind in the order of the schema.
func ReleaseNotes(ss *usecase.Session, name, workspace string) (string, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return "", err
	}
	r, err := findRelease(ss, ix, name, workspace)
	if err != nil {
		return "", err
	}
	sc := ix.Schema()
	done := map[string][]*index.Entry{}
	for _, e := range ix.Tasks() {
		if st, ok := sc.Status(e.Status); ok && st.Category == schema.CategoryDone && inRelease(e, r.ULID) && e.DeletedAt == nil {
			done[e.Kind] = append(done[e.Kind], e)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", r.Name)
	section := func(title string, es []*index.Entry) {
		slices.SortFunc(es, func(a, b *index.Entry) int {
			return strings.Compare(string(a.Rank)+a.ULID.String(), string(b.Rank)+b.ULID.String())
		})
		fmt.Fprintf(&b, "\n## %s\n\n", title)
		for _, e := range es {
			fmt.Fprintf(&b, "- %s %s\n", ix.DisplayID(e), e.Title)
		}
	}
	for _, k := range sc.Kinds {
		if es := done[k.Name]; len(es) > 0 {
			section(kindTitle(k), es)
			delete(done, k.Name)
		}
	}
	rest := make([]string, 0, len(done))
	for k := range done {
		rest = append(rest, k)
	}
	slices.Sort(rest)
	for _, k := range rest {
		section(k, done[k])
	}
	return b.String(), nil
}

func kindTitle(k schema.Kind) string {
	if k.Label != "" {
		return k.Label
	}
	return k.Name
}

// ReleaseRemove takes the release off its tasks and deletes it, in one
// commit, as SprintRemove does.
func ReleaseRemove(ss *usecase.Session, name, workspace string) (ReleaseWrite, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return ReleaseWrite{}, err
	}
	r, err := findRelease(ss, ix, name, workspace)
	if err != nil {
		return ReleaseWrite{}, err
	}
	view := releaseViewOf(ix, r)
	hash, err := ss.UpdateCommit(s, store.Op{Name: "release-rm", Workspace: r.Workspace, Detail: r.Name}, func(tx *store.Tx) error {
		for _, e := range ix.Tasks() {
			if !inRelease(e, r.ULID) {
				continue
			}
			t, err := tx.Task(e.ULID)
			if err != nil {
				return err
			}
			t.Releases = slices.DeleteFunc(t.Releases, func(u model.ULID) bool { return u == r.ULID })
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return tx.RemoveFile(model.ReleasePath(r.ULID))
	})
	if err != nil {
		return ReleaseWrite{}, err
	}
	return ReleaseWrite{Release: view, Commit: usecase.CommitPtr(usecase.CommitHex(hash))}, nil
}
