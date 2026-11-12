package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func headCount(t *testing.T, dir string) int {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	if err := iter.ForEach(func(*object.Commit) error { n++; return nil }); err != nil {
		t.Fatalf("walk log: %v", err)
	}
	return n
}

func headCommit(t *testing.T, dir string) *object.Commit {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	c, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("head commit: %v", err)
	}
	return c
}

func TestOneWriteIsOneCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	c := headCommit(t, dir)
	if headCount(t, dir) != 2 {
		t.Errorf("commits = %d, want 2 (store.yaml + one save)", headCount(t, dir))
	}
	if c.Author.Name != "taro" || c.Author.Email != "taro@hedgehug.invalid" || c.Committer.Name != "taro" {
		t.Errorf("author = %+v, committer = %+v", c.Author, c.Committer)
	}
	subject, trailer, _ := strings.Cut(c.Message, "\n\n")
	if subject != "サンプルデータを作成" || !strings.HasPrefix(trailer, "Hedgehug-Op: 01TEST") {
		t.Errorf("message = %q", c.Message)
	}
	status := worktreeStatus(t, dir)
	if !status.IsClean() {
		t.Errorf("worktree is not clean after a save:\n%s", status)
	}
}

func TestEmptyWriteMakesNoCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro")}, func(port.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if headCount(t, dir) != 1 {
		t.Errorf("an empty write made a commit")
	}
}

