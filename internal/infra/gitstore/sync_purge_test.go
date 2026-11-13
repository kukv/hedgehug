package gitstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// sampleStep は、B が HH-2 の下に作る Step(HH-3)。
const sampleStep item.ID = "01SAMPLESTEP00000000000001"

// noRepair は、(1) を動かすための何もしない修復(本番はいつも Repair がある)。
func noRepair(context.Context, port.RepairWriter) error { return nil }

// purgeForTest は、本物の「完全に削除」と同じく、アイテム id と、その下のもの(削除済みを含む)を、
// 1 件ずつ (*tx).purge する(決定 C3a-6)。1 回の保存にする。
func (m machine) purgeForTest(t *testing.T, id item.ID) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "完全に削除"}
	if err := m.stores.Write(t.Context(), m.id, op, func(ptx port.Tx) error {
		x := ptx.(*tx)
		all, err := x.Items().List(apptest.SampleProject)
		if err != nil {
			return err
		}
		for _, g := range withBelow(all, id) {
			it, err := x.Items().Get(g)
			if err != nil {
				return err
			}
			if err := x.purge(change.Ref{Kind: change.KindItem, ID: g}, it.DisplayID("HH")); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// withBelow は、id と、all のうちその下にあるもの。
func withBelow(all []item.Item, id item.ID) []item.ID {
	out := []item.ID{id}
	for changed := true; changed; {
		changed = false
		for _, it := range all {
			if it.Place.Parent != nil && slices.Contains(out, *it.Place.Parent) && !slices.Contains(out, it.ID) {
				out, changed = append(out, it.ID), true
			}
		}
	}
	return out
}

// createStepUnder は、parent の下に Step HH-3 を作る。
func createStepUnder(parent item.ID) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		p := parent
		step := item.Item{
			ID: sampleStep, Kind: item.Step, Rank: rank.First, Title: "入力欄",
			Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &p, Number: 3},
		}
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, 3)),
			tx.WriteItems().Append(step.ID, item.Create(step)),
		)
	}
}

