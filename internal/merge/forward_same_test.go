package merge

import (
	"testing"
	"time"
)

// Two forwards that name the same store lose nothing, whatever else differs
// in the bytes of the files.
func TestMergeTwoForwardsOfTheSameStoreRecordNothing(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: old")
	plain := forwardFile(taskU, storeB)
	spaced := []byte("---\nmoved_to:   " + storeB + "\nulid: " + taskU + "\n---\n\n")
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	o := Side{Data: plain, Author: "a", Time: at}
	th := Side{Data: spaced, Author: "b", Time: at.Add(time.Minute)}
	r := mergeTask(t, side(base, ""), o, th)
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %+v", r.Overwritten)
	}
}
