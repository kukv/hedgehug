package notifications_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// addMovedItem puts into org-a's inbox an overwritten value of the task that
// MovedFixture moved away.
func addMovedItem(t *testing.T, ts *usecasetest.Session, ulid model.ULID, path, field string) model.ULID {
	t.Helper()
	s, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	added, err := inbox.Open(s).Add([]inbox.Item{{
		Kind: inbox.Overwritten, Task: ulid, Path: path, Commit: "c1", To: "x",
		Time:   time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
		Detail: json.RawMessage(`{"field":"` + field + `","value":"\"old\""}`),
	}})
	if err != nil || len(added) != 1 {
		t.Fatalf("Add item = %v, %v", added, err)
	}
	return added[0].ULID
}

func TestInboxListSaysWhereAMovedTaskIs(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	ulid := usecasetest.MovedFixture(t, ts)
	addMovedItem(t, ts, ulid, model.TaskPath(ulid), "*")
	_, items, err := notifications.InboxList(ts.ForStore("org-a"), false)
	if err != nil || len(items) != 1 {
		t.Fatalf("InboxList = %v, %v", items, err)
	}
	if want := " → apply it to org-b's WEB-2"; !strings.HasSuffix(items[0].Content, want) {
		t.Errorf("content = %q; want it to end with %q", items[0].Content, want)
	}
	if !strings.HasPrefix(items[0].Target, "API-1") {
		t.Errorf("target = %q; want the id the task had", items[0].Target)
	}
}

func TestInboxListSaysTheStoreOfAMovedTaskNotMounted(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	ulid := usecasetest.MovedFixture(t, ts)
	addMovedItem(t, ts, ulid, model.CommentPath(ulid, model.NewULID()), "*")
	bs, err := ts.ForStore("org-b").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("org-b"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.DropMount()
	_, items, err := notifications.InboxList(ts.ForStore("org-a"), false)
	if err != nil || len(items) != 1 {
		t.Fatalf("InboxList = %v, %v", items, err)
	}
	if want := " → apply it in the store " + bs.Meta().ULID.String(); !strings.HasSuffix(items[0].Content, want) {
		t.Errorf("content = %q; want it to end with %q", items[0].Content, want)
	}
}

func TestInboxRevertOfAMovedTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	ulid := usecasetest.MovedFixture(t, ts)
	item := addMovedItem(t, ts, ulid, model.TaskPath(ulid), "title")
	_, err := notifications.InboxRevert(ts.ForStore("org-a"), item.String())
	var oe *usecase.Error
	if !errors.As(err, &oe) || oe.Kind != usecase.KindNotFound || oe.Code != "task.moved" {
		t.Fatalf("err = %v; want KindNotFound task.moved", err)
	}
	if want := "API-1 moved to org-b; fix it there (org-b:WEB-2)"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	// nothing was written, and the item is not marked read
	if _, items, _ := notifications.InboxList(ts.ForStore("org-a"), false); len(items) != 1 {
		t.Errorf("unread items = %d; want 1", len(items))
	}
}

func TestRepairedContentOfMoves(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	ulid := usecasetest.MovedFixture(t, ts)
	ix := usecasetest.MustIndex(t, ts, "org-a")
	item := func(kind, value string) inbox.Item {
		return inbox.Item{Kind: inbox.Repaired, Detail: json.RawMessage(`{"kind":"` + kind + `","value":` + value + `}`)}
	}
	for _, c := range []struct{ kind, value, want string }{
		{"parent_moved", `{"from":"` + ulid.String() + `","to":null}`, "moved to the root because its parent moved to another store"},
		{"ref_moved", `{"field":"blocked_by","target":"` + ulid.String() + `"}`, "blocked_by to API-1 was removed because it moved to another store"},
		{"ref_moved", `{"field":"props.relates","target":"` + ulid.String() + `"}`, "props.relates to API-1 was removed because it moved to another store"},
	} {
		if got := usecase.RepairedContent(ix, item(c.kind, c.value)); got != c.want {
			t.Errorf("%s: content = %q; want %q", c.kind, got, c.want)
		}
	}
	gone := model.NewULID().String()
	if got := usecase.RepairedContent(nil, item("ref_moved", `{"field":"blocked_by","target":"`+gone+`"}`)); got != "blocked_by to "+gone+" was removed because it moved to another store" {
		t.Errorf("without an index: %q", got)
	}
}
