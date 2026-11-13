package items_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// descriptionOf は、アイテムの説明の本文。
func descriptionOf(t *testing.T, s port.Stores, id item.ID) string {
	t.Helper()
	var text string
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		text, err = r.Items().Description(id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return text
}

// logTypes は、アイテムのログの行の種類を、書いた順に返す。
func logTypes(t *testing.T, s port.Stores, id item.ID) []change.Type {
	t.Helper()
	var out []change.Type
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		for _, rec := range log {
			out = append(out, rec.T)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCreateWritesTheDescriptionWithoutADescRow(t *testing.T) {
	svc, rec := setupRec(t)
	it, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task,
		Title: "API を作る", Description: "## 手順\n1. 確認\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := descriptionOf(t, rec, it.ID); got != "## 手順\n1. 確認\n" {
		t.Errorf("description = %q", got)
	}
	// 履歴は「作成」の 1 行だけ(02「作成したあと」)。desc の行は書かない。
	if types := logTypes(t, rec, it.ID); len(types) != 1 || types[0] != change.Create {
		t.Errorf("log types = %v", types)
	}
	if got := last(rec); got != `Task「API を作る」を作成` {
		t.Errorf("summary = %q", got)
	}
}

func TestCreateWithABlankDescriptionWritesNoDescription(t *testing.T) {
	svc, rec := setupRec(t)
	it, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task,
		Title: "API を作る", Description: " \n\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := descriptionOf(t, rec, it.ID); got != "" {
		t.Errorf("description = %q", got)
	}
}

func TestCreateFromATemplateWritesTheChildrenDescriptions(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{
		Name: "リリース", Kind: "Epic", Title: "リリース", Description: "## テンプレートの説明",
		Children: []workspace.TemplateChild{
			{Kind: "Story", Title: "QA", Description: "## 確認項目\n", Steps: []workspace.TemplateStep{{Title: "手順書", Assignee: workspace.StepParent}}},
			{Kind: "Task", Title: "本番反映"},
		},
	})
	root, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Kind: "Epic", Title: "11 月のリリース", Template: &tp.ID,
		Description: "入力した説明",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 番号は深さ優先: Epic 3、Story 4、Step 5、Task 6。
	all := byNumber(t, svc)
	qa, step, deploy := all[4], all[5], all[6]
	if got := descriptionOf(t, rec, root.ID); got != "入力した説明" {
		t.Errorf("root description = %q (the template's text is applied by the dialog, not the server)", got)
	}
	if got := descriptionOf(t, rec, qa.ID); got != "## 確認項目\n" {
		t.Errorf("child description = %q", got)
	}
	if descriptionOf(t, rec, step.ID) != "" || descriptionOf(t, rec, deploy.ID) != "" {
		t.Errorf("a step and a child without a description must have none")
	}
	if types := logTypes(t, rec, qa.ID); len(types) != 1 || types[0] != change.Create {
		t.Errorf("child log types = %v", types)
	}
}

func TestCreateWithoutADescriptionLeavesTheRootEmptyEvenForATemplateWithOne(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{Name: "手順", Kind: "Task", Title: "作業", Description: "## 手順"})
	it, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: "作業", Template: &tp.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := descriptionOf(t, rec, it.ID); got != "" {
		t.Errorf("description = %q", got)
	}
}
