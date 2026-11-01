package ops

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/sync"
)

// A rewrite that was prepared and confirmed syncs the store once: the sync
// of PrepareRewrite, not a second one after the name was typed.
func TestRewriteHistorySyncsOnceAfterAConfirmation(t *testing.T) {
	f := newRewriteFx(t, false)
	f.moved(t)
	a := f.org()
	base := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	a.Now = func() time.Time { return base }
	plan, err := a.PrepareRewrite(RewriteInput{Arg: "API-2"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	st, err := sync.ReadState(s)
	if err != nil || !st.LastAttempt.Equal(base) {
		t.Fatalf("the prepare synced at %v, %v; want %v", st.LastAttempt, err, base)
	}

	a.Now = func() time.Time { return base.Add(time.Hour) }
	if _, err := a.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a", Confirmed: plan.ULIDs()}); err != nil {
		t.Fatal(err)
	}
	s, err = a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if st, err = sync.ReadState(s); err != nil || !st.LastAttempt.Equal(base) {
		t.Errorf("last sync attempt = %v, %v; want %v (no second sync)", st.LastAttempt, err, base)
	}

	// Without a confirmation from PrepareRewrite the rewrite syncs itself.
	g := newRewriteFx(t, false)
	g.moved(t)
	b := g.org()
	b.Now = func() time.Time { return base.Add(2 * time.Hour) }
	if _, err := b.RewriteHistory(RewriteInput{Arg: "API-2", Typed: "org-a"}); err != nil {
		t.Fatal(err)
	}
	s, err = b.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if st, err = sync.ReadState(s); err != nil || !st.LastAttempt.Equal(base.Add(2*time.Hour)) {
		t.Errorf("last sync attempt = %v, %v; want the rewrite's own sync at %v", st.LastAttempt, err, base.Add(2*time.Hour))
	}
}
