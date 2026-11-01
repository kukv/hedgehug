package github

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A plain http API on a host that is not loopback gets no request, so no token.
func TestPlainHTTPAPIGetsNoRequest(t *testing.T) {
	called := false
	hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, http.ErrNotSupported
	})}
	_, err := New(config.Host{API: "http://ghe.example.com/api/v3"}, "secret", hc).UserRepos()
	if err == nil || !strings.Contains(err.Error(), "plain http") {
		t.Errorf("err = %v", err)
	}
	if called {
		t.Error("a request was sent")
	}
}

// A loopback http server still works.
func TestLoopbackHTTPAPIWorks(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	if _, err := New(config.Host{API: srv.URL}, "secret", nil).UserRepos(); err != nil {
		t.Fatal(err)
	}
	if got != "Bearer secret" {
		t.Errorf("Authorization = %q", got)
	}
}