func TestYAMLIsRebuiltFromLog(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("hanako"), Summary: "HH-2 のタイトルを変更"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン画面"))
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "items", string(apptest.SampleTask), "item.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `id: 01SAMPLEIT0000000000000002
workspace: 01SAMPLEWS0000000000000001
project: 01SAMPLEPR0000000000000001
parent: 01SAMPLEIT0000000000000001
number: 2
kind: Task
rank: V
title: ログイン画面
created: {at: "2026-11-12T09:00:00.000+09:00", by: taro}
status: ` + string(apptest.SampleStatusTodo) + ` # Todo
assignee: null
priority: ` + string(apptest.SamplePriorityMedium) + ` # Medium
start: null
end: null
points: null
systems: []
fields: {}
estimate: null
watchers: [taro]
`
	if string(b) != want {
		t.Errorf("item.yaml =\n%s\nwant\n%s", b, want)
	}
	log, err := os.ReadFile(filepath.Join(dir, "items", string(apptest.SampleTask), "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(log), "\n"); n != 2 {
		t.Errorf("log.jsonl has %d lines, want 2", n)
	}
}

func TestFailedCommitLeavesNothing(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	s.beforeCommit = func() error { return boom }
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "x"}, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "x")),
			tx.WriteItems().Append("01NEWITEM00000000000000001", change.NewCreate(map[string]any{"id": "01NEWITEM00000000000000001"})),
		)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Write error = %v", err)
	}
	if status := worktreeStatus(t, dir); !status.IsClean() {
		t.Errorf("worktree after a failed commit:\n%s", status)
	}
	if _, err := os.Stat(filepath.Join(dir, "items", "01NEWITEM00000000000000001")); !os.IsNotExist(err) {
		t.Errorf("new folder left: %v", err)
	}
	if headCount(t, dir) != 2 {
		t.Errorf("a failed write made a commit")
	}
}

func TestRefusesIDThatIsNotAPath(t *testing.T) {
	s, id, _ := newStore(t)
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro")}, func(tx port.Tx) error {
		return tx.WriteItems().Append("../../etc", change.NewCreate(map[string]any{}))
	})
	if err == nil {
		t.Error("an id with a slash was written")
	}
}

func TestReopenReadsTheSameData(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	s2 := New(apptest.NewClock(t0), apptest.NewIDs())
	if _, err := s2.Attach(dir); err != nil {
		t.Fatal(err)
	}
	err := s2.Read(t.Context(), id, func(r port.Reader) error {
		it, err := r.Items().Get(apptest.SampleTask)
		if err != nil || it.Title != "ログイン画面を作る" || it.DisplayID("HH") != "HH-2" {
			t.Errorf("item = %+v, %v", it, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func worktreeStatus(t *testing.T, dir string) git.Status {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	st, err := wt.Status()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// 偽物と同じく、Tx の中で読んだ行を呼び出し側が変えても、書く行は変わらない。
func TestTxReadRowsAreCopies(t *testing.T) {
	s, id, _ := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	ref := change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "x"}, func(tx port.Tx) error {
		if err := tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "y")); err != nil {
			return err
		}
		log, err := tx.Changes().Of(ref)
		if err != nil {
			return err
		}
		log[1].To[1] = 'z'
		log[1].Prev[0] = "01BROKEN"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = s.Read(t.Context(), id, func(r port.Reader) error {
		log, err := r.Changes().Of(ref)
		if err != nil || len(log) != 2 || string(log[1].To) != `"y"` || log[1].Prev[0] != log[0].ID {
			t.Errorf("log = %+v, %v", log, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// 偽物と同じく、行のない Append は何も書かない(コミットも、新しいフォルダも作らない)。
func TestAppendWithoutDraftsWritesNothing(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	const newID = "01NEWITEM00000000000000001"
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "x"}, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Append(apptest.SampleTask), tx.WriteItems().Append(newID))
	})
	if err != nil {
		t.Fatal(err)
	}
	if headCount(t, dir) != 2 {
		t.Errorf("a write without rows made a commit")
	}
	if _, err := os.Stat(filepath.Join(dir, "items", newID)); !os.IsNotExist(err) {
		t.Errorf("new folder made: %v", err)
	}
}

// 自動化・同期・GitHub の保存の作者とコミッターは hedgehug[bot](01「「あなた」の決まり方」)。
func TestNonUserWriteIsByBot(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "修復"}
	err := s.Write(t.Context(), id, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "y"))
	})
	if err != nil {
		t.Fatal(err)
	}
	c := headCommit(t, dir)
	if c.Author.Name != "hedgehug[bot]" || c.Author.Email != "bot@hedgehug.invalid" || c.Committer.Name != "hedgehug[bot]" || c.Committer.Email != "bot@hedgehug.invalid" {
		t.Errorf("author = %+v, committer = %+v", c.Author, c.Committer)
	}
}

// 値を持つアイテムの item.yaml は、status・priority・systems・fields を書く(値のないものは空)。
func TestItemYAMLValues(t *testing.T) {
	const (
		sDoing change.ID = "01STATUS00000000000000000B"
		yAPI   change.ID = "01SYS00000000000000000000B"
		fEnv   change.ID = "01PROP0000000000000000000A" // 単一選択
		fTags  change.ID = "01PROP0000000000000000000B" // 複数選択
		vProd  change.ID = "01CHOICE00000000000000000A"
		vA     change.ID = "01CHOICE00000000000000000B"
		vB     change.ID = "01CHOICE00000000000000000C"
	)
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	const bare item.ID = "01BARE000000000000000000001" // 値のないアイテム
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "値を付ける"}, func(tx port.Tx) error {
		if err := tx.WriteItems().Append(bare, item.Create(item.Item{
			ID: bare, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3},
			Kind: item.Epic, Rank: "W", Title: "値なし",
		})); err != nil {
			return err
		}
		return tx.WriteItems().Append(apptest.SampleTask,
			change.NewSet("status", nil, sDoing),
			change.NewAdd("systems", yAPI),
			change.NewSet(item.FieldKey(fEnv), nil, vProd),
			change.NewAdd(item.FieldKey(fTags), vB),
			change.NewAdd(item.FieldKey(fTags), vA),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	read := func(it item.ID) string {
		b, err := os.ReadFile(filepath.Join(dir, "items", string(it), "item.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	y := read(apptest.SampleTask)
	for _, want := range []string{
		"status: " + string(sDoing) + "\n",
		"priority: " + string(apptest.SamplePriorityMedium) + " # Medium\n",
		"systems: [" + string(yAPI) + "]\n",
		"fields:\n  " + string(fEnv) + ": " + string(vProd) + "\n  " + string(fTags) + ": [" + string(vA) + ", " + string(vB) + "]\n",
	} {
		if !strings.Contains(y, want) {
			t.Errorf("item.yaml に %q がない:\n%s", want, y)
		}
	}
	y = read(bare)
	for _, want := range []string{"status: null\n", "priority: null\n", "systems: []\n", "fields: {}\n"} {
		if !strings.Contains(y, want) {
			t.Errorf("値のないアイテムの item.yaml に %q がない:\n%s", want, y)
		}
	}
}
