package server

import (
	"testing"
)

// drainEvents reads what the hub has put in ch, without waiting. A poll
// publishes while it runs, so what a poll made is in ch when it returns.
func drainEvents(ch <-chan sseEvent) []sseEvent {
	var out []sseEvent
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, e)
		default:
			return out
		}
	}
}

// countStoresChanged is how many stores.changed are in evs.
func countStoresChanged(evs []sseEvent) int {
	n := 0
	for _, e := range evs {
		if _, ok := e.(storesChanged); ok {
			n++
		}
	}
	return n
}

// stores.changed goes out on /api/events with an empty body.
func TestStoresChangedEvent(t *testing.T) {
	t.Parallel()
	ts := startedServer(t)
	es := ts.events()
	es.next(t, "hello")
	ts.srv.storesWritten()
	if got := string(es.next(t, "stores.changed")); got != "{}" {
		t.Errorf("data = %s; want {}", got)
	}
}
