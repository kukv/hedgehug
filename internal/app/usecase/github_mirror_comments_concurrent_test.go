package usecase_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// concurrentClones makes two clones of one store that mirror the comments of
// the same issue, and returns them with the task and the issue.
func concurrentClones(t *testing.T) (ts, other *usecasetest.Session, fake *fakegh.Server, x *model.Task, n int) {
	t.Helper()
	ts, fake = usecasetest.GitHubSession(t)
	remote := usecasetest.AddBareRemote(t, ts)
	devStore(t, ts, nil)
	x, n = mirrorTask(t, ts, fake)
	if r, err := replication.SyncOne(ts.Session); err != nil || r.Err != nil {
		t.Fatalf("sync: %+v, %v", r, err)
	}
	other = usecasetest.Clone(t, ts, remote, ts.Now())
	other.DropDefaultAccount()
	other.AppendConfig(hostsRE.FindString(usecasetest.ConfigText(t, ts)) +
		"accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	other.Renew()
	return ts, other, fake, x, n
}

// bothRead makes both clones read GitHub before either has the commit of the
// other, and then lets them sync until they agree. It checks that the comment
// files are the same bytes and that no one is told of an overwrite.
func bothRead(t *testing.T, ts, other *usecasetest.Session, x *model.Task) {
	t.Helper()
	later(ts, 5*time.Minute)
	later(other, 5*time.Minute+7*time.Second)
	for _, s := range []*usecasetest.Session{ts, other} {
		if w := fetch(t, s).Warnings; len(w) != 0 {
			t.Fatalf("warnings %v", w)
		}
	}
	for _, s := range []*usecasetest.Session{ts, other, ts, other} {
		if r, err := replication.SyncOne(s.Session); err != nil || r.Err != nil {
			t.Fatalf("sync: %+v, %v", r, err)
		}
	}
	a, b := commentsOf(t, ts, x.ULID), commentsOf(t, other, x.ULID)
	if len(a) != len(b) {
		t.Fatalf("comments %d and %d", len(a), len(b))
	}
	for i := range a {
		ea, _ := a[i].Encode()
		eb, _ := b[i].Encode()
		if string(ea) != string(eb) {
			t.Errorf("the clones differ:\n%s\n%s", ea, eb)
		}
	}
	for _, s := range []*usecasetest.Session{ts, other} {
		_, items, err := notifications.InboxList(s.Session, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			t.Errorf("inbox item %+v", it)
		}
	}
}

// Two clones that read the same new comment at the same time write the same
// file, so the merge has nothing to overwrite.
func TestMirrorCommentsTakenInAtTheSameTimeAreOne(t *testing.T) {
	t.Parallel()
	ts, other, fake, x, n := concurrentClones(t)
	if _, err := fake.AddComment("org-a/web", n, "bob", "hello"); err != nil {
		t.Fatal(err)
	}
	bothRead(t, ts, other, x)
	if cs := commentsOf(t, ts, x.ULID); len(cs) != 1 || cs[0].Body != "hello" {
		t.Errorf("comments %+v", cs)
	}
}

func TestMirrorCommentsEditedAtTheSameTimeAreOne(t *testing.T) {
	t.Parallel()
	ts, other, fake, x, n := concurrentClones(t)
	id, err := fake.AddComment("org-a/web", n, "bob", "hello")
	if err != nil {
		t.Fatal(err)
	}
	bothRead(t, ts, other, x)
	if err := fake.EditComment("org-a/web", id, "changed"); err != nil {
		t.Fatal(err)
	}
	bothRead(t, ts, other, x)
	if cs := commentsOf(t, ts, x.ULID); len(cs) != 1 || cs[0].Body != "changed" {
		t.Errorf("comments %+v", cs)
	}
}

func TestMirrorCommentsDeletedAtTheSameTimeAreOne(t *testing.T) {
	t.Parallel()
	ts, other, fake, x, n := concurrentClones(t)
	id, err := fake.AddComment("org-a/web", n, "bob", "hello")
	if err != nil {
		t.Fatal(err)
	}
	bothRead(t, ts, other, x)
	if err := fake.RemoveComment("org-a/web", id); err != nil {
		t.Fatal(err)
	}
	bothRead(t, ts, other, x)
	if cs := commentsOf(t, ts, x.ULID); len(cs) != 1 || cs[0].DeletedAt == nil {
		t.Errorf("comments %+v", cs)
	}
}
