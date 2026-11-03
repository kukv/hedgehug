package server

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// Presence is whether the GUI is on screen: a client per connection to
// /api/events, visible until it says otherwise. With no client visible,
// nothing is.
type Presence struct {
	mu       sync.Mutex
	clients  map[string]bool // id → visible
	onChange func()
}

func newPresence(onChange func()) *Presence {
	if onChange == nil {
		onChange = func() {}
	}
	return &Presence{clients: map[string]bool{}, onChange: onChange}
}

// Connect adds a client, visible, and returns its id.
func (p *Presence) Connect() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	p.mu.Lock()
	p.clients[id] = true
	p.mu.Unlock()
	p.onChange()
	return id
}

// Disconnect removes the client id.
func (p *Presence) Disconnect(id string) {
	p.mu.Lock()
	delete(p.clients, id)
	p.mu.Unlock()
	p.onChange()
}

// Set sets whether the client id is visible; false for an unknown client.
func (p *Presence) Set(id string, visible bool) bool {
	p.mu.Lock()
	_, ok := p.clients[id]
	if ok {
		p.clients[id] = visible
	}
	p.mu.Unlock()
	if ok {
		p.onChange()
	}
	return ok
}

// Visible reports whether a client is visible.
func (p *Presence) Visible() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, v := range p.clients {
		if v {
			return true
		}
	}
	return false
}
