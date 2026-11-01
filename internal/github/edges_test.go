package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

func answer(t *testing.T, code int, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(config.Host{API: srv.URL}, "t", nil)
}

func TestStatusEdges(t *testing.T) {
	// A 403 that is not a rate limit is ErrForbidden, with or without a message.
	if _, err := answer(t, 403, `{"message":"Resource not accessible"}`).User(); !errors.Is(err, ErrForbidden) || !strings.Contains(err.Error(), "Resource not accessible") {
		t.Errorf("403 = %v", err)
	}
	_, err := answer(t, 403, ``).User()
	var rl *RateLimitError
	if !errors.Is(err, ErrForbidden) || errors.As(err, &rl) {
		t.Errorf("a plain 403 is ErrForbidden, not a rate limit: %v", err)
	}
	// A 422 other than "already exists" is not ErrExists.
	_, err = answer(t, 422, `{"message":"Validation Failed","errors":[{"message":"name is too long"}]}`).CreateRepo("o", "n", false)
	if err == nil || errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "422") {
		t.Errorf("422 = %v", err)
	}
	if _, err := answer(t, 500, `not json`).User(); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("500 = %v", err)
	}
	if _, err := answer(t, 200, `not json`).User(); err == nil {
		t.Error("a 200 that is not JSON is an error")
	}
}

func TestCreateRepoForUser(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"full_name":"alice/x"}`))
	}))
	defer srv.Close()
	if _, err := New(config.Host{API: srv.URL}, "t", nil).CreateRepo("alice", "x", true); err != nil || path != "/user/repos" {
		t.Errorf("path %q, err %v", path, err)
	}
}

func TestDeviceErrors(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()
	if _, err := RequestDeviceCode(nil, bad.URL, "c"); err == nil {
		t.Error("RequestDeviceCode on a 502 is an error")
	}
	if _, err := PollToken(nil, bad.URL, "c", "d"); err == nil {
		t.Error("PollToken on a 502 is an error")
	}
	if _, err := RefreshToken(nil, bad.URL, "c", "r"); err == nil {
		t.Error("RefreshToken on a 502 is an error")
	}
	refused := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":"incorrect_client_credentials"}`))
	}))
	defer refused.Close()
	if _, err := RequestDeviceCode(nil, refused.URL, "c"); err == nil || !strings.Contains(err.Error(), "incorrect_client_credentials") {
		t.Errorf("RequestDeviceCode refused = %v", err)
	}
	tr, err := RefreshToken(nil, refused.URL, "c", "r")
	if err != nil || tr.Error != "incorrect_client_credentials" || tr.AccessToken != "" {
		t.Errorf("RefreshToken refused = %+v, %v", tr, err)
	}
}
