package saves

import (
	"slices"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const (
	otherTask change.ID = "01ITEM00000000000000000002"
	otherLog            = "items/" + string(otherTask) + "/log.jsonl"
)

func shas(cs []port.Commit) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.SHA)
	}
	return out
}

func history(t *testing.T, r *testRepo, from plumbing.Hash, seen map[plumbing.Hash]port.Commit) []port.Commit {
	t.Helper()
	recall := func(h plumbing.Hash) (port.Commit, bool) { c, ok := seen[h]; return c, ok }
	remember := func(h plumbing.Hash, c port.Commit) { seen[h] = c }
	got, err := History(r.repo, from, recall, remember)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// 最初のコミット(op なし)とマージ(親 2 つ)は返さない。
func TestHistorySkipsMergeAndFirst(t *testing.T) {
	r := newRepo(t)
	c0 := r.save(map[string]string{"a": "0"}, BotName, t0, "")
	s1 := r.save(map[string]string{itemLog: logOf(t, rec("R1", "OP1", t0.Add(time.Minute), change.Create, ""))}, "taro", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	s2 := r.save(map[string]string{otherLog: logOf(t, rec("R2", "OP2", t0.Add(2*time.Minute), change.Create, ""))}, "hanako", t0.Add(2*time.Minute), "OP2")
	m := r.save(nil, BotName, t0.Add(3*time.Minute), "", s2, s1)
	got := history(t, r, m, map[plumbing.Hash]port.Commit{})
	if g, w := shas(got), want(s2, s1); !slices.Equal(g, w) {
		t.Errorf("History = %v, want %v", g, w)
	}
	if got[0].Op != "OP2" || got[0].Author != "hanako" || got[0].Summary != "保存" || !got[0].At.Equal(t0.Add(2*time.Minute)) {
		t.Errorf("commit = %+v", got[0])
	}
}

// 並びは作者の時刻の新しい順。親子の順ではない。
func TestHistoryOrdersByAuthorTime(t *testing.T) {
	r := newRepo(t)
	r.save(map[string]string{"a": "0"}, BotName, t0, "")
	s1 := r.save(map[string]string{"a": "1"}, "taro", t0.Add(10*time.Minute), "OP1")
	s2 := r.save(map[string]string{"a": "2"}, "hanako", t0.Add(time.Minute), "OP2")
	got := history(t, r, s2, map[plumbing.Hash]port.Commit{})
	if g, w := shas(got), want(s1, s2); !slices.Equal(g, w) {
		t.Errorf("History = %v, want %v", g, w)
	}
}

// 時刻が同じなら SHA の辞書順。
func TestHistoryTiesBySHA(t *testing.T) {
	r := newRepo(t)
	c0 := r.save(map[string]string{"a": "0"}, BotName, t0, "")
	s1 := r.save(map[string]string{"a": "1"}, "taro", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	s2 := r.save(map[string]string{"a": "2"}, "hanako", t0.Add(time.Minute), "OP2")
	m := r.save(nil, BotName, t0.Add(2*time.Minute), "", s2, s1)
	got := shas(history(t, r, m, map[plumbing.Hash]port.Commit{}))
	if !slices.IsSorted(got) || len(got) != 2 {
		t.Errorf("History = %v, want SHA ascending", got)
	}
}

// Changes は、その保存で足した行だけ。
func TestHistoryAddedRows(t *testing.T) {
	r := newRepo(t)
	create := rec("R1", "OP1", t0.Add(time.Minute), change.Create, "")
	set := rec("R2", "OP2", t0.Add(2*time.Minute), change.Set, "title")
	r.save(map[string]string{"a": "0"}, BotName, t0, "")
	r.save(map[string]string{itemLog: logOf(t, create)}, "taro", t0.Add(time.Minute), "OP1")
	s2 := r.save(map[string]string{itemLog: logOf(t, create, set)}, "taro", t0.Add(2*time.Minute), "OP2")
	got := history(t, r, s2, map[plumbing.Hash]port.Commit{})
	if len(got) != 2 || len(got[0].Changes) != 1 || len(got[0].Changes[0].Rows) != 1 || got[0].Changes[0].Rows[0].ID != "R2" {
		t.Fatalf("latest changes = %+v", got)
	}
	if got[0].Changes[0].Ref != itemRef {
		t.Errorf("ref = %+v", got[0].Changes[0].Ref)
	}
	if len(got[1].Changes) != 1 || got[1].Changes[0].Rows[0].ID != "R1" {
		t.Errorf("first save changes = %+v", got[1].Changes)
	}
}

// seen にあるコミットは差分を取らず、その値を返す。新しく読んだものは seen に足す。
func TestHistoryUsesSeen(t *testing.T) {
	r := newRepo(t)
	r.save(map[string]string{"a": "0"}, BotName, t0, "")
	s1 := r.save(map[string]string{"a": "1"}, "taro", t0.Add(time.Minute), "OP1")
	s2 := r.save(map[string]string{"a": "2"}, "taro", t0.Add(2*time.Minute), "OP2")
	seen := map[plumbing.Hash]port.Commit{s1: {SHA: s1.String(), Summary: "覚えた", Op: "OP1", At: t0.Add(time.Minute)}}
	got := history(t, r, s2, seen)
	if len(got) != 2 || got[1].Summary != "覚えた" || got[0].Summary != "保存" {
		t.Errorf("History = %+v", got)
	}
	if _, ok := seen[s2]; !ok {
		t.Error("the newly read commit is not added to seen")
	}
}

// フォルダごと消えたエンティティは Removed、Store の記録に行を足したコミットは StoreLog。
func TestHistoryRemovedAndStoreLog(t *testing.T) {
	r := newRepo(t)
	create := rec("R1", "OP1", t0.Add(time.Minute), change.Create, "")
	r.save(map[string]string{"a": "0"}, BotName, t0, "")
	r.save(map[string]string{itemLog: logOf(t, create), itemDesc: "説明"}, "taro", t0.Add(time.Minute), "OP1")
	s2 := r.save(map[string]string{itemLog: "", itemDesc: ""}, "taro", t0.Add(2*time.Minute), "OP2")
	s3 := r.save(map[string]string{".hedgehug/log.jsonl": logOf(t, rec("S1", "OP3", t0.Add(3*time.Minute), change.Set, "retention_days"))}, "taro", t0.Add(3*time.Minute), "OP3")
	got := history(t, r, s3, map[plumbing.Hash]port.Commit{})
	if len(got) != 3 {
		t.Fatalf("History = %d commits", len(got))
	}
	if got[0].SHA != s3.String() || !got[0].StoreLog || len(got[0].Removed) != 0 || len(got[0].Changes) != 0 {
		t.Errorf("store log commit = %+v", got[0])
	}
	if got[1].SHA != s2.String() || got[1].StoreLog || !slices.Equal(got[1].Removed, []change.Ref{itemRef}) || len(got[1].Changes) != 0 {
		t.Errorf("removed commit = %+v", got[1])
	}
	if got[2].StoreLog || len(got[2].Removed) != 0 {
		t.Errorf("create commit = %+v", got[2])
	}
}

func TestHistoryZeroHash(t *testing.T) {
	r := newRepo(t)
	if got := history(t, r, plumbing.ZeroHash, map[plumbing.Hash]port.Commit{}); len(got) != 0 {
		t.Errorf("History(zero) = %v", got)
	}
}

// 同じ秒の保存は、行の At(ミリ秒)で並べる。op の行がないコミットは作者の時刻。
func TestHistoryOrdersByRowAtWithinASecond(t *testing.T) {
	r := newRepo(t)
	r.save(map[string]string{"a": "0"}, BotName, t0, "")
	a1 := rec("R1", "OP1", t0.Add(5*time.Millisecond), change.Create, "")
	a2 := rec("R2", "OP2", t0.Add(9*time.Millisecond), change.Set, "title")
	s1 := r.save(map[string]string{itemLog: logOf(t, a1)}, "taro", t0, "OP1")
	s2 := r.save(map[string]string{itemLog: logOf(t, a1, a2)}, "taro", t0, "OP2")
	s3 := r.save(map[string]string{"a": "3"}, BotName, t0, "OP3")
	got := history(t, r, s3, map[plumbing.Hash]port.Commit{})
	if len(got) != 3 || got[0].SHA != s2.String() || got[1].SHA != s1.String() || got[2].SHA != s3.String() {
		t.Fatalf("History = %v, want %v", shas(got), want(s2, s1, s3))
	}
	if !got[0].At.Equal(a2.At) || !got[2].At.Equal(t0) {
		t.Errorf("At = %v, %v", got[0].At, got[2].At)
	}
}

// 覚えるコミットは説明の全文を持たない(アクティビティは使わない)。説明だけを変えたエンティティも、行がなければ持たない。
func TestHistoryDropsDescription(t *testing.T) {
	r := newRepo(t)
	create := rec("R1", "OP1", t0.Add(time.Minute), change.Create, "")
	r.save(map[string]string{"a": "0"}, BotName, t0, "")
	r.save(map[string]string{itemLog: logOf(t, create), itemDesc: "説明"}, "taro", t0.Add(time.Minute), "OP1")
	s2 := r.save(map[string]string{itemDesc: "説明を変えた"}, "taro", t0.Add(2*time.Minute), "OP2")
	seen := map[plumbing.Hash]port.Commit{}
	got := history(t, r, s2, seen)
	if len(got) != 2 || len(got[0].Changes) != 0 {
		t.Fatalf("description-only commit changes = %+v", got)
	}
	if len(got[1].Changes) != 1 || got[1].Changes[0].Description != nil || len(got[1].Changes[0].Rows) != 1 {
		t.Errorf("create commit changes = %+v, want the row without the description", got[1].Changes)
	}
	for h, c := range seen {
		for _, e := range c.Changes {
			if e.Description != nil {
				t.Errorf("remembered %s keeps the description %q", h, *e.Description)
			}
		}
	}
}
