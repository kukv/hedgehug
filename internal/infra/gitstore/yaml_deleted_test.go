package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// 削除済みの印は item.yaml・project.yaml の deleted に書く。削除されていなければ null(保存形式「アイテムの例」)。
func TestYAMLWritesTheDeletionMark(t *testing.T) {
	jst := time.FixedZone("JST", 9*3600)
	meta := func(op string, h int) change.Meta {
		return change.Meta{Op: change.ID(op), At: time.Date(2026, 11, 13, h, 0, 0, 0, jst), Actor: change.User("taro")}
	}
	p := project.Project{ID: yProj, Place: project.Place{Workspace: yWS, Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	live := change.Stamp(nil, []change.Draft{project.Create(p)}, meta("o1", 9), yIDs())
	deleted := slices.Concat(live, change.Stamp(live, []change.Draft{change.NewDeleteEntity(true)}, meta("o2", 10), yIDs()))

	out, err := renderYAML(change.KindProject, live, nil, nil)
	if err != nil || !strings.Contains(string(out), "\ndeleted: null\n") {
		t.Errorf("a live project.yaml =\n%s\n%v", out, err)
	}
	out, err = renderYAML(change.KindProject, deleted, nil, nil)
	if err != nil || !strings.Contains(string(out), "\ndeleted: {op: o2, at: \"2026-11-13T10:00:00.000+09:00\", root: true}\n") {
		t.Errorf("a deleted project.yaml =\n%s\n%v", out, err)
	}
	// アイテムも同じ。下のもの(root: false)は root が false。
	it := item.Item{ID: yItem, Place: item.Place{Workspace: yWS, Project: yProj, Number: 1}, Kind: item.Epic, Rank: "V", Title: "認証"}
	ilog := change.Stamp(nil, []change.Draft{item.Create(it)}, meta("o1", 9), yIDs())
	ilog = slices.Concat(ilog, change.Stamp(ilog, []change.Draft{change.NewDeleteEntity(false)}, meta("o2", 10), yIDs()))
	out, err = renderYAML(change.KindItem, ilog, nil, nil)
	if err != nil || !strings.Contains(string(out), "\ndeleted: {op: o2, at: \"2026-11-13T10:00:00.000+09:00\", root: false}\n") {
		t.Errorf("a deleted item.yaml =\n%s\n%v", out, err)
	}
}

// 削除の保存は、削除した 2 つのアイテムの item.yaml の deleted を、同じコミットで書き換える。
func TestDeleteWriteRewritesItemYAMLInTheSameCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	before := headCount(t, dir)
	op := port.Operation{Actor: change.User("hanako"), Summary: "HH-1 を削除"}
	err := s.Write(t.Context(), id, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleEpic, change.NewDeleteEntity(true)),
			tx.WriteItems().Append(apptest.SampleTask, change.NewDeleteEntity(false)),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	if headCount(t, dir) != before+1 {
		t.Errorf("commits = %d, want %d", headCount(t, dir), before+1)
	}
	for it, root := range map[item.ID]string{apptest.SampleEpic: "true", apptest.SampleTask: "false"} {
		b, err := os.ReadFile(filepath.Join(dir, "items", string(it), "item.yaml"))
		if err != nil || !strings.Contains(string(b), ", root: "+root+"}\n") || strings.Contains(string(b), "deleted: null") {
			t.Errorf("item.yaml of %s = %s, %v", it, b, err)
		}
	}
}
