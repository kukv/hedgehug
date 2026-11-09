package upgrade

import (
	"bytes"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

const oldSchema = `kinds:
  - {name: area, role: container}
  - {name: chore, role: work}
`

func taskFile(t *testing.T, kind string, parent *model.ULID) []byte {
	t.Helper()
	tk := &model.Task{
		ULID: model.NewULID(), ID: "ME-1", Workspace: model.NewULID(), Kind: kind, Title: "x", Status: "todo",
		Parent: parent, Created: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Updated: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
	}
	b, err := tk.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func kindOf(t *testing.T, b []byte) string {
	t.Helper()
	tk, err := model.DecodeTask(b)
	if err != nil {
		t.Fatal(err)
	}
	return tk.Kind
}

func TestTask1to2GivesAFixedKind(t *testing.T) {
	t.Parallel()
	parent := model.NewULID()
	for _, c := range []struct {
		name   string
		kind   string
		parent *model.ULID
		want   string
	}{
		{"a fixed kind stays", "story", nil, "story"},
		{"a custom group at the root is a project", "area", nil, "project"},
		{"a custom group below is an epic", "area", &parent, "epic"},
		{"a custom item is a task", "chore", &parent, "task"},
		{"a kind the schema does not list is a task", "gone", nil, "task"},
	} {
		raw := taskFile(t, c.kind, c.parent)
		got, err := Upgrade(1, raw, Cx{Schema: []byte(oldSchema)})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if kindOf(t, got) != c.want {
			t.Errorf("%s: kind = %q, want %q", c.name, kindOf(t, got), c.want)
		}
	}
}

func TestTask1to2KeepsUpdatedAndTheOtherBytes(t *testing.T) {
	t.Parallel()
	raw := taskFile(t, "chore", nil)
	got, err := Upgrade(1, raw, Cx{Schema: []byte(oldSchema)})
	if err != nil {
		t.Fatal(err)
	}
	was, _ := model.DecodeTask(raw)
	now, _ := model.DecodeTask(got)
	if !now.Updated.Equal(was.Updated) {
		t.Errorf("updated moved: %v to %v", was.Updated, now.Updated)
	}
	if want := strings.Replace(string(raw), "kind: chore", "kind: task", 1); string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	// a file that needs no change comes back as it is
	same := taskFile(t, "task", nil)
	if got, _ := Upgrade(1, same, Cx{Schema: []byte(oldSchema)}); !bytes.Equal(got, same) {
		t.Errorf("a task of a fixed kind changed: %q", got)
	}
}

func TestUpgradeLeavesAForwardAndNothingAlone(t *testing.T) {
	t.Parallel()
	f := &model.Forward{ULID: model.NewULID(), MovedTo: model.NewULID(), MovedAt: time.Now()}
	raw, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Upgrade(1, raw, Cx{}); err != nil || !bytes.Equal(got, raw) {
		t.Errorf("forward: %q, %v", got, err)
	}
	if got, err := Upgrade(1, nil, Cx{}); err != nil || got != nil {
		t.Errorf("nil: %q, %v", got, err)
	}
}

func TestUpgradeNeedsAStepForEveryFormat(t *testing.T) {
	t.Parallel()
	if _, err := To(7, 8, taskFile(t, "task", nil), Cx{}); err == nil || !strings.Contains(err.Error(), "no step") {
		t.Errorf("err = %v", err)
	}
	if _, err := Upgrade(1, []byte("not a task"), Cx{Schema: []byte(oldSchema)}); err == nil {
		t.Error("a file that is not a task upgraded")
	}
	if _, err := Upgrade(1, taskFile(t, "chore", nil), Cx{Schema: []byte("kinds: [")}); err == nil {
		t.Error("a schema that does not decode was used")
	}
}

// withFormat5 stands a fifth format in, whose step writes "(5)" after the
// title. The test must not be parallel.
func withFormat5(t *testing.T) {
	t.Helper()
	was, wasSteps := model.StoreFormat, Steps
	model.StoreFormat = 5
	Steps = append(slices.Clone(Steps), Step{From: 4, Apply: func(raw []byte, _ Cx) ([]byte, error) {
		tk, err := model.DecodeTask(raw)
		if err != nil {
			return nil, err
		}
		tk.Title += "(5)"
		return tk.Encode()
	}})
	t.Cleanup(func() { model.StoreFormat, Steps = was, wasSteps })
}

func TestUpgradeRunsTheStepsInOrderToTheNewestOrToAFormat(t *testing.T) {
	withFormat5(t)
	raw := taskFile(t, "chore", nil)
	cx := Cx{Schema: []byte(oldSchema)}
	all, err := Upgrade(1, raw, cx)
	if err != nil {
		t.Fatal(err)
	}
	if tk, _ := model.DecodeTask(all); tk.Kind != "task" || tk.Title != "x(5)" {
		t.Errorf("1 to the newest: kind %q, title %q", tk.Kind, tk.Title)
	}
	mid, err := To(1, 2, raw, cx)
	if err != nil {
		t.Fatal(err)
	}
	if tk, _ := model.DecodeTask(mid); tk.Kind != "task" || tk.Title != "x" {
		t.Errorf("1 to 2: kind %q, title %q", tk.Kind, tk.Title)
	}
	last, err := Upgrade(2, mid, cx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(last, all) {
		t.Errorf("1 to 2 then 2 to the newest differs from 1 to the newest: %q, %q", last, all)
	}
}

// files is a commit for the tests: path to content.
type files map[string]string

func (f files) read(p string) ([]byte, error) {
	if s, ok := f[p]; ok {
		return []byte(s), nil
	}
	return nil, fs.ErrNotExist
}

func TestCommitReadsTheFormatAndTheSchemaOfTheCommit(t *testing.T) {
	t.Parallel()
	meta := func(format string) string {
		return "ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: x\nformat: " + format + "\n"
	}
	raw := taskFile(t, "chore", nil)

	old := At(files{model.StoreMetaPath: meta("1"), schema.Path: oldSchema}.read)
	if f, err := old.Format(); f != 1 || err != nil {
		t.Errorf("format = %d, %v", f, err)
	}
	got, err := old.Task(raw)
	if err != nil || kindOf(t, got) != "task" {
		t.Errorf("task of format 1: kind %q, %v", kindOf(t, got), err)
	}

	// a commit in the newest format is read as it is, and needs no schema
	cur := At(files{model.StoreMetaPath: meta("2")}.read)
	if got, err := cur.Task(raw); err != nil || !bytes.Equal(got, raw) {
		t.Errorf("task of the newest format: %q, %v", got, err)
	}

	// no store.yaml, one that does not decode, one that cannot be read, and
	// one of a newer format: an error, never the newest format by guess
	boom := errors.New("boom")
	for name, read := range map[string]func(string) ([]byte, error){
		"missing": files{}.read,
		"broken":  files{model.StoreMetaPath: "format: ["}.read,
		"newer":   files{model.StoreMetaPath: meta("99")}.read,
		"io":      func(string) ([]byte, error) { return nil, boom },
	} {
		got, err := At(read).Task(raw)
		if err == nil || got != nil || !strings.Contains(err.Error(), model.StoreMetaPath) {
			t.Errorf("%s store.yaml: %q, %v; want an error naming store.yaml", name, got, err)
		}
	}
	if _, err := At(files{model.StoreMetaPath: meta("99")}.read).Format(); !errors.Is(err, model.ErrNewerFormat) {
		t.Errorf("newer format: %v", err)
	}

	// a schema that cannot be read is the error of the read
	failing := At(func(p string) ([]byte, error) {
		if p == schema.Path {
			return nil, boom
		}
		return files{model.StoreMetaPath: meta("1")}.read(p)
	})
	if _, err := failing.Task(raw); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}
