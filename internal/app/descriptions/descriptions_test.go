package descriptions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/descriptions"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

const store port.StoreID = "S1"

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T) (descriptions.Service, *recorder) {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return descriptions.Service{Stores: rec, Identity: apptest.Identity{Login: "hanako"}}, rec
}

func logTypes(t *testing.T, s port.Stores, ref change.Ref) []change.Type {
	t.Helper()
	var out []change.Type
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(ref)
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

var taskRef = change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}

func TestSaveItemWritesTheTextAndOneDescRow(t *testing.T) {
	svc, rec := setup(t)
	before := len(logTypes(t, rec, taskRef))
	res, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "## 手順\n1. 確認\n")
	if err != nil || res.Text != "## 手順\n1. 確認\n" || res.Conflict {
		t.Fatalf("SaveItem = %+v, %v", res, err)
	}
	if got, err := svc.GetItem(t.Context(), store, apptest.SampleTask); err != nil || got != "## 手順\n1. 確認\n" {
		t.Errorf("GetItem = %q, %v", got, err)
	}
	types := logTypes(t, rec, taskRef)
	if len(types) != before+1 || types[len(types)-1] != change.Desc {
		t.Errorf("log types = %v", types)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "HH-2 の説明を変更" {
		t.Errorf("activity = %q", last)
	}
}

func TestSaveItemSavesNothingWhenTheTextIsTheSame(t *testing.T) {
	svc, rec := setup(t)
	if _, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "本文\n"); err != nil {
		t.Fatal(err)
	}
	before := len(logTypes(t, rec, taskRef))
	res, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "本文\n", "本文\n")
	if err != nil || res.Text != "本文\n" || res.Conflict || len(logTypes(t, rec, taskRef)) != before {
		t.Errorf("same text: %+v, %v, log %d → %d", res, err, before, len(logTypes(t, rec, taskRef)))
	}
	// 別の人が先に同じ説明にしていても、同じになるなら保存しない。
	res, err = svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "本文\n")
	if err != nil || res.Conflict || len(logTypes(t, rec, taskRef)) != before {
		t.Errorf("already the same: %+v, %v", res, err)
	}
}

func TestSaveItemMergesWithWhatWasSavedMeanwhile(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "", "a\nb\n"); err != nil {
		t.Fatal(err)
	}
	// 編集を始めたときの説明は "a\nb\n"。そのあいだに別の人が 1 行目を変えた。
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "A\nb\n"); err != nil {
		t.Fatal(err)
	}
	// 別の行なら両方が残り、トーストは出さない。
	res, err := svc.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\nB\n")
	if err != nil || res.Text != "A\nB\n" || res.Conflict {
		t.Fatalf("different lines = %+v, %v", res, err)
	}
	// 片方しか変えていなければ、そちらを取る。
	res, err = svc.SaveItem(ctx, store, apptest.SampleTask, "A\nB\n", "A\nC\n")
	if err != nil || res.Text != "A\nC\n" || res.Conflict {
		t.Fatalf("one side = %+v, %v", res, err)
	}
	// 同じ行なら、保存してあるほうを先にして両方を残す。
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "A\nC\n", "A\nD\n"); err != nil {
		t.Fatal(err)
	}
	res, err = svc.SaveItem(ctx, store, apptest.SampleTask, "A\nC\n", "A\nE\n")
	if err != nil || res.Text != "A\nD\n-----\nE\n" || !res.Conflict {
		t.Fatalf("same line = %+v, %v", res, err)
	}
	if got, _ := svc.GetItem(ctx, store, apptest.SampleTask); got != "A\nD\n-----\nE\n" {
		t.Errorf("saved = %q", got)
	}
}

func TestSaveItemTreatsBlankAsEmptyAndRemovesTheText(t *testing.T) {
	svc, _ := setup(t)
	if _, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "本文\n"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "本文\n", " \n\n")
	if err != nil || res.Text != "" {
		t.Fatalf("blank = %+v, %v", res, err)
	}
	if got, err := svc.GetItem(t.Context(), store, apptest.SampleTask); err != nil || got != "" {
		t.Errorf("GetItem = %q, %v", got, err)
	}
}

func TestSaveProjectAndGet(t *testing.T) {
	svc, rec := setup(t)
	res, err := svc.SaveProject(t.Context(), store, apptest.SampleProject, "", "ログインの作り直し")
	if err != nil || res.Text != "ログインの作り直し" {
		t.Fatalf("SaveProject = %+v, %v", res, err)
	}
	if got, err := svc.GetProject(t.Context(), store, apptest.SampleProject); err != nil || got != "ログインの作り直し" {
		t.Errorf("GetProject = %q, %v", got, err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "ログイン の説明を変更" {
		t.Errorf("activity = %q", last)
	}
	types := logTypes(t, rec, change.Ref{Kind: change.KindProject, ID: apptest.SampleProject})
	if types[len(types)-1] != change.Desc {
		t.Errorf("project log types = %v", types)
	}
}

func TestSaveProjectActivityCollapsesTitleWhitespace(t *testing.T) {
	svc, rec := setup(t)
	const id = "01MULTILINE00000000000000A"
	p := project.Project{ID: id, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "再構築\n  の\t計画", Owner: "taro", Status: project.Todo}
	op := port.Operation{Actor: change.User("taro"), Summary: "プロジェクトを作成"}
	if err := rec.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteProjects().Append(p.ID, project.Create(p))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveProject(t.Context(), store, id, "", "概要"); err != nil {
		t.Fatalf("SaveProject = %v", err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "再構築 の 計画 の説明を変更" {
		t.Errorf("activity = %q", last)
	}
}

func TestUnknownIDsAreNotFound(t *testing.T) {
	svc, _ := setup(t)
	const missing = "01NOPE00000000000000000000"
	if _, err := svc.GetItem(t.Context(), store, missing); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("GetItem = %v", err)
	}
	if _, err := svc.SaveItem(t.Context(), store, missing, "", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SaveItem = %v", err)
	}
	if _, err := svc.GetProject(t.Context(), store, missing); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("GetProject = %v", err)
	}
	if _, err := svc.SaveProject(t.Context(), store, missing, "", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SaveProject = %v", err)
	}
}
