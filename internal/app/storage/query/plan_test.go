package query

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

func TestRunSprintAndRelease(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ws := firstWorkspace(t, s)
	other := &model.Workspace{ULID: model.NewULID(), Name: "other", IDPrefix: "OTH"}
	active1 := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "active one", State: model.SprintActive}
	active2 := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "a2", State: model.SprintActive}
	next := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "n1", State: model.SprintPlanned}
	later := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "n2", State: model.SprintPlanned}
	closed := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "c1", State: model.SprintClosed}
	sameName := &model.Sprint{ULID: model.NewULID(), Workspace: other.ULID, Name: "c1", State: model.SprintPlanned}
	v1 := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: "v1", State: model.ReleaseUnreleased}
	v2 := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: "v2", State: model.ReleaseUnreleased}
	d1, err := model.ParseDate("2026-11-01")
	if err != nil {
		t.Fatal(err)
	}
	d2, err := model.ParseDate("2026-12-01")
	if err != nil {
		t.Fatal(err)
	}
	next.Start, later.Start = d1, d2
	if _, err := s.Update(store.Op{Name: "plan", Author: testAuthor}, func(tx *store.Tx) error {
		if err := tx.PutWorkspace(other); err != nil {
			return err
		}
		for _, sp := range []*model.Sprint{active1, active2, next, later, closed, sameName} {
			if err := tx.PutSprint(sp); err != nil {
				return err
			}
		}
		for _, r := range []*model.Release{v1, v2} {
			if err := tx.PutRelease(r); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	in := func(title string, sp *model.Sprint, rels ...model.ULID) *model.Task {
		task := newTask(title)
		task.Releases = rels
		if sp != nil {
			task.Sprint = &sp.ULID
		}
		return task
	}
	otherTask := in("o", sameName)
	otherTask.Workspace = other.ULID
	put(t, s,
		in("a1", active1, v1.ULID), in("a2", active2, v1.ULID, v2.ULID), in("n", next), in("l", later), in("c", closed), in("none", nil),
		otherTask,
	)
	ix := mustOpen(t, s)

	for _, tc := range []struct {
		q    string
		want []string
	}{
		{"sprint:current", []string{"a1", "a2"}},
		{"sprint:next", []string{"n", "o"}},
		{"sprint:none", []string{"none"}},
		{"-sprint:none", []string{"a1", "a2", "c", "l", "n", "o"}},
		{`sprint:"active one"`, []string{"a1"}},
		{"sprint:A2", []string{"a2"}},
		{"sprint:n1,n2", []string{"l", "n"}},
		{"sprint:current,n1", []string{"a1", "a2", "n"}},
		{"sprint:c1", []string{"c", "o"}},
		{"sprint:nothing", nil},
		{"release:v1", []string{"a1", "a2"}},
		{"release:v2", []string{"a2"}},
		{"-release:none", []string{"a1", "a2"}},
		{"release:nothing", nil},
	} {
		got := run(t, ix, tc.q, Options{})[0]
		slices.Sort(got)
		if len(got) != len(tc.want) || (len(got) > 0 && !slices.Equal(got, tc.want)) {
			t.Errorf("%s = %v, want %v", tc.q, got, tc.want)
		}
	}
}

func TestRunSprintFileMissing(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: firstWorkspace(t, s), Name: "gone", State: model.SprintPlanned}
	if _, err := s.Update(store.Op{Name: "plan", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutSprint(sp) }); err != nil {
		t.Fatal(err)
	}
	a := newTask("a")
	a.Sprint = &sp.ULID
	put(t, s, a)
	if _, err := s.Update(store.Op{Name: "plan", Author: testAuthor}, func(tx *store.Tx) error { return tx.RemoveFile(model.SprintPath(sp.ULID)) }); err != nil {
		t.Fatal(err)
	}
	ix := mustOpen(t, s)
	if got := run(t, ix, "sprint:none", Options{})[0]; len(got) != 0 {
		t.Errorf("sprint:none matches a task whose sprint file is gone: %v", got)
	}
}
