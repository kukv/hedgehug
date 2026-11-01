package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	t.Parallel()
	var warns []string
	st := Load(t.TempDir(), func(m string) { warns = append(warns, m) })
	if st.Use != nil || len(st.Recent) != 0 || len(warns) != 0 {
		t.Errorf("Load = %+v, warns %v; want empty and quiet", st, warns)
	}
}

func TestLoadBrokenIsEmptyAndWarns(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "context.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var warns []string
	st := Load(d, func(m string) { warns = append(warns, m) })
	if st.Use != nil || len(st.Recent) != 0 {
		t.Errorf("Load = %+v; want empty", st)
	}
	if len(warns) != 1 || !strings.HasPrefix(warns[0], "context.json is broken, so it is read as empty: ") {
		t.Errorf("warns = %q", warns)
	}
}

func TestSaveThenLoad(t *testing.T) {
	t.Parallel()
	d := filepath.Join(t.TempDir(), "data")
	want := State{
		Use:    &Ref{Store: model.NewULID(), Project: model.NewULID()},
		Recent: map[string]Ref{"github.com/org-a/api": {Store: model.NewULID(), Project: model.NewULID()}},
	}
	if err := Save(d, want); err != nil {
		t.Fatal(err)
	}
	got := Load(d, func(m string) { t.Errorf("warned: %s", m) })
	if got.Use == nil || *got.Use != *want.Use || got.Recent["github.com/org-a/api"] != want.Recent["github.com/org-a/api"] {
		t.Errorf("Load = %+v; want %+v", got, want)
	}
	ents, err := os.ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != "context.json" {
		t.Errorf("files = %v; want only context.json", ents)
	}
}

func TestSaveEmptyWritesNullUse(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	if err := Save(d, State{}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d, "context.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"use": null`) || !strings.Contains(string(b), `"recent": {}`) {
		t.Errorf("context.json = %s", b)
	}
}

func TestLoadInvalidULIDIsBroken(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "context.json"), []byte(`{"use": {"store": "nope", "project": "nope"}, "recent": {}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var warns []string
	st := Load(d, func(m string) { warns = append(warns, m) })
	if st.Use != nil || len(st.Recent) != 0 {
		t.Errorf("Load = %+v; want empty", st)
	}
	if len(warns) != 1 || !strings.HasPrefix(warns[0], "context.json is broken, so it is read as empty: ") {
		t.Errorf("warns = %q", warns)
	}
}

func TestLoadUnreadableIsNotBroken(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	// a directory where the file should be cannot be read, on every system
	if err := os.Mkdir(filepath.Join(d, "context.json"), 0o750); err != nil {
		t.Fatal(err)
	}
	var warns []string
	st := Load(d, func(m string) { warns = append(warns, m) })
	if st.Use != nil || len(st.Recent) != 0 {
		t.Errorf("Load = %+v; want empty", st)
	}
	if len(warns) != 1 || !strings.HasPrefix(warns[0], "context.json cannot be read, so it is read as empty: ") {
		t.Errorf("warns = %q", warns)
	}
}
