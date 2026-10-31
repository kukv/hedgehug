package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/github"
)

func fakeHost(t *testing.T, st fakegh.State) (*fakegh.Server, config.Host, *atomic.Int32) {
	t.Helper()
	s := fakegh.New(st)
	var refreshes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/oauth/access_token" {
			_ = r.ParseForm()
			if r.PostForm.Get("grant_type") == "refresh_token" {
				refreshes.Add(1)
			}
		}
		s.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return s, config.Host{Host: "github.com", API: srv.URL, Web: srv.URL, ClientID: "Iv23x"}, &refreshes
}

func TestDeviceLogin(t *testing.T) {
	zero := 0
	_, h, _ := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero, AutoApprove: "alice"})
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var shown string
	dt, err := DeviceLogin(h, Options{Now: func() time.Time { return now }, Sleep: func(time.Duration) {}}, func(dc *github.DeviceCode) { shown = dc.UserCode })
	if err != nil || dt.AccessToken == "" || shown == "" {
		t.Fatalf("DeviceLogin = %+v, %v (shown %q)", dt, err, shown)
	}
	if !dt.ExpiresAt.Equal(now.Add(8*time.Hour)) || dt.RefreshExpiresAt.Before(now.Add(4000*time.Hour)) {
		t.Errorf("expiry = %v, %v", dt.ExpiresAt, dt.RefreshExpiresAt)
	}
	if _, err := DeviceLogin(config.Host{Host: "ghe", Web: h.Web}, Options{}, func(*github.DeviceCode) {}); err == nil || !strings.Contains(err.Error(), "client_id") {
		t.Errorf("no client ID = %v", err)
	}
}

func TestDeviceLoginWaits(t *testing.T) {
	zero := 0
	s, h, _ := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero})
	var codes []string
	var sleeps int
	opt := Options{Sleep: func(time.Duration) {
		sleeps++
		if sleeps == 2 {
			// The user approves while the CLI waits.
			for _, c := range codes {
				_ = s.ApproveDevice(c, "alice")
			}
		}
	}}
	dt, err := DeviceLogin(h, opt, func(dc *github.DeviceCode) { codes = append(codes, dc.UserCode) })
	if err != nil || dt.AccessToken == "" || sleeps < 2 {
		t.Errorf("DeviceLogin = %+v, %v after %d sleeps", dt, err, sleeps)
	}
}

func TestDeviceLoginExpires(t *testing.T) {
	zero := 0
	_, h, _ := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero})
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	// Time runs past the life of the code while the user never approves.
	opt := Options{Now: func() time.Time { return now }, Sleep: func(time.Duration) { now = now.Add(time.Hour) }}
	_, err := DeviceLogin(h, opt, func(*github.DeviceCode) {})
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("never approved = %v", err)
	}
}

// tokenServer starts the flow with a fixed device code and answers the token
// endpoint with the bodies in turn; the last one repeats.
func tokenServer(t *testing.T, bodies ...map[string]any) config.Host {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/login/device/code" {
			_ = json.NewEncoder(w).Encode(map[string]any{"device_code": "dc", "user_code": "ABCD-1234", "verification_uri": "x", "expires_in": 900, "interval": 5})
			return
		}
		i := min(int(n.Add(1))-1, len(bodies)-1)
		_ = json.NewEncoder(w).Encode(bodies[i])
	}))
	t.Cleanup(srv.Close)
	return config.Host{Host: "github.com", API: srv.URL, Web: srv.URL, ClientID: "Iv23x"}
}

