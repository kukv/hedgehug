// Package templates is the operations on the task templates of a store.
package templates

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// skeleton is what a new template starts from.
const skeleton = "---\nkind: task\n---\n"

// TemplateView is a template in a list.
type TemplateView struct {
	Name     string `json:"name"`
	Kind     string `json:"kind,omitempty"`
	Title    string `json:"title,omitempty"`
	Children int    `json:"children" doc:"The tasks it adds under the task, at every depth"`
}

// TemplateFile is a template as its file holds it.
type TemplateFile struct {
	Name    string `json:"name"`
	Content string `json:"content" doc:"The file: front matter and body"`
	Exists  bool   `json:"exists" doc:"False for a name that has no file: Content is the skeleton"`
}

// TemplateWrite is what a write to a template answers.
type TemplateWrite struct {
	Template TemplateView `json:"template"`
	Commit   *string      `json:"commit"`
}

func viewOf(t *model.Template) TemplateView {
	return TemplateView{Name: t.Name, Kind: t.Item.Kind, Title: t.Item.Title, Children: count(t.Item.Children)}
}

func count(items []model.TemplateItem) int {
	n := len(items)
	for i := range items {
		n += count(items[i].Children)
	}
	return n
}

func badName(name string) error {
	if err := model.ValidTemplateName(name); err != nil {
		return usecase.NewErr(usecase.KindUsage, "template.bad_name", err, "value", name)
	}
	return nil
}

func notFound(name string) error {
	return usecase.NewErr(usecase.KindNotFound, "template.not_found", fmt.Errorf("template %s is not in this store", name), "name", name)
}

// List lists the templates by name. A file that does not decode is a warning;
// hedgehug doctor reports it too.
func List(ss *usecase.Session) ([]TemplateView, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return nil, err
	}
	out := []TemplateView{}
	for t, err := range s.Templates() {
		if err != nil {
			ss.AddWarning("a template does not read: " + err.Error())
			continue
		}
		out = append(out, viewOf(t))
	}
	slices.SortFunc(out, func(a, b TemplateView) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// Read reads the file of the template name as it is, so that one that does
// not decode can be repaired. A name with no file gives the skeleton.
func Read(ss *usecase.Session, name string) (TemplateFile, error) {
	if err := badName(name); err != nil {
		return TemplateFile{}, err
	}
	s, err := ss.OpenStore()
	if err != nil {
		return TemplateFile{}, err
	}
	b, err := osfs.ReadFile(filepath.Join(s.Dir(), model.TemplatePath(name)))
	if errors.Is(err, fs.ErrNotExist) {
		return TemplateFile{Name: name, Content: skeleton}, nil
	}
	if err != nil {
		return TemplateFile{}, err
	}
	return TemplateFile{Name: name, Content: string(b), Exists: true}, nil
}

// checkItem refuses a kind or a status the schema lacks, at every depth. What
// points at tasks and sprints is checked when the template is expanded.
func checkItem(sc *schema.Schema, it *model.TemplateItem) error {
	if it.Kind != "" {
		if _, ok := sc.Kind(it.Kind); !ok {
			return fmt.Errorf("kind %q is not in the schema", it.Kind)
		}
	}
	if it.Status != "" {
		if _, ok := sc.Status(it.Status); !ok {
			return fmt.Errorf("status %q is not in the schema", it.Status)
		}
	}
	for i := range it.Children {
		if err := checkItem(sc, &it.Children[i]); err != nil {
			return err
		}
	}
	return nil
}

// Put writes the template name with the file content.
func Put(ss *usecase.Session, name, content string) (TemplateWrite, error) {
	if err := badName(name); err != nil {
		return TemplateWrite{}, err
	}
	t, err := model.DecodeTemplate(name, []byte(content))
	if err != nil {
		return TemplateWrite{}, usecase.NewErr(usecase.KindInvalid, "template.invalid", err, "name", name)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TemplateWrite{}, err
	}
	if err := checkItem(ix.Schema(), &t.Item); err != nil {
		return TemplateWrite{}, usecase.NewErr(usecase.KindInvalid, "template.invalid", err, "name", name)
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "template-set", Detail: name}, func(tx *store.Tx) error {
		return tx.PutTemplate(t)
	})
	if err != nil {
		return TemplateWrite{}, err
	}
	return TemplateWrite{Template: viewOf(t), Commit: usecase.CommitPtr(usecase.CommitHex(hash))}, nil
}

// usedBy says what of the schema names the template; "" for nothing.
func usedBy(sc *schema.Schema, name string) string {
	for _, k := range sc.Kinds {
		if k.Template == name {
			return "the kind " + k.Name
		}
	}
	for _, sco := range sc.Scoped {
		if sco.Templates == nil {
			continue
		}
		if slices.Contains(sco.Templates.Only, name) {
			return "the override " + sco.Where
		}
		for _, v := range sco.Templates.Default {
			if v == name {
				return "the override " + sco.Where
			}
		}
	}
	return ""
}

// Remove deletes the template name, which nothing in the schema names.
func Remove(ss *usecase.Session, name string) (TemplateWrite, error) {
	if err := badName(name); err != nil {
		return TemplateWrite{}, err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TemplateWrite{}, err
	}
	t, err := s.Template(name)
	view := TemplateView{Name: name}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return TemplateWrite{}, notFound(name)
	case err == nil:
		view = viewOf(t)
	}
	if by := usedBy(ix.Schema(), name); by != "" {
		return TemplateWrite{}, usecase.NewErr(usecase.KindConflict, "template.in_use", fmt.Errorf("template %s is used by %s", name, by), "name", name)
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "template-rm", Detail: name}, func(tx *store.Tx) error {
		return tx.RemoveFile(model.TemplatePath(name))
	})
	if err != nil {
		return TemplateWrite{}, err
	}
	return TemplateWrite{Template: view, Commit: usecase.CommitPtr(usecase.CommitHex(hash))}, nil
}
