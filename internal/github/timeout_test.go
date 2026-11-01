package github

import (
	"net/http"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
)

// A request takes at most 30 seconds (spec: GitHub API), for the client
// New makes and for the copy it makes of one that is handed in.
func TestClientTimeout(t *testing.T) {
	h := config.Host{API: "http://127.0.0.1:1"}
	if got := New(h, "t", nil).hc.Timeout; got != 30*time.Second {
		t.Errorf("default client timeout = %v", got)
	}
	own := &http.Client{}
	if got := New(h, "t", own).hc.Timeout; got != 30*time.Second {
		t.Errorf("injected client with no timeout: copy has %v", got)
	}
	if own.Timeout != 0 {
		t.Errorf("the injected client was changed: %v", own.Timeout)
	}
	if got := New(h, "t", &http.Client{Timeout: 5 * time.Second}).hc.Timeout; got != 5*time.Second {
		t.Errorf("injected client with its own timeout: copy has %v", got)
	}
}