func TestDeviceLoginSlowDown(t *testing.T) {
	h := tokenServer(t,
		map[string]any{"error": "authorization_pending"},
		map[string]any{"error": "slow_down"},
		map[string]any{"access_token": "ghu_1", "refresh_token": "ghr_1", "expires_in": 28800, "refresh_token_expires_in": 15811200},
	)
	var sleeps []time.Duration
	dt, err := DeviceLogin(h, Options{Sleep: func(d time.Duration) { sleeps = append(sleeps, d) }}, func(*github.DeviceCode) {})
	if err != nil || dt.AccessToken != "ghu_1" {
		t.Fatalf("DeviceLogin = %+v, %v", dt, err)
	}
	if len(sleeps) != 2 || sleeps[0] != 5*time.Second || sleeps[1] != 10*time.Second {
		t.Errorf("sleeps = %v, want [5s 10s]", sleeps)
	}
}

func TestDeviceLoginErrors(t *testing.T) {
	for code, want := range map[string]string{
		"access_denied":                "denied",
		"expired_token":                "expired",
		"incorrect_client_credentials": "incorrect_client_credentials",
	} {
		h := tokenServer(t, map[string]any{"error": code})
		_, err := DeviceLogin(h, Options{Sleep: func(time.Duration) {}}, func(*github.DeviceCode) {})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s = %v, want %q", code, err, want)
		}
	}
}

func TestDeviceRefresh(t *testing.T) {
	zero := 0
	_, h, refreshes := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero, AutoApprove: "alice"})
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	k := newMem()
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	opt := Options{Keyring: k, Host: h, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, LockFile: filepath.Join(t.TempDir(), "auth.lock")}
	dt, err := DeviceLogin(h, opt, func(*github.DeviceCode) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveDeviceToken(a, dt, k); err != nil {
		t.Fatal(err)
	}
	if tok, err := Token(a, opt); err != nil || tok != dt.AccessToken || refreshes.Load() != 0 {
		t.Errorf("fresh token = %q, %v, %d refreshes", tok, err, refreshes.Load())
	}
	now = now.Add(8*time.Hour - 4*time.Minute) // within 5 minutes of the end
	var wg sync.WaitGroup
	toks := make([]string, 4)
	for i := range toks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			toks[i], _ = Token(a, opt)
		}()
	}
	wg.Wait()
	if refreshes.Load() != 1 {
		t.Errorf("refreshes = %d, want 1 for 4 callers at once", refreshes.Load())
	}
	for _, tok := range toks {
		if tok == "" || tok == dt.AccessToken || tok != toks[0] {
			t.Errorf("tokens = %q", toks)
		}
	}
	saved, _ := LoadDeviceToken(a, k)
	if saved.AccessToken != toks[0] {
		t.Errorf("saved = %+v", saved)
	}
	// A refresh token that GitHub no longer takes asks for a new login.
	saved.RefreshToken = "ghr_gone"
	saved.ExpiresAt = now
	_ = SaveDeviceToken(a, saved, k)
	if _, err := Token(a, opt); err == nil || !strings.Contains(err.Error(), "hedgehug auth login a") {
		t.Errorf("bad refresh = %v", err)
	}
}

func TestDeviceTokenEdges(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	k := newMem()
	opt := Options{Keyring: k, Now: func() time.Time { return now }}
	if _, err := Token(a, opt); err == nil || !strings.Contains(err.Error(), "hedgehug auth login a") {
		t.Errorf("not logged in = %v", err)
	}
	// A due refresh without a lock file is a bug of the caller.
	_ = SaveDeviceToken(a, DeviceToken{AccessToken: "x", RefreshToken: "r", ExpiresAt: now, RefreshExpiresAt: now.Add(-time.Hour)}, k)
	if _, err := Token(a, opt); err == nil || !strings.Contains(err.Error(), "LockFile") {
		t.Errorf("no lock file = %v", err)
	}
	// Past the end of the refresh token: no request, a new login.
	opt.LockFile = filepath.Join(t.TempDir(), "auth.lock")
	if _, err := Token(a, opt); err == nil || !strings.Contains(err.Error(), "expired") || !strings.Contains(err.Error(), "hedgehug auth login a") {
		t.Errorf("refresh expired = %v", err)
	}
}
