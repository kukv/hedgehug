package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/store"
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// barrierKeyring holds the first n Gets until all n have come, so n callers
// all see the token as due before any of them takes the lock.
type barrierKeyring struct {
	*memKeyring
	n    int32
	seen atomic.Int32
	on   atomic.Bool // the barrier holds only while on
	all  chan struct{}
	fail error // Set returns this when not nil
}

func newBarrier(n int) *barrierKeyring {
	return &barrierKeyring{memKeyring: newMem(), n: int32(n), all: make(chan struct{})}
}

func (b *barrierKeyring) Get(service, user string) (string, error) {
	v, err := b.memKeyring.Get(service, user)
	if !b.on.Load() {
		return v, err
	}
	if c := b.seen.Add(1); c < b.n {
		<-b.all
	} else if c == b.n {
		close(b.all)
	}
	return v, err
}

func (b *barrierKeyring) Set(service, user, secret string) error {
	if b.fail != nil {
		return b.fail
	}
	return b.memKeyring.Set(service, user, secret)
}

func TestDeviceRefresh(t *testing.T) {
	t.Parallel()
	zero := 0
	_, h, refreshes := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero, AutoApprove: "alice"})
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	k := newBarrier(4)
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
	k.on.Store(true)                           // the barrier is for the four callers below
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
	t.Parallel()
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
	opt.Host = config.Host{Web: "http://127.0.0.1:1"}
	if _, err := Token(a, opt); err == nil || !strings.Contains(err.Error(), "expired") || !strings.Contains(err.Error(), "hedgehug auth login a") {
		t.Errorf("refresh expired = %v", err)
	}
}

func TestDeviceRefreshKeepFails(t *testing.T) {
	t.Parallel()
	zero := 0
	_, h, _ := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero, AutoApprove: "alice"})
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	k := newBarrier(0)
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	opt := Options{Keyring: k, Host: h, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, LockFile: filepath.Join(t.TempDir(), "auth.lock")}
	dt, err := DeviceLogin(h, opt, func(*github.DeviceCode) {})
	if err != nil {
		t.Fatal(err)
	}
	dt.ExpiresAt = now
	if err := SaveDeviceToken(a, dt, k); err != nil {
		t.Fatal(err)
	}
	k.fail = errors.New("keychain locked")
	_, err = Token(a, opt)
	if err == nil || !strings.Contains(err.Error(), "could not keep it") || !strings.Contains(err.Error(), "keychain locked") || !strings.Contains(err.Error(), "hedgehug auth login a") {
		t.Errorf("save failed = %v", err)
	}
}

func TestDeviceNoExpiry(t *testing.T) {
	t.Parallel()
	h := tokenServer(t, map[string]any{"access_token": "ghu_1", "refresh_token": "ghr_1"})
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	dt, err := DeviceLogin(h, Options{Now: func() time.Time { return now }, Sleep: func(time.Duration) {}}, func(*github.DeviceCode) {})
	if err != nil || !dt.ExpiresAt.IsZero() || !dt.RefreshExpiresAt.IsZero() {
		t.Fatalf("DeviceLogin = %+v, %v", dt, err)
	}
	// A token with no end is used years later with no refresh (no lock file, no host).
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	k := newMem()
	_ = SaveDeviceToken(a, dt, k)
	later := now.AddDate(10, 0, 0)
	if tok, err := Token(a, Options{Keyring: k, Now: func() time.Time { return later }}); err != nil || tok != "ghu_1" {
		t.Errorf("Token = %q, %v", tok, err)
	}
}

func TestDeviceRefreshNeedsHost(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	k := newMem()
	_ = SaveDeviceToken(a, DeviceToken{AccessToken: "x", RefreshToken: "r", ExpiresAt: now}, k)
	opt := Options{Keyring: k, Now: func() time.Time { return now }, LockFile: filepath.Join(t.TempDir(), "auth.lock")}
	if _, err := Token(a, opt); err == nil || !strings.Contains(err.Error(), "Host") {
		t.Errorf("no host = %v", err)
	}
}

// 直列: パッケージ変数 lockWait の差し替え
func TestDeviceRefreshLockBusy(t *testing.T) {
	old := lockWait
	lockWait = 5 * time.Millisecond
	t.Cleanup(func() { lockWait = old })
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	k := newMem()
	_ = SaveDeviceToken(a, DeviceToken{AccessToken: "x", RefreshToken: "r", ExpiresAt: now}, k)
	lock := filepath.Join(t.TempDir(), "auth.lock")
	opt := Options{Keyring: k, Host: config.Host{Web: "http://127.0.0.1:1"}, Now: func() time.Time { return now }, LockFile: lock}
	var err error
	if lerr := store.WithFileLock(lock, time.Second, func() error {
		_, err = Token(a, opt)
		return nil
	}); lerr != nil {
		t.Fatal(lerr)
	}
	if err == nil || !strings.Contains(err.Error(), "another hedgehug is refreshing") {
		t.Errorf("busy lock = %v", err)
	}
}

func TestDeviceRefreshLockFails(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	k := newMem()
	_ = SaveDeviceToken(a, DeviceToken{AccessToken: "x", RefreshToken: "r", ExpiresAt: now}, k)
	// The parent of the lock file is a file, so the lock cannot be made.
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	opt := Options{Keyring: k, Host: config.Host{Web: "http://127.0.0.1:1"}, Now: func() time.Time { return now }, LockFile: filepath.Join(parent, "auth.lock")}
	_, err := Token(a, opt)
	if err == nil || strings.Contains(err.Error(), "another hedgehug") {
		t.Errorf("lock failure = %v, want the plain error", err)
	}
}
