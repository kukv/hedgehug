package server

import "testing"

func TestPresence(t *testing.T) {
	changes := 0
	p := newPresence(func() { changes++ })
	if p.Visible() {
		t.Fatal("no client is visible")
	}
	a := p.Connect()
	b := p.Connect()
	if a == b || !p.Visible() {
		t.Fatalf("clients %q %q, visible %v", a, b, p.Visible())
	}
	if !p.Set(a, false) || !p.Visible() {
		t.Error("b is still visible")
	}
	p.Set(b, false)
	if p.Visible() {
		t.Error("no tab is visible")
	}
	p.Set(b, true)
	p.Disconnect(b)
	if p.Visible() {
		t.Error("a closed tab does not count")
	}
	if p.Set("nope", true) {
		t.Error("an unknown client is refused")
	}
	if changes == 0 {
		t.Error("onChange was not called")
	}
}
