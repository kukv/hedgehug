package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/convert"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// oldStoreEnv is a store "me" in format 1: a task of the kind task at the
// root of its workspace, a project, and the kinds in schema.yaml.
func oldStoreEnv(t *testing.T) *testEnv {
	t.Helper()
	te := newStoreEnv(t)
	te.ok("add", "Web")
	te.ok("add", "Loose")
	te.handEdit("me", "ME-2", "\nkind: project\n", "\nkind: task\n")
	dir := te.storeDir("me")
	schemaPath := filepath.Join(dir, "schema.yaml")
	b, err := os.ReadFile(schemaPath) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	old := string(b) + "kinds:\n  - {name: project, role: container}\n  - {name: task, role: work, default: true}\n"
	if err := os.WriteFile(schemaPath, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(dir, ".hedgehug", "store.yaml")
	m, err := os.ReadFile(metaPath) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, []byte(strings.Replace(string(m), fmt.Sprintf("format: %d", model.StoreFormat), "format: 1", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	gitrepotest.CommitFiles(t, dir, "make it old", "Alice", "alice@example.com", te.now, "schema.yaml", ".hedgehug/store.yaml")
	return te
}

func TestOldFormatStopsTheCommands(t *testing.T) {
	t.Parallel()
	te := oldStoreEnv(t)
	for _, args := range [][]string{{"ls"}, {"add", "x"}, {"show", "ME-1"}} {
		code, _, errs := te.run(args...)
		if code != 1 || !strings.Contains(errs, "hh store convert で新しい形式にしてください") {
			t.Errorf("hedgehug %s: exit %d, stderr %q", strings.Join(args, " "), code, errs)
		}
	}
	code, out, _ := te.run("doctor")
	if code != 1 || !strings.Contains(out, "format 1") || !strings.Contains(out, "hh store convert") {
		t.Errorf("doctor: exit %d, stdout %q", code, out)
	}
	te.goldenRun("ls-json", 1, "ls", "--json")
}

func TestStoreConvert(t *testing.T) {
	t.Parallel()
	te := oldStoreEnv(t)
	te.goldenOK("dry-run", "store", "convert", "--dry-run")
	if code, _, _ := te.run("ls"); code != 1 {
		t.Errorf("a dry run converted: ls exits %d", code)
	}
	te.goldenOK("convert", "store", "convert")
	te.goldenOK("ls", "ls")
	te.goldenOK("again", "store", "convert")
	te.goldenOK("again-json", "store", "convert", "--json")
	if out := te.ok("doctor"); out != "No problems\n" {
		t.Errorf("doctor after the conversion: %q", out)
	}
}

func TestStoreConvertJSON(t *testing.T) {
	t.Parallel()
	te := oldStoreEnv(t)
	var got convert.Result
	if err := json.Unmarshal([]byte(te.ok("store", "convert", "--dry-run", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if !got.DryRun || len(got.Steps) != 2 || got.Steps[0].From != 1 || len(got.Steps[0].Lines) == 0 {
		t.Errorf("result = %+v", got)
	}
}

func TestStoreConvertKindFlag(t *testing.T) {
	t.Parallel()
	te := oldStoreEnv(t)
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"no equals", []string{"store", "convert", "--kind", "task"}, 2, "--kind wants <old>=<new>"},
		{"not a kind", []string{"store", "convert", "--kind", "task=sprint"}, 2, "sprint is not a kind"},
		{"not in the store", []string{"store", "convert", "--kind", "nope=task"}, 2, "has no kind nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errs := te.run(tc.args...)
			if code != tc.code || !strings.Contains(errs, tc.want) {
				t.Errorf("exit %d, stderr %q; want exit %d with %q", code, errs, tc.code, tc.want)
			}
		})
	}
	te.ok("store", "convert", "--kind", "task=bug")
	if out := te.ok("ls"); !strings.Contains(out, "Loose") {
		t.Errorf("ls: %q", out)
	}
}

func TestStoreConvertDropBadValues(t *testing.T) {
	t.Parallel()
	te := oldStoreEnv(t)
	dir := te.storeDir("me")
	schemaPath := filepath.Join(dir, "schema.yaml")
	b, err := os.ReadFile(schemaPath) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	// a task_ref property of format 2, and a value of it that is no ULID
	if err := os.WriteFile(schemaPath, []byte(string(b)+"props:\n  fixes: {type: task_ref}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitrepotest.CommitFiles(t, dir, "a task_ref", "Alice", "alice@example.com", te.now, "schema.yaml")
	te.handEdit("me", "ME-1", "\nkind: project\n", "\nkind: project\nprops:\n  fixes: nope\n")

	for _, args := range [][]string{{"store", "convert", "--dry-run"}, {"store", "convert"}} {
		code, _, errs := te.run(args...)
		if code != 1 || !strings.Contains(errs, "props.fixes: nope") || !strings.Contains(errs, "--drop-bad-values") {
			t.Errorf("hedgehug %s: exit %d, stderr %q", strings.Join(args, " "), code, errs)
		}
	}
	if out := te.ok("store", "convert", "--dry-run", "--drop-bad-values"); !strings.Contains(out, "ULID の形でない値を外した") || !strings.Contains(out, "props.fixes: nope") {
		t.Errorf("dry run with --drop-bad-values: %q", out)
	}
	if code, _, _ := te.run("ls"); code != 1 {
		t.Errorf("a dry run converted: ls exits %d", code)
	}
	te.ok("store", "convert", "--drop-bad-values")
	if out := te.ok("doctor"); out != "No problems\n" {
		t.Errorf("doctor after the conversion: %q", out)
	}
}
