package server

import "sync"

// hubBuffer is how many events a subscriber may be behind before it is
// dropped; the GUI then connects again and fetches what it shows.
const hubBuffer = 256

// sseEvent is one of the event types of /api/events.
type sseEvent = any

// Hub hands the events to the connections of /api/events.
type Hub struct {
	mu     sync.Mutex
	subs   map[chan sseEvent]struct{}
	closed bool
}

func newHub() *Hub { return &Hub{subs: map[chan sseEvent]struct{}{}} }

// Subscribe returns the events from now on, and the function that stops
// them. The channel is closed when the subscriber falls behind or the hub
// closes.
func (h *Hub) Subscribe() (<-chan sseEvent, func()) {
	ch := make(chan sseEvent, hubBuffer)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	h.subs[ch] = struct{}{}
	return ch, func() { h.drop(ch) }
}

func (h *Hub) drop(ch chan sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}

// Publish sends e to every subscriber, dropping those that are behind.
func (h *Hub) Publish(e sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// Close ends every subscription, and the ones after.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for ch := range h.subs {
		delete(h.subs, ch)
		close(ch)
	}
}
