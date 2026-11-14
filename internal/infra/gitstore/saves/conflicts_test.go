package saves

import (
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// descMerge は、祖先の説明 base から、親 0 と親 1 がそれぞれ説明を texts にして desc の行を足し、合わせる。
// swap なら、合わせの親の順を入れ替える。親 0・親 1 の保存と合わせを返す。説明が空の文字列なら、ファイルを置かない。
func descMerge(t *testing.T, base string, texts [2]string, swap bool) (s0, s1, m plumbing.Hash, r *testRepo) {
	t.Helper()
	r = newRepo(t)
	create := rec("R1", "OP0", t0, change.Create, "")
	d0 := rec("D0", "OP1", t0.Add(time.Minute), change.Desc, "")
	d1 := rec("D1", "OP2", t0.Add(2*time.Minute), change.Desc, "")
	d1.By, d1.Actor = "hanako", change.User("hanako")
	files := [3]map[string]string{
		{itemLog: logOf(t, create)},
		{itemLog: logOf(t, create, d0)},
		{itemLog: logOf(t, create, d1)},
	}
	for i, text := range []string{base, texts[0], texts[1]} {
		if text != "" {
			files[i][itemDesc] = text
		}
	}
	c0 := r.save(files[0], "taro", t0, "OP0")
	s0 = r.save(files[1], "taro", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	s1 = r.save(files[2], "hanako", t0.Add(2*time.Minute), "OP2")
	parents := []plumbing.Hash{s0, s1}
	if swap {
		parents = []plumbing.Hash{s1, s0}
	}
	r.reset(parents[0])
	merged := map[string]string{itemLog: logOf(t, create, d0, d1), itemDesc: texts[0] + "\n-----\n" + texts[1]}
	m = r.save(merged, BotName, t0.Add(3*time.Minute), "", parents...)
	return s0, s1, m, r
}

func TestMergeConflicts(t *testing.T) {
	_, _, m, r := descMerge(t, "a\nb\nc", [2]string{"a\nx\nc", "a\ny\nc"}, false)
	got, err := mergeConflicts(r.commit(m))
	if err != nil || len(got) != 1 {
		t.Fatalf("conflicts = %+v, %v, want 1", got, err)
	}
	c := got[0]
	if c.Ref != itemRef || len(c.Sides[0]) != 1 || c.Sides[0][0].ID != "D0" || len(c.Sides[1]) != 1 || c.Sides[1][0].ID != "D1" {
		t.Errorf("conflict = %+v", c)
	}
}

func TestMergeConflictsOrderFree(t *testing.T) {
	for _, swap := range []bool{false, true} {
		_, _, m, r := descMerge(t, "a\nb\nc", [2]string{"a\nx\nc", "a\ny\nc"}, swap)
		got, err := mergeConflicts(r.commit(m))
		if err != nil || len(got) != 1 {
			t.Errorf("swap=%v: conflicts = %+v, %v, want 1", swap, got, err)
		}
	}
}

func TestMergeNoConflict(t *testing.T) {
	_, _, m, r := descMerge(t, "a\nb\nc", [2]string{"x\nb\nc", "a\nb\ny"}, false)
	if got, err := mergeConflicts(r.commit(m)); err != nil || len(got) != 0 {
		t.Errorf("conflicts = %+v, %v, want none", got, err)
	}
}

func TestMergeOneSide(t *testing.T) {
	_, _, m, r := descMerge(t, "a\nb\nc", [2]string{"a\nx\nc", "a\nb\nc"}, false)
	if got, err := mergeConflicts(r.commit(m)); err != nil || len(got) != 0 {
		t.Errorf("conflicts = %+v, %v, want none", got, err)
	}
}

// 祖先に説明がなく、両側が新しく書いた説明が重なる合わせも、重なりに数える(ない説明は空の本文)。
func TestMergeConflictsNoBaseDescription(t *testing.T) {
	_, _, m, r := descMerge(t, "", [2]string{"x", "y"}, false)
	if got, err := mergeConflicts(r.commit(m)); err != nil || len(got) != 1 {
		t.Errorf("conflicts = %+v, %v, want 1", got, err)
	}
}

// 消えたエンティティがなく、説明だけが重なった合わせも、Pending が Merge で渡す。
func TestPendingPassesConflictMerge(t *testing.T) {
	s0, s1, m, r := descMerge(t, "a\nb\nc", [2]string{"a\nx\nc", "a\ny\nc"}, false)
	j, known := judged(t)
	r.mark(j, s0, s1)
	s, ok := pending(t, r, m, m, known)[m.String()]
	if !ok || !s.Merge || len(s.Conflicts) != 1 || len(s.Removed) != 0 {
		t.Fatalf("merge = %+v, want a merge with one conflict", s)
	}
}
