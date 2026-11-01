package server

import "testing"

func TestHub(t *testing.T) {
	t.Parallel()
	h := newHub()
	a, cancelA := h.Subscribe()
	b, cancelB := h.Subscribe()
	h.Publish(viewsChanged{Store: "me"})
	for _, ch := range []<-chan sseEvent{a, b} {
		if e := <-ch; e.(viewsChanged).Store != "me" {
			t.Errorf("event = %+v", e)
		}
	}
	cancelA()
	h.Publish(viewsChanged{Store: "x"})
	if e := <-b; e.(viewsChanged).Store != "x" {
		t.Errorf("event = %+v", e)
	}
	// A subscriber that does not read is dropped (its channel closed), not
	// waited for.
	for range hubBuffer + 1 {
		h.Publish(viewsChanged{})
	}
	for range b {
	}
	cancelB() // after the drop: does nothing
	h.Close()
	c, _ := h.Subscribe()
	if _, ok := <-c; ok {
		t.Error("a subscription after Close is closed")
	}
}
