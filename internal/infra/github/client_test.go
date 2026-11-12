package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

const fakeToken = "fake-token"

var now = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)

func newClient(t *testing.T, h http.HandlerFunc) (*Client, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	var skews []time.Duration
	c := New(func(d time.Duration) { skews = append(skews, d) })
	c.BaseURL = srv.URL
	c.Now = func() time.Time { return now }
	return c, &skews
}

func TestViewer(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" || r.Header.Get("Authorization") != "Bearer "+fakeToken ||
			r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"login":"taro","id":1}`))
	})
	login, err := c.Viewer(context.Background(), fakeToken)
	if err != nil || login != "taro" {
		t.Errorf("Viewer = %q, %v", login, err)
	}
}

func TestFailures(t *testing.T) {
	cases := []struct {
		name  string
		h     http.HandlerFunc
		kind  ghcall.Kind
		reset time.Time
	}{
		{"401", status(401, nil), ghcall.TokenRejected, time.Time{}},
		{"403 permission", status(403, nil), ghcall.Forbidden, time.Time{}},
		{"404", status(404, nil), ghcall.NotFound, time.Time{}},
		{"500", status(500, nil), ghcall.Unreachable, time.Time{}},
		{"rate limit with reset", status(403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1794484800"}), ghcall.RateLimited, time.Unix(1794484800, 0)},
		{"retry after seconds", status(429, map[string]string{"Retry-After": "90"}), ghcall.RateLimited, now.Add(90 * time.Second)},
		{"rate limit unknown time", status(429, nil), ghcall.RateLimited, time.Time{}},
		{"broken json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) }, ghcall.Unreachable, time.Time{}},
	}
	for _, c := range cases {
		client, _ := newClient(t, c.h)
		_, err := client.Viewer(context.Background(), fakeToken)
		var f *ghcall.Failure
		if !ghcall.Is(err, c.kind) {
			t.Errorf("%s: err = %v, want %s", c.name, err, c.kind)
			continue
		}
		f = err.(*ghcall.Failure)
		if !f.ResetAt.Equal(c.reset) {
			t.Errorf("%s: ResetAt = %v, want %v", c.name, f.ResetAt, c.reset)
		}
		if strings.Contains(err.Error(), fakeToken) {
			t.Errorf("%s: the error shows the token", c.name)
		}
	}
}

func TestTimeoutIsUnreachable(t *testing.T) {
	release := make(chan struct{})
	c, _ := newClient(t, func(http.ResponseWriter, *http.Request) { <-release })
	defer close(release)
	c.HTTP.Timeout = 50 * time.Millisecond
	_, err := c.Viewer(context.Background(), fakeToken)
	if !ghcall.Is(err, ghcall.Unreachable) || strings.Contains(err.Error(), fakeToken) {
		t.Errorf("err = %v", err)
	}
}

func TestSkewIsReportedOnEveryAnswer(t *testing.T) {
	c, skews := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Date", now.Add(6*time.Minute).Format(http.TimeFormat))
		http.Error(w, "", http.StatusUnauthorized)
	})
	_, _ = c.Viewer(context.Background(), fakeToken)
	if len(*skews) != 1 || (*skews)[0] != 6*time.Minute {
		t.Errorf("skews = %v", *skews)
	}
}

func TestDefaults(t *testing.T) {
	c := New(nil)
	if c.BaseURL != "https://api.github.com" || c.HTTP.Timeout != 30*time.Second {
		t.Errorf("New = %+v", c)
	}
}

func status(code int, headers map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		http.Error(w, `{"message":"no"}`, code)
	}
}
