package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

var moveOp = port.Operation{Actor: change.User("taro"), Summary: "移動"}

// sampleTaskLog は、サンプルのストアのタスクの行。
func sampleTaskLog(t *testing.T, s *Stores, id port.StoreID) []change.Record {
	t.Helper()
	var log []change.Record
	if err := s.Read(t.Context(), id, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(taskRef)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return log
}

func TestImportWritesTheFolderInOneCommit(t *testing.T) {
	src, srcID, srcDir := newStore(t)
	if err := apptest.Sample(t.Context(), src, srcID); err != nil {
		t.Fatal(err)
	}
	log := sampleTaskLog(t, src, srcID)
	dst, dstID, dstDir := newStore(t)
	before := headCount(t, dstDir)
	if err := dst.Write(t.Context(), dstID, moveOp, func(tx port.Tx) error {
		return tx.WriteItems().Import(apptest.SampleTask, log, "説明")
	}); err != nil {
		t.Fatal(err)
	}
	if headCount(t, dstDir) != before+1 {
		t.Errorf("commits = %d, want %d", headCount(t, dstDir), before+1)
	}
	p := "items/" + string(apptest.SampleTask) + "/"
	got, want := files(t, dstDir), files(t, srcDir)
	if got[p+"log.jsonl"] != want[p+"log.jsonl"] {
		t.Errorf("log.jsonl =\n%s\nwant (写した行は変わらない)\n%s", got[p+"log.jsonl"], want[p+"log.jsonl"])
	}
	if got[p+"description.md"] != "説明" || !strings.Contains(got[p+"item.yaml"], "ログイン画面を作る") {
		t.Errorf("description.md = %q, item.yaml =\n%s", got[p+"description.md"], got[p+"item.yaml"])
	}
	if st := worktreeStatus(t, dstDir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}

func TestRemoveDeletesTheFolderInOneCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	before := headCount(t, dir)
	if err := s.Write(t.Context(), id, moveOp, func(tx port.Tx) error { return tx.WriteItems().Remove(apptest.SampleTask) }); err != nil {
		t.Fatal(err)
	}
	if headCount(t, dir) != before+1 {
		t.Errorf("commits = %d, want %d", headCount(t, dir), before+1)
	}
	if _, err := os.Stat(filepath.Join(dir, "items", string(apptest.SampleTask))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder is still there: %v", err)
	}
	if _, ok := files(t, dir)["items/"+string(apptest.SampleEpic)+"/log.jsonl"]; !ok {
		t.Error("another item went too")
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}

func TestTransfersAreAppendedToOneFile(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	rec := func(old string) transfer.Record {
		return transfer.Record{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: old, New: "MB-3", ToStore: "01OTHERSTORE", ToStoreName: "team"}
	}
	for _, old := range []string{"HH-2", "HH-9"} {
		if err := s.Write(t.Context(), id, moveOp, func(tx port.Tx) error { return tx.WriteTransfers().Append(rec(old)) }); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "transfers.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	got := transfer.ParseRecords(b)
	if len(got) != 2 || got[0].Old != "HH-2" || got[1].Old != "HH-9" || strings.Count(string(b), "\n") != 2 {
		t.Fatalf("transfers.jsonl =\n%s", b)
	}
	if msg := headCommit(t, dir).Message; !strings.Contains(msg, "Hedgehug-Op: "+string(got[1].Op)) {
		t.Errorf("the last commit %q does not carry op %s", msg, got[1].Op)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree is not clean (transfers.jsonl は同じコミットに入る):\n%s", st)
	}
	err = s.Read(t.Context(), id, func(r port.Reader) error {
		list, err := r.Transfers().List()
		if err != nil || len(list) != 2 || list[0].ID != got[0].ID || list[1].ID != got[1].ID {
			t.Errorf("List = %+v, %v", list, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPurgedIsReadFromTheFileAndSkipsBrokenLines(t *testing.T) {
	s, id, dir := newStore(t)
	file := `{"id":"01P1","at":"2026-12-12T00:00:05.000+09:00","by":"hedgehug[bot]","actor":"none","entity":"item","target":"01JF3KQ8W2A7XG0M5C9D4E6HTR","display":"HH-12"}` + "\n" +
		"not json\n" + `{"id":"01P2","at":"bad","entity":"item","target":"01X"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "purged.jsonl"), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	err := s.Read(t.Context(), id, func(r port.Reader) error {
		got, err := r.Purged().List()
		if err != nil || len(got) != 1 || got[0].Entity != "item" || got[0].Target != "01JF3KQ8W2A7XG0M5C9D4E6HTR" ||
			got[0].Display != "HH-12" || got[0].At.UTC().Format("2006-01-02T15:04:05") != "2026-12-11T15:00:05" {
			t.Errorf("Purged = %+v, %v", got, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// コミットが失敗したら、写したフォルダ・消したフォルダ・転送記録は、何も残らず元に戻る。
func TestFailedWriteRestoresImportedAndRemovedFolders(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	log := sampleTaskLog(t, s, id)
	beforeFiles, beforeHead := files(t, dir), headCommit(t, dir).Hash
	boom := errors.New("boom")
	s.beforeCommit = func() error { return boom }
	err := s.Write(t.Context(), id, moveOp, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Remove(apptest.SampleTask),
			tx.WriteItems().Import("01MOVEDIT0000000000000001", log, "写し"),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "HH-2", New: "MB-3", ToStore: "S"}),
		)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Write = %v", err)
	}
	if !reflect.DeepEqual(files(t, dir), beforeFiles) || headCommit(t, dir).Hash != beforeHead {
		t.Error("the failed write left files or a commit behind")
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}

// プロジェクトの Import は projects/<ID>/ に行・説明・project.yaml を 1 つのコミットで作り、Remove は 1 つのコミットで
// フォルダごと消す(下のアイテムのフォルダは残る)。
func TestProjectImportAndRemoveWriteTheFolder(t *testing.T) {
	src, srcID, srcDir := newStore(t)
	if err := apptest.Sample(t.Context(), src, srcID); err != nil {
		t.Fatal(err)
	}
	ref := change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}
	var log []change.Record
	if err := src.Read(t.Context(), srcID, func(r port.Reader) (err error) { log, err = r.Changes().Of(ref); return }); err != nil {
		t.Fatal(err)
	}
	dst, dstID, dstDir := newStore(t)
	before := headCount(t, dstDir)
	if err := dst.Write(t.Context(), dstID, moveOp, func(tx port.Tx) error {
		return tx.WriteProjects().Import(apptest.SampleProject, log, "説明")
	}); err != nil {
		t.Fatal(err)
	}
	p := "projects/" + string(apptest.SampleProject) + "/"
	got, want := files(t, dstDir), files(t, srcDir)
	if headCount(t, dstDir) != before+1 || got[p+"log.jsonl"] != want[p+"log.jsonl"] || got[p+"description.md"] != "説明" || !strings.Contains(got[p+"project.yaml"], "ログイン") {
		t.Errorf("commits %d (want %d), files:\n%v", headCount(t, dstDir), before+1, got)
	}
	if err := src.Write(t.Context(), srcID, moveOp, func(tx port.Tx) error { return tx.WriteProjects().Remove(apptest.SampleProject) }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "projects", string(apptest.SampleProject))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the project folder is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "items", string(apptest.SampleTask))); err != nil {
		t.Errorf("the item folder went too: %v", err)
	}
	if st := worktreeStatus(t, srcDir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}
