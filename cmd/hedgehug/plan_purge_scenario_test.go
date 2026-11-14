//go:build !e2e

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
)

// 完全に削除したスプリントの、合わせでの行の取り除き(C5a-7)と、それを指すアイテムの修復 (8) が同じ同期で起きる場面を、
// 本物の gitstore と repairing.Run と state.db の受信箱で確かめる(決定 C5a-12)。A のログイン名は taro、B は hanako。

// purgeSprintAs は、本物の「完全に削除」と同じく、ワークスペースのログからスプリント id の行を除いて書き直し、
// purged.jsonl に entity が sprint で display が name の行を足して、login の 1 つのコミットにする(完全な削除の
// 書く口はまだない。gitstore のテストの purgePlanForTest と同じ形)。
func purgeSprintAs(t *testing.T, m machine, login string, id change.ID, name string) {
	t.Helper()
	logPath := filepath.Join(m.dir, "workspaces", string(apptest.SampleWorkspace), "log.jsonl")
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, filemerge.DropPurgedPlan(log, map[string]bool{string(id): true}), 0o600); err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(map[string]string{
		"id": "01PURGEPLAN000000000000001", "at": scenarioStart.Format(change.TimeLayout), "by": login,
		"actor": "user", "entity": "sprint", "target": string(id), "display": name,
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(m.dir, "purged.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	commitAll(t, m.dir, login, "完全に削除")
}

// commitAll は、作業ツリーの変更をすべて、login の 1 つのコミットにする。
func commitAll(t *testing.T, dir, login, subject string) {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: login, Email: login + "@example.invalid", When: scenarioStart}
	if _, err := wt.Commit(subject, &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
}

// subjectCount は、先端から辿れるコミットのうち、件名が subject のものの数。
func subjectCount(t *testing.T, dir, subject string) int {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	iter, err := repo.Log(&git.LogOptions{From: head(t, dir)})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	if err := iter.ForEach(func(c *object.Commit) error {
		if strings.SplitN(c.Message, "\n", 2)[0] == subject {
			n++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

// A が Sprint 1 を作って同期し、B が受け取る。B が Sprint 1 を完全に削除し、A が同時に Sprint 1 の名前を変えて HH-2 を
// Sprint 1 に入れる。A が同期し、B が同期すると、B の合わせが Sprint 1 の行を取り除き、同じ同期の修復が HH-2 を
// バックログに戻す。A が同期して受け取ると、A(名前を変えて HH-2 を入れた人)の受信箱に、取り除いた行の通知と
// 修復 (8) の通知が 1 行ずつ。B には作らない。2 台のファイルは同じ。
func TestRepairNoticePurgedSprint(t *testing.T) {
	remote, a, b := noticePair(t)
	one := planned(sprintOne, "Sprint 1", "2026-11-16", "2026-11-29")
	sprintAs(t, a, "taro", planning.Sprint{}, one)
	a.sync(t, remote)
	b.sync(t, remote)
	purgeSprintAs(t, b, "hanako", sprintOne, "Sprint 1")
	renamed := one
	renamed.Name = "Sprint 1 改"
	sprintAs(t, a, "taro", one, renamed)
	taskAs(t, a, "taro", toSprint(sprintOne))
	a.sync(t, remote)
	b.sync(t, remote)
	a.sync(t, remote)
	sameFiles(t, a, b)
	wsLog := "workspaces/" + string(apptest.SampleWorkspace) + "/log.jsonl"
	for _, m := range []machine{a, b} {
		if s := m.item(t, apptest.SampleTask).Sprint; s != "" {
			t.Errorf("HH-2 sprint on %s = %q, want empty", m.dir, s)
		}
		if strings.Contains(files(t, m.dir)[wsLog], string(sprintOne)) {
			t.Errorf("%s: the log still has %s", m.dir, sprintOne)
		}
	}
	const subject = "HH-2 をバックログに戻す(スプリントが完全に削除)"
	if n := subjectCount(t, b.dir, subject); n != 1 {
		t.Errorf("%d commits %q", n, subject)
	}
	wantPurgedSprintNotices(t, a)
	wantNoRepair(t, b)
}

// wantPurgedSprintNotices は、m の受信箱の「同期の修復」が、Sprint 1 の取り除いた行の通知と、HH-2 の修復 (8) の
// 通知の 1 行ずつなことを確かめる。
func wantPurgedSprintNotices(t *testing.T, m machine) {
	t.Helper()
	want := map[string]string{
		"repair:purged:" + string(sprintOne):                        "「Sprint 1」は完全に削除されたため、あなたの変更は反映されませんでした",
		"repair:planref:" + string(apptest.SampleTask) + ":sprint:": "スプリント「Sprint 1」は完全に削除されたため、HH-2 をバックログに戻しました",
	}
	rows := m.notices(t, port.NoticeRepair)
	if len(rows) != len(want) {
		t.Fatalf("repair rows on %s = %+v, want %d", m.dir, rows, len(want))
	}
	for _, r := range rows {
		key := r.Source
		if strings.HasPrefix(key, "repair:planref:") {
			key = key[:strings.LastIndex(key, ":")+1]
		}
		if text, ok := want[key]; !ok || r.Text != text || r.Sender != "" {
			t.Errorf("repair row on %s = %+v", m.dir, r)
		}
		delete(want, key)
	}
}