// subjects は、先端から辿れるコミットの件名。
func subjects(t *testing.T, dir string) []string {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	if err := walk(repo, headCommit(t, dir).Hash, func(c *object.Commit) {
		out = append(out, strings.SplitN(c.Message, "\n", 2)[0])
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func count(s []string, v string) int {
	n := 0
	for _, x := range s {
		if x == v {
			n++
		}
	}
	return n
}

// purgedLines は、purged.jsonl の行のうち、target が id のもの。
func purgedLines(t *testing.T, m machine, id item.ID) []string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(files(t, m.dir)["purged.jsonl"], "\n") {
		if strings.Contains(l, `"target":"`+string(id)+`"`) {
			out = append(out, l)
		}
	}
	return out
}

func hasDir(m machine, id item.ID) bool {
	_, err := os.Stat(filepath.Join(m.dir, "items", string(id)))
	return err == nil
}

func TestPurgeInTx(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "HH-2 を完全に削除"}
	err := a.stores.Write(t.Context(), a.id, op, func(ptx port.Tx) error {
		if err := ptx.(*tx).purge(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}, "HH-2"); err != nil {
			return err
		}
		if _, err := ptx.Items().Get(apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Get after purge: %v", err)
		}
		its, err := ptx.Items().List(apptest.SampleProject)
		if err != nil || len(its) != 1 {
			t.Errorf("List after purge = %v, %v", its, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasDir(a, apptest.SampleTask) {
		t.Error("the folder is still there")
	}
	got := purgedLines(t, a, apptest.SampleTask)
	want := regexp.MustCompile(`^\{"id":"01A\d{23}","at":"2026-11-12T09:00:00\.\d{3}\+09:00","by":"hedgehug\[bot\]","actor":"sync","entity":"item","target":"` +
		string(apptest.SampleTask) + `","display":"HH-2"\}$`)
	if len(got) != 1 || !want.MatchString(got[0]) {
		t.Fatalf("purged.jsonl:\n%v\nwant %s", got, want)
	}
	if st := worktreeStatus(t, a.dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
	if s := subjects(t, a.dir); s[0] != "HH-2 を完全に削除" {
		t.Errorf("subject = %q", s[0])
	}
	wantPurgedRead(t, a)
}

// 同じ Tx で完全に削除したものは、同じ Tx で写し直せない(Import は ErrInvalidOperation。Remove したものと同じ)。
func TestImportRefusesWhatWasPurgedInTheSameTx(t *testing.T) {
	a := firstMachine(t, newRemote(t))
	op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "HH-2 を完全に削除"}
	err := a.stores.Write(t.Context(), a.id, op, func(ptx port.Tx) error {
		log, err := ptx.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		if err != nil {
			return err
		}
		if err := ptx.(*tx).purge(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}, "HH-2"); err != nil {
			return err
		}
		if err := ptx.WriteItems().Import(apptest.SampleTask, log, ""); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Import after purge = %v, want ErrInvalidOperation", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasDir(a, apptest.SampleTask) {
		t.Error("the purged folder is back")
	}
}

// wantPurgedRead は、purge が書いた行を、Reader.Purged(A5a の旧 ID の解決が読む)で読めることを確かめる。
func wantPurgedRead(t *testing.T, a machine) {
	t.Helper()
	if err := a.stores.Read(t.Context(), a.id, func(r port.Reader) error {
		ps, err := r.Purged().List()
		if err != nil || len(ps) != 1 || ps[0].Entity != "item" || ps[0].Target != apptest.SampleTask || ps[0].Display != "HH-2" || ps[0].At.IsZero() {
			t.Errorf("Purged = %+v, %v", ps, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// purgedThenStep は、A が parent を完全に削除して送り、B が HH-2 の下に Step を作った状態を作る。
func purgedThenStep(t *testing.T, parent item.ID) (remote port.Remote, a, b machine) {
	t.Helper()
	remote = newRemote(t)
	a = firstMachine(t, remote)
	b = cloneMachine(t, remote, "B", t0)
	b.rep.Repair = noRepair
	a.purgeForTest(t, parent)
	b.write(t, "子を作る", createStepUnder(apptest.SampleTask))
	a.sync(t, remote)
	return remote, a, b
}

const stepPurged = "HH-3 を完全に削除(祖先が完全に削除)"

func TestRepairPurgesChildOfPurgedParent(t *testing.T) {
	remote, a, b := purgedThenStep(t, apptest.SampleTask)
	b.sync(t, remote)
	a.sync(t, port.Remote{URL: remote.URL})
	sameFiles(t, a, b)
	if hasDir(b, sampleStep) {
		t.Error("the step folder is still there")
	}
	if n := len(purgedLines(t, b, apptest.SampleTask)); n != 1 {
		t.Errorf("%d lines for HH-2", n)
	}
	step := purgedLines(t, b, sampleStep)
	if len(step) != 1 || !strings.Contains(step[0], `"by":"hedgehug[bot]","actor":"sync","entity":"item","target":"`+string(sampleStep)+`","display":"HH-3"}`) {
		t.Errorf("lines for HH-3 = %v", step)
	}
	if n := count(subjects(t, a.dir), stepPurged); n != 1 {
		t.Errorf("%d commits %q", n, stepPurged)
	}
}

func TestRepairPurgesUnderPurgedEpic(t *testing.T) {
	remote, a, b := purgedThenStep(t, apptest.SampleEpic)
	b.sync(t, remote)
	a.sync(t, port.Remote{URL: remote.URL})
	sameFiles(t, a, b)
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask, sampleStep} {
		if hasDir(b, id) {
			t.Errorf("%s is still there", id)
		}
		if n := len(purgedLines(t, b, id)); n != 1 {
			t.Errorf("%d lines for %s", n, id)
		}
	}
	if n := count(subjects(t, b.dir), stepPurged); n != 1 {
		t.Errorf("%d commits %q", n, stepPurged)
	}
}

// rejectFirstSend は、b の最初の送信を断り、修復が 1 回の同期で 2 回動くようにする。修復の呼ばれた回数を返す。
func rejectFirstSend(b machine) *int {
	calls := 0
	b.rep.Repair = func(context.Context, port.RepairWriter) error {
		calls++
		return nil
	}
	sends := 0
	b.rep.beforeSend = func() error {
		sends++
		if sends == 1 {
			return errRejected
		}
		return nil
	}
	return &calls
}

// 送信が断られてもう一度修復しても、2 回目は何も書かない。
func TestRepairPurgeWritesOnce(t *testing.T) {
	remote, a, b := purgedThenStep(t, apptest.SampleTask)
	calls := rejectFirstSend(b)
	b.sync(t, remote)
	if *calls != 2 {
		t.Fatalf("repair ran %d times, want 2", *calls)
	}
	if n := len(purgedLines(t, b, sampleStep)); n != 1 {
		t.Errorf("%d lines for HH-3", n)
	}
	if n := count(subjects(t, b.dir), stepPurged); n != 1 {
		t.Errorf("%d commits %q", n, stepPurged)
	}
	a.sync(t, port.Remote{URL: remote.URL})
	sameFiles(t, a, b)
}

func TestRepairPurgeNotOnReceiveOnly(t *testing.T) {
	remote, _, b := purgedThenStep(t, apptest.SampleTask)
	if _, err := b.rep.Sync(t.Context(), b.id, port.Remote{URL: remote.URL, ReceiveOnly: true}); err != nil {
		t.Fatal(err)
	}
	if !hasDir(b, sampleStep) {
		t.Error("the step was purged on a receive-only store")
	}
	if n := len(purgedLines(t, b, sampleStep)); n != 0 {
		t.Errorf("%d lines for HH-3", n)
	}
	if n := count(subjects(t, b.dir), stepPurged); n != 0 {
		t.Errorf("%d commits %q", n, stepPurged)
	}
}

func TestRepairRemovesRelationToPurged(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	a.purgeForTest(t, apptest.SampleTask)
	b.write(t, "関連を足す", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic,
			change.NewAdd("relations", item.Relation{Type: "blocks", To: apptest.SampleTask}),
			change.NewAdd("relations", item.Relation{Type: "relates", To: apptest.SampleTask}))
	})
	a.sync(t, remote)
	calls := rejectFirstSend(b) // 2 回目の修復は何も書かない
	b.sync(t, remote)
	if *calls != 2 {
		t.Fatalf("repair ran %d times, want 2", *calls)
	}
	a.sync(t, port.Remote{URL: remote.URL})
	sameFiles(t, a, b)
	var epic item.Item
	if err := b.stores.Read(t.Context(), b.id, func(r port.Reader) error {
		var err error
		epic, err = r.Items().Get(apptest.SampleEpic)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(epic.Relations) != 0 {
		t.Errorf("relations = %v", epic.Relations)
	}
	const want = "HH-1 の関連を解除(相手が完全に削除)"
	if n := count(subjects(t, b.dir), want); n != 1 {
		t.Errorf("%d commits %q", n, want)
	}
}

func TestPurgedRefRefusesWrites(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	head := headCommit(t, a.dir).Hash
	op := port.Operation{Actor: change.User("taro"), Summary: "完全に削除"}
	err := a.stores.Write(t.Context(), a.id, op, func(ptx port.Tx) error {
		if err := ptx.(*tx).purge(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}, "HH-2"); err != nil {
			return err
		}
		if err := ptx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "x", "y")); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Append after purge: %v", err)
		}
		if err := ptx.WriteItems().Describe(apptest.SampleTask, "本文"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Describe after purge: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasDir(a, apptest.SampleTask) {
		t.Error("the folder came back")
	}
	if headCommit(t, a.dir).Hash == head {
		t.Error("the purge was not committed")
	}
}

// sampleLoose は、B がプロジェクトの直下に作る Epic(HH-3)。
const sampleLoose item.ID = "01SAMPLELOOSE0000000000001"

// ワークスペースとプロジェクトが完全に削除され、もう一方がその下にアイテムを作ったら、そのアイテムも消す。
// ワークスペースが読めないので、purged.jsonl の display は内部 ID。
func TestRepairPurgesUnderPurgedWorkspace(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	b.rep.Repair = noRepair
	op := port.Operation{Actor: change.User("taro"), Summary: "ワークスペースを完全に削除"}
	if err := a.stores.Write(t.Context(), a.id, op, func(ptx port.Tx) error {
		x := ptx.(*tx)
		return errors.Join(
			x.purge(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}, "Web"),
			x.purge(change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}, "HH-P1"),
			x.purge(change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic}, "HH-1"),
			x.purge(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}, "HH-2"),
		)
	}); err != nil {
		t.Fatal(err)
	}
	b.write(t, "Epic を作る", func(tx port.Tx) error {
		epic := item.Item{
			ID: sampleLoose, Kind: item.Epic, Rank: rank.First, Title: "別の Epic",
			Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3},
		}
		return tx.WriteItems().Append(epic.ID, item.Create(epic))
	})
	a.sync(t, remote)
	b.sync(t, remote)
	a.sync(t, port.Remote{URL: remote.URL})
	sameFiles(t, a, b)
	if hasDir(b, sampleLoose) {
		t.Error("the item is still there")
	}
	got := purgedLines(t, b, sampleLoose)
	if len(got) != 1 || !strings.HasSuffix(got[0], `"display":"`+string(sampleLoose)+`"}`) {
		t.Errorf("lines = %v", got)
	}
	want := string(sampleLoose) + " を完全に削除(祖先が完全に削除)"
	if n := count(subjects(t, b.dir), want); n != 1 {
		t.Errorf("%d commits %q", n, want)
	}
}

// repairWriter.Purged は、purged.jsonl の target を返す。ファイルがなければ空。
func TestRepairWriterPurged(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	w := repairWriter{stores: a.stores, root: a.dir}
	if got, err := w.Purged(); err != nil || len(got) != 0 {
		t.Fatalf("Purged before = %v, %v", got, err)
	}
	a.purgeForTest(t, apptest.SampleTask)
	got, err := w.Purged()
	if err != nil || len(got) != 1 || !got[apptest.SampleTask] {
		t.Errorf("Purged = %v, %v, want %s", got, err, apptest.SampleTask)
	}
}
