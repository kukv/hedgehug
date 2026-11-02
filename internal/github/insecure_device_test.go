package github

import (
	"net/http"
	"strings"
	"testing"
)

// The device flow posts secrets (device code, refresh token): never over
// plain http to a host that is not loopback, and the userinfo is not echoed.
func TestDeviceFlowPlainHTTPGetsNoRequest(t *testing.T) {
	t.Parallel()
	called := false
	hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, http.ErrNotSupported
	})}
	web := "http://user:pw@ghe.example.com"
	for name, f := range map[string]func() error{
		"code":    func() error { _, err := RequestDeviceCode(hc, web, "cid"); return err },
		"poll":    func() error { _, err := PollToken(hc, web, "cid", "dc"); return err },
		"refresh": func() error { _, err := RefreshToken(hc, web, "cid", "rt"); return err },
	} {
		err := f()
		if err == nil || !strings.Contains(err.Error(), "plain http") || strings.Contains(err.Error(), "pw") || strings.Contains(err.Error(), "user") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if called {
		t.Error("a request was sent")
	}
}
