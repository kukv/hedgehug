package main

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/lib/plural"
)

// progressInterval is how long the work runs before the first line, and the
// least time between two lines.
const progressInterval = time.Second

// newProgress makes the Session.Progress of the command: it writes
// "Indexing the history of <store>: <done>/<total> commits" to w, once the
// work of a store has run for a second and then at most once a second. When
// the work ends, the last line is written if any line was.
func newProgress(w io.Writer, clock func() time.Time) func(store string, done, total int) {
	type state struct {
		start, last time.Time
		printed     bool
	}
	var mu sync.Mutex
	states := map[string]*state{}
	return func(store string, done, total int) {
		mu.Lock()
		defer mu.Unlock()
		now := clock()
		st := states[store]
		if st == nil {
			st = &state{start: now}
			states[store] = st
		}
		finished := done >= total
		if finished && !st.printed {
			delete(states, store)
			return
		}
		if !finished && (now.Sub(st.start) < progressInterval || (st.printed && now.Sub(st.last) < progressInterval)) {
			return
		}
		_, _ = fmt.Fprintf(w, "Indexing the history of %s: %d/%s\n", store, done, plural.Count(total, "commit"))
		st.printed, st.last = true, now
		if finished {
			delete(states, store)
		}
	}
}
