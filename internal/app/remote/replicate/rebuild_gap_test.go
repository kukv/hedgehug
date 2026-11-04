package replicate

import (
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// A write to .hedgehug/local of the old store between the last look and the
// rename is carried over after the rename; the inbox's lines are joined with
// no line twice.
func TestSwapCarriesLocalWritesOfTheGap(t *testing.T) {
	// 直列: パッケージ変数 beforeRename の差し替え
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	old := c.stores[1]
	line := `{"ulid":"` + model.NewULID().String() + `","kind":"watch","task":"` + sc.keep.ULID.String() + `","commit":"x","author":"","to":"github.com/u1","time":"2026-11-02T00:00:00Z"}` + "\n"
	prev := beforeRename
	beforeRename = func() {
		if err := old.WriteLocal("late.json", []byte("late\n")); err != nil {
			t.Error(err)
		}
		if err := old.WriteLocal("inbox.jsonl", []byte(line)); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { beforeRename = prev })
	c.swap(sc.rb)
	s := c.stores[1]
	if b, err := s.ReadLocal("late.json"); err != nil || string(b) != "late\n" {
		t.Errorf("late.json = %q, %v", b, err)
	}
	b, err := s.ReadLocal("inbox.jsonl")
	if err != nil || strings.Count(string(b), line) != 1 || !strings.Contains(string(b), `"rewrite_applied"`) {
		t.Errorf("inbox.jsonl = %q, %v", b, err)
	}
	if left, err := store.FindLeftover(s.Dir()); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
}

// A commit made to the old store between the check and the rename would
// vanish with the old directory: the swap finds it after the first rename,
// puts the old store back and tells the rebuild is stale.
func TestSwapFindsACommitOfTheGap(t *testing.T) {
	// 直列: パッケージ変数 beforeRename の差し替え
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	dir := c.stores[1].Dir()
	if err := c.stores[1].Repo().Close(); err != nil {
		t.Fatal(err)
	}
	prev := beforeRename
	beforeRename = func() {
		old, err := store.Open(dir)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = old.Repo().Close() }()
		if _, err := old.Update(c.op(1, "update", sc.keep.ULID), func(tx *store.Tx) error {
			x, err := tx.Task(sc.keep.ULID)
			if err != nil {
				return err
			}
			x.Title = "edited in the gap"
			return tx.PutTask(x)
		}); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { beforeRename = prev })
	if err := Swap(dir, sc.rb); !errors.Is(err, ErrRebuildStale) {
		t.Fatalf("Swap = %v, want ErrRebuildStale", err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.stores[1] = s
	if x := c.task(1, sc.keep.ULID); x == nil || x.Title != "edited in the gap" {
		t.Errorf("the commit of the gap is lost: %+v", x)
	}
	if left, err := store.FindLeftover(dir); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
}
