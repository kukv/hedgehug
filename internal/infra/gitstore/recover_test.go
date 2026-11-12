package gitstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const strayItem = "01STRAYIT00000000000000001"

var taskRef = change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}

// simulateCrash は、保存の途中でプロセスが落ちた跡を作る。印を置き、保存済みの log.jsonl に
// コミットしていない行と改行のない書きかけの行を足し、コミットしていないエンティティのフォルダを作る。
func simulateCrash(t *testing.T, s *Stores, id port.StoreID, dir string) {
	t.Helper()
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	var createID change.ID
	if err := s.Read(t.Context(), id, func(r port.Reader) error {
		log, err := r.Changes().Of(taskRef)
		if err != nil {
			return err
		}
		createID = log[0].ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath(dir), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	leftover, err := json.Marshal(change.Record{
		ID: "01LEFTOVER0000000000000001", Op: "01LEFTOVEROP00000000000001", At: t0, By: "taro", Actor: change.User("taro"),
		T: change.Set, F: "title", From: change.Value("ログイン画面を作る"), To: change.Value("残り"), Prev: []change.ID{createID},
	})
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "items", string(apptest.SampleTask), "log.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(string(leftover) + "\n" + `{"id":"01HALF`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "items", strayItem), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "items", strayItem, "log.jsonl"), []byte(string(leftover)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// assertRecovered は、途中で止まった保存の跡が何も残っていないことを確かめる。
func assertRecovered(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(markerPath(dir)); !os.IsNotExist(err) {
		t.Errorf("marker left: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "items", strayItem)); !os.IsNotExist(err) {
		t.Errorf("stray folder left: %v", err)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
	b, err := os.ReadFile(filepath.Join(dir, "items", string(apptest.SampleTask), "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "01LEFTOVER") || strings.Contains(string(b), "01HALF") {
		t.Errorf("log.jsonl keeps the interrupted rows:\n%s", b)
	}
}

func TestWriteRecoversFromInterruptedSave(t *testing.T) {
	s, id, dir := newStore(t)
	simulateCrash(t, s, id, dir)
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "HH-2 のタイトルを変更"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "新しい"))
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRecovered(t, dir)
	if n := headCount(t, dir); n != 3 {
		t.Errorf("commits = %d, want 3", n)
	}
	err = s.Read(t.Context(), id, func(r port.Reader) error {
		log, err := r.Changes().Of(taskRef)
		if err != nil || len(log) != 2 || string(log[1].To) != `"新しい"` || len(log[1].Prev) != 1 || log[1].Prev[0] != log[0].ID {
			t.Errorf("log = %+v, %v", log, err)
		}
		if _, err := r.Items().Get(strayItem); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("stray item: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadRecoversFromInterruptedSave(t *testing.T) {
	s, id, dir := newStore(t)
	simulateCrash(t, s, id, dir)
	err := s.Read(t.Context(), id, func(r port.Reader) error {
		log, err := r.Changes().Of(taskRef)
		if err != nil || len(log) != 1 {
			t.Errorf("log = %+v, %v", log, err)
		}
		it, err := r.Items().Get(apptest.SampleTask)
		if err != nil || it.Title != "ログイン画面を作る" {
			t.Errorf("item = %+v, %v", it, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRecovered(t, dir)
	if n := headCount(t, dir); n != 2 {
		t.Errorf("commits = %d, want 2", n)
	}
}

// 1 つ目のエンティティをインデックスに足したあと、2 つ目を書く前に失敗しても、何も残らない。
func TestFailurePartwayThroughEntitiesLeavesNothing(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	const newItem = "01NEWITEM00000000000000001" // 辞書順で SampleTask より先に書かれる
	boom := errors.New("boom")
	calls := 0
	s.afterEntity = func() error {
		calls++
		return boom
	}
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "x"}, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(newItem, change.NewCreate(map[string]any{"id": newItem})),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "x")),
		)
	})
	if !errors.Is(err, boom) || calls != 1 {
		t.Fatalf("Write error = %v, calls = %d", err, calls)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree after a failed write:\n%s", st)
	}
	if _, err := os.Stat(filepath.Join(dir, "items", newItem)); !os.IsNotExist(err) {
		t.Errorf("new folder left: %v", err)
	}
	if _, err := os.Stat(markerPath(dir)); !os.IsNotExist(err) {
		t.Errorf("marker left: %v", err)
	}
	if n := headCount(t, dir); n != 2 {
		t.Errorf("a failed write made a commit (%d commits)", n)
	}
}

func TestRefusesSummaryThatIsNotOneLine(t *testing.T) {
	for _, summary := range []string{"", "1 行目\n2 行目", "改行\r"} {
		s, id, dir := newStore(t)
		err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: summary}, func(tx port.Tx) error {
			return tx.WriteItems().Append(strayItem, change.NewCreate(map[string]any{"id": strayItem}))
		})
		if !errors.Is(err, errSummary) {
			t.Errorf("summary %q: Write error = %v", summary, err)
		}
		if n := headCount(t, dir); n != 1 {
			t.Errorf("summary %q: commits = %d", summary, n)
		}
		if _, err := os.Stat(filepath.Join(dir, "items")); !os.IsNotExist(err) {
			t.Errorf("summary %q: items folder made: %v", summary, err)
		}
	}
}

// 見つからないとき以外の読み取りのエラーは、List が飲み込まずに返す。
func TestWorkspaceListReturnsReadErrors(t *testing.T) {
	s, id, dir := newStore(t)
	// log.jsonl がフォルダなので、読むとエラーになる。
	if err := os.MkdirAll(filepath.Join(dir, "workspaces", "01BADWS0000000000000000001", "log.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := s.Read(t.Context(), id, func(r port.Reader) error {
		if ws, err := r.Workspaces().List(); err == nil {
			t.Errorf("List = %+v, want an error", ws)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProjectAndItemListReturnReadErrors(t *testing.T) {
	s, id, dir := newStore(t)
	for _, d := range []string{"projects/01BADPR0000000000000000001", "items/01BADIT0000000000000000001"} {
		// log.jsonl がフォルダなので、読むとエラーになる。
		if err := os.MkdirAll(filepath.Join(dir, d, "log.jsonl"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	err := s.Read(t.Context(), id, func(r port.Reader) error {
		if ps, err := r.Projects().List("W"); err == nil {
			t.Errorf("Projects().List = %+v, want an error", ps)
		}
		if its, err := r.Items().List("P"); err == nil {
			t.Errorf("Items().List = %+v, want an error", its)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
