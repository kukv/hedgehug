package descriptions_test

import (
	"context"
	"errors"
	"reflect"
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
	return descriptions.Service{Stores: rec, Identity: apptest.Identity{Login: "hanako"}, Inbox: &apptest.Inbox{}}, rec
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

const conflictText = "説明の同じ箇所が同時に編集されたため、両方を残しました"

// 同じ箇所が重なって両方を残した保存は、保存した本人の受信箱に 1 行。差出人は、保存の中で読んだいまの説明の
// 最後の desc の行の人(C4c1-13)。重ならない保存では作らない。
func TestSaveConflictNotifies(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	inbox := svc.Inbox.(*apptest.Inbox)
	taro := svc
	taro.Identity = apptest.Identity{Login: "taro"}
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "", "a\nb\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := taro.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\nx\n"); err != nil {
		t.Fatal(err)
	}
	if got := inbox.Notices(); len(got) != 0 {
		t.Fatalf("no conflict: notices = %+v", got)
	}
	res, err := svc.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\ny\n")
	if err != nil || !res.Conflict {
		t.Fatalf("SaveItem = %+v, %v", res, err)
	}
	want := port.Notice{
		Kind: port.NoticeOverwrite, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask,
		Text: conflictText, Display: "HH-2", Sender: "taro", Field: "desc",
	}
	if got := inbox.Notices(); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("notices = %+v, want %+v", got, want)
	}
}

// 作成のあと desc の行がないプロジェクトでは、差出人は作成の行の人。
func TestSaveProjectConflictNotifiesTheCreator(t *testing.T) {
	svc, rec := setup(t)
	const id = "01CONFLICT0000000000000000"
	p := project.Project{ID: id, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "計画", Owner: "taro", Status: project.Todo}
	op := port.Operation{Actor: change.User("taro"), Summary: "プロジェクトを作成"}
	if err := rec.Write(t.Context(), store, op, func(tx port.Tx) error {
		return errors.Join(tx.WriteProjects().Append(p.ID, project.Create(p)), tx.WriteProjects().Describe(p.ID, "a\nx\n"))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveProject(t.Context(), store, id, "a\nb\n", "a\ny\n"); err != nil {
		t.Fatal(err)
	}
	want := port.Notice{
		Kind: port.NoticeOverwrite, Store: store, Workspace: apptest.SampleWorkspace, Project: id,
		Text: conflictText, Display: "HH-P2", Sender: "taro", Field: "desc",
	}
	if got := svc.Inbox.(*apptest.Inbox).Notices(); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("notices = %+v, want %+v", got, want)
	}
}

// 最後の desc の行が自動化の行なら、差出人は空(「hedgehug」)。
func TestSaveConflictAutomationSender(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "", "a\nb\n"); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "自動化"}
	if err := rec.Write(ctx, store, op, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(apptest.SampleTask, "a\nx\n"),
			tx.WriteItems().Append(apptest.SampleTask, change.NewDesc().ByRule("01RULE0000000000000000001")))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\ny\n"); err != nil {
		t.Fatal(err)
	}
	if got := svc.Inbox.(*apptest.Inbox).Notices(); len(got) != 1 || got[0].Sender != "" {
		t.Errorf("notices = %+v, want one from hedgehug", got)
	}
}

// failing は、fn のあとに Write を失敗させる Stores。
type failing struct {
	port.Stores
	err error
}

func (f failing) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	return f.Stores.Write(ctx, id, op, func(tx port.Tx) error {
		if err := fn(tx); err != nil {
			return err
		}
		return f.err
	})
}

// 重なった保存の書き込みが失敗したら、受信箱に行を足さない。
func TestSaveConflictWriteFailsAddsNothing(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	taro := svc
	taro.Identity = apptest.Identity{Login: "taro"}
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "", "a\nb\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := taro.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\nx\n"); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	broken := svc
	broken.Stores = failing{Stores: svc.Stores, err: boom}
	if _, err := broken.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\ny\n"); !errors.Is(err, boom) {
		t.Fatalf("SaveItem err = %v, want %v", err, boom)
	}
	if got := svc.Inbox.(*apptest.Inbox).Notices(); len(got) != 0 {
		t.Errorf("notices = %+v, want none", got)
	}
}

// 書き込みが済んだあとに受信箱へ足せなくても、保存は成功として合わせた結果を返す(失敗にすると、同じ入力の
// やり直しで両方を残す区切りがもう一度入る)。
func TestSaveConflictInboxFailureStillSucceeds(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	taro := svc
	taro.Identity = apptest.Identity{Login: "taro"}
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "", "a\nb\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := taro.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\nx\n"); err != nil {
		t.Fatal(err)
	}
	svc.Inbox = &apptest.Inbox{Err: errors.New("inbox down")}
	res, err := svc.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\ny\n")
	if err != nil || !res.Conflict {
		t.Fatalf("SaveItem = %+v, %v, want conflict result and nil error", res, err)
	}
	got, err := svc.GetItem(ctx, store, apptest.SampleTask)
	if err != nil || got != res.Text {
		t.Errorf("GetItem = %q, %v, want %q", got, err, res.Text)
	}
}
