package server

// storesChanged is the event that the stores of config.yaml changed: one came
// or went, or its path, color, visibility or account did, or the accounts or
// the default store did. It has no body; the GUI reads GET /api/stores again.
type storesChanged struct{}

// storesWritten is what a write to the stores or the accounts of config.yaml
// through the API does afterwards: it tells the clients, makes the poller
// look now (so that a store that came is opened, and one that went is let
// go), and wakes the sync loop (a store that came is synced). The poller
// tells the clients too when it finds the change itself, so a client may get
// the event twice; reading the stores again is idempotent.
func (s *Server) storesWritten() {
	s.hub.Publish(storesChanged{})
	s.poller.Kick()
	s.loop.Wake()
}
