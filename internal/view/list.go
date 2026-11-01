package view

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
)

// ErrNotFound means no view has the name.
var ErrNotFound = errors.New("no such view")

// Entry is one file directly under views/ with the .yaml extension. Err
// tells why it is not a view: a name ValidName refuses, or a file that
// does not decode. View is nil then.
type Entry struct {
	Name string
	Rel  string // the path in the store, views/<name>.yaml
	View *View
	Err  error
}

// List reads the views of the store in storeDir, sorted by name. A store
// without views/ has none. Files in subdirectories, and files without the
// .yaml extension, are not views and are left out; doctor reports them.
func List(storeDir string) ([]Entry, error) {
	des, err := os.ReadDir(filepath.Join(storeDir, model.ViewsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, de := range des {
		rel := model.ViewsDir + "/" + de.Name()
		name, ok := model.IsViewPath(rel)
		if !ok || !de.Type().IsRegular() {
			continue
		}
		out = append(out, read(storeDir, name, rel))
	}
	slices.SortFunc(out, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func read(storeDir, name, rel string) Entry {
	e := Entry{Name: name, Rel: rel}
	if err := ValidName(name); err != nil {
		e.Err = err
		return e
	}
	data, err := os.ReadFile(filepath.Join(storeDir, filepath.FromSlash(rel))) //nolint:gosec // G304: the path is built from the store root and a listed name
	if err != nil {
		e.Err = err
		return e
	}
	if e.View, err = Decode(name, data); err != nil {
		e.View, e.Err = nil, fmt.Errorf("does not decode: %w", err)
	}
	return e
}

// Find returns the view whose file is named exactly name. The directory is
// listed, not opened by name, so that a file system that folds case does
// not hand back another view. ok is false when there is none.
func Find(storeDir, name string) (Entry, bool, error) {
	es, err := List(storeDir)
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range es {
		if e.Name == name {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}
