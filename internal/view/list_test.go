package view

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func writeView(t *testing.T, dir, file, data string) {
	t.Helper()
	p := filepath.Join(dir, "views", file)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListAndFind(t *testing.T) {
	dir := t.TempDir()
	if es, err := List(dir); err != nil || len(es) != 0 {
		t.Fatalf("List without views/ = %v, %v", es, err)
	}
	writeView(t, dir, "b.yaml", "layout: tree\n")
	writeView(t, dir, "a.yaml", "layout: table\n")
	writeView(t, dir, "broken.yaml", "- x\n")
	// Windows cannot create CON.yaml.
	con := runtime.GOOS != "windows"
	if con {
		writeView(t, dir, "CON.yaml", "layout: tree\n")
	}
	writeView(t, dir, "notes.txt", "x")
	writeView(t, dir, "sub/c.yaml", "layout: tree\n")
	es, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name)
	}
	want := []string{"a", "b", "broken"}
	if con {
		want = []string{"CON", "a", "b", "broken"}
	}
	if !slices.Equal(names, want) {
		t.Fatalf("names = %q; want %q", names, want)
	}
	i := 0
	if con {
		i = 1
		if es[0].Err == nil {
			t.Errorf("CON entry = %+v; want an error", es[0])
		}
	}
	if es[i].Err != nil || es[i].View.Layout != "table" || es[i].Rel != "views/a.yaml" || es[i+2].Err == nil {
		t.Errorf("entries = %+v", es)
	}
	if e, ok, err := Find(dir, "b"); err != nil || !ok || e.View.Layout != "tree" {
		t.Errorf("Find b = %+v %v %v", e, ok, err)
	}
	// Find matches the name exactly, even where the file system folds case.
	if _, ok, err := Find(dir, "B"); err != nil || ok {
		t.Errorf("Find B = %v %v; want not found", ok, err)
	}
	if _, ok, _ := Find(dir, "broken"); !ok {
		t.Error("Find gives a broken view too, with its error")
	}
}

func TestListUnreadableDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "views"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := List(dir); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Errorf("List with a file at views = %v; want an error", err)
	}
}
