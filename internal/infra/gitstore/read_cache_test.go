package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// 1 回の Read のあいだは、一度読んだ log.jsonl を読み直さない。次の Read は読み直す。
func TestReadCachesLogWithinOneView(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "items", string(apptest.SampleTask), "log.jsonl")
	ref := change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}
	err := s.Read(t.Context(), id, func(r port.Reader) error {
		first, err := r.Changes().Of(ref)
		if err != nil {
			return err
		}
		first[0].Prev = append(first[0].Prev, "01BROKEN") // 呼び出し側が変えても、次に読む行は変わらない
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			return err
		}
		again, err := r.Changes().Of(ref)
		if err != nil || len(again) != len(first) || len(again[0].Prev) != 0 {
			t.Errorf("second read in the same view = %+v, %v", again, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = s.Read(t.Context(), id, func(r port.Reader) error {
		if _, err := r.Items().Get(apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("next view did not re-read the file: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Write の中では、読んだあとに足した行(pending)も読める。
func TestWriteReadsItsOwnRowsAfterCaching(t *testing.T) {
	s, id, _ := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "x"}, func(tx port.Tx) error {
		if _, err := tx.Items().Get(apptest.SampleTask); err != nil {
			return err
		}
		if err := tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "y")); err != nil {
			return err
		}
		it, err := tx.Items().Get(apptest.SampleTask)
		if err != nil || it.Title != "y" {
			t.Errorf("item = %+v, %v", it, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
