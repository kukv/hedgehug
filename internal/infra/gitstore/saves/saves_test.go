package saves

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const (
	store port.StoreID = "01STORE0000000000000000001"
	task  change.ID    = "01ITEM00000000000000000001"
)

var t0 = time.Date(2026, 11, 12, 9, 0, 0, 0, time.FixedZone("JST", 9*3600))

// testRepo は、手で保存を作るためのリポジトリ(メモリの中)。
type testRepo struct {
	t    *testing.T
	repo *git.Repository
	wt   *git.Worktree
}

func newRepo(t *testing.T) *testRepo {
	t.Helper()
	fs := memfs.New()
	repo, err := git.Init(memory.NewStorage(), fs)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	return &testRepo{t: t, repo: repo, wt: wt}
}

// save は、files を書いて(空の中身なら消して)1 つのコミットにする。op が空なら Hedgehug-Op を付けない
// (合わせ・最初の保存)。parents が空なら、いまの先端が親になる。
func (r *testRepo) save(files map[string]string, author string, at time.Time, op string, parents ...plumbing.Hash) plumbing.Hash {
	r.t.Helper()
	for p, v := range files {
		if v == "" {
			if _, err := r.wt.Remove(p); err != nil {
				r.t.Fatal(err)
			}
			continue
		}
		if err := util.WriteFile(r.wt.Filesystem, p, []byte(v), 0o644); err != nil {
			r.t.Fatal(err)
		}
		if _, err := r.wt.Add(p); err != nil {
			r.t.Fatal(err)
		}
	}
	msg := "保存\n"
	if op != "" {
		msg = "保存\n\nHedgehug-Op: " + op + "\n"
	}
	sig := &object.Signature{Name: author, Email: author + "@hedgehug.invalid", When: at}
	h, err := r.wt.Commit(msg, &git.CommitOptions{Author: sig, Committer: sig, Parents: parents, AllowEmptyCommits: true})
	if err != nil {
		r.t.Fatal(err)
	}
	return h
}

// reset は、作業ツリーと先端を h に戻す(枝分かれを作るため)。
func (r *testRepo) reset(h plumbing.Hash) {
	r.t.Helper()
	if err := r.wt.Reset(&git.ResetOptions{Commit: h, Mode: git.HardReset}); err != nil {
		r.t.Fatal(err)
	}
}

func (r *testRepo) commit(h plumbing.Hash) *object.Commit {
	r.t.Helper()
	c, err := r.repo.CommitObject(h)
	if err != nil {
		r.t.Fatal(err)
	}
	return c
}

// judged は、判定済みの記録(偽物)と、それを引く known。
func judged(t *testing.T) (*apptest.JudgedSaves, func(port.SaveKey) (port.Match, error)) {
	t.Helper()
	j := apptest.NewJudgedSaves()
	return j, func(k port.SaveKey) (port.Match, error) { return j.Known(t.Context(), store, k) }
}

func (r *testRepo) mark(j *apptest.JudgedSaves, hs ...plumbing.Hash) {
	r.t.Helper()
	keys := make([]port.SaveKey, 0, len(hs))
	for _, h := range hs {
		keys = append(keys, KeyOf(r.commit(h)))
	}
	if err := j.Mark(r.t.Context(), store, keys); err != nil {
		r.t.Fatal(err)
	}
}

func versions(saves []port.ReceivedSave) []string {
	out := make([]string, 0, len(saves))
	for _, s := range saves {
		out = append(out, s.Key.Version)
	}
	return out
}

func want(hs ...plumbing.Hash) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.String())
	}
	return out
}

// logOf は、記録を log.jsonl の中身にする。
func logOf(t *testing.T, rs ...change.Record) string {
	t.Helper()
	var b strings.Builder
	for _, r := range rs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(append(line, '\n'))
	}
	return b.String()
}

func rec(id, op string, at time.Time, typ change.Type, f string) change.Record {
	return change.Record{ID: change.ID(id), Op: change.ID(op), At: at, By: "taro", Actor: change.User("taro"), T: typ, F: f}
}

const (
	itemLog  = "items/" + string(task) + "/log.jsonl"
	itemDesc = "items/" + string(task) + "/description.md"
)

// 判定済みに当たったら辿るのを止める。返すのは古い順(親が先)で、時刻が親より前でも親が先。
func TestUnjudgedStopsAtKnown(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	c0 := r.save(map[string]string{".hedgehug/store.yaml": "store: x\n"}, "hedgehug[bot]", t0, "")
	s1 := r.save(map[string]string{"a": "1"}, "taro", t0.Add(time.Minute), "OP1")
	s2 := r.save(map[string]string{"a": "2"}, "hanako", t0.Add(3*time.Minute), "OP2")
	s3 := r.save(map[string]string{"a": "3"}, "jiro", t0.Add(2*time.Minute), "OP3") // 時計のずれたマシン
	_ = c0
	r.mark(j, s1)
	got, err := Unjudged(r.repo, s3, known)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := versions(got), want(s2, s3); strings.Join(g, ",") != strings.Join(w, ",") {
		t.Errorf("Unjudged = %v, want %v", g, w)
	}
	if got[0].Key.Author != "hanako" || got[0].Key.Op != "OP2" || !got[0].Key.At.Equal(t0.Add(3*time.Minute)) {
		t.Errorf("key = %+v", got[0].Key)
	}
	if none, err := Unjudged(r.repo, plumbing.ZeroHash, known); err != nil || len(none) != 0 {
		t.Errorf("Unjudged(zero) = %v, %v, want none", none, err)
	}
}

// 合わせ(Hedgehug-Op のない保存)は返さないが、その先は辿る。同じ深さは時刻の順。Reachable は合わせも返す。
func TestUnjudgedSkipsMerge(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	c0 := r.save(map[string]string{".hedgehug/store.yaml": "store: x\n"}, "hedgehug[bot]", t0, "")
	s1 := r.save(map[string]string{"a": "1"}, "taro", t0.Add(2*time.Minute), "OP1")
	r.reset(c0)
	s2 := r.save(map[string]string{"b": "1"}, "hanako", t0.Add(time.Minute), "OP2")
	m := r.save(map[string]string{"a": "1"}, "hedgehug[bot]", t0.Add(3*time.Minute), "", s2, s1)
	r.mark(j, c0)
	got, err := Unjudged(r.repo, m, known)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := versions(got), want(s2, s1); strings.Join(g, ",") != strings.Join(w, ",") {
		t.Errorf("Unjudged = %v, want %v", g, w)
	}
	keys, err := Reachable(r.repo, m, known)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Errorf("Reachable = %+v, want the two saves and the merge", keys)
	}
	if none, err := Reachable(r.repo, plumbing.ZeroHash, known); err != nil || len(none) != 0 {
		t.Errorf("Reachable(zero) = %v, %v, want none", none, err)
	}
}

// 保存が足した行は、親の log.jsonl にない id の行。説明を変えていなければ Description は nil。
func TestRowsAddedBySave(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	create := rec("R1", "OP0", t0, change.Create, "")
	assign := rec("R2", "OP1", t0.Add(time.Minute), change.Set, "assignee")
	c0 := r.save(map[string]string{itemLog: logOf(t, create), "members/log.jsonl": logOf(t, rec("M1", "OP0", t0, change.Create, ""))}, "taro", t0, "OP0")
	s1 := r.save(map[string]string{itemLog: logOf(t, create, assign)}, "taro", t0.Add(time.Minute), "OP1")
	r.mark(j, c0)
	got, err := Unjudged(r.repo, s1, known)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Entities) != 1 {
		t.Fatalf("Unjudged = %+v, want one save with one entity", got)
	}
	e := got[0].Entities[0]
	if e.Ref != (change.Ref{Kind: change.KindItem, ID: task}) {
		t.Errorf("ref = %+v", e.Ref)
	}
	if len(e.Rows) != 1 || e.Rows[0].ID != "R2" || e.Rows[0].F != "assignee" {
		t.Errorf("rows = %+v, want only R2", e.Rows)
	}
	if e.Description != nil {
		t.Errorf("description = %q, want nil", *e.Description)
	}
}

// 説明を 2 回変えたら、それぞれの保存の Description は、その時点の本文。
func TestDescriptionAtSave(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	create := rec("R1", "OP0", t0, change.Create, "")
	d1 := rec("R2", "OP1", t0.Add(time.Minute), change.Desc, "")
	d2 := rec("R3", "OP2", t0.Add(2*time.Minute), change.Desc, "")
	c0 := r.save(map[string]string{itemLog: logOf(t, create)}, "taro", t0, "OP0")
	s1 := r.save(map[string]string{itemLog: logOf(t, create, d1), itemDesc: "一つ目 @hanako"}, "taro", t0.Add(time.Minute), "OP1")
	s2 := r.save(map[string]string{itemLog: logOf(t, create, d1, d2), itemDesc: "二つ目"}, "taro", t0.Add(2*time.Minute), "OP2")
	r.mark(j, c0)
	got, err := Unjudged(r.repo, s2, known)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := versions(got), want(s1, s2); strings.Join(g, ",") != strings.Join(w, ",") {
		t.Fatalf("Unjudged = %v, want %v", g, w)
	}
	for i, text := range []string{"一つ目 @hanako", "二つ目"} {
		es := got[i].Entities
		if len(es) != 1 || es[0].Description == nil || *es[0].Description != text {
			t.Errorf("save %d entities = %+v, want description %q", i, es, text)
		}
	}
}

// 作者・時刻・op が同じで印の違う保存(書き換えられた remote から受け取ったもの)は外し、その親は返す。
func TestKnownByOpAfterRewrite(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	c0 := r.save(map[string]string{".hedgehug/store.yaml": "store: x\n"}, "hedgehug[bot]", t0, "")
	s1 := r.save(map[string]string{"a": "1"}, "taro", t0.Add(time.Minute), "OP1")
	s2 := r.save(map[string]string{"a": "2"}, "hanako", t0.Add(2*time.Minute), "OP2")
	r.mark(j, c0)
	k := KeyOf(r.commit(s2))
	k.Version = strings.Repeat("0", 40)
	if err := j.Mark(t.Context(), store, []port.SaveKey{k}); err != nil {
		t.Fatal(err)
	}
	got, err := Unjudged(r.repo, s2, known)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := versions(got), want(s1); strings.Join(g, ",") != strings.Join(w, ",") {
		t.Errorf("Unjudged = %v, want %v", g, w)
	}
}
