package ops

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

// A token goes over https, and over http only to the loopback. Over plain
// http to another host it is not even looked for.
func TestConnPlainHTTP(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	var calls atomic.Int32
	ts.env["HH_TOKEN"] = "tok-1"
	ts.appendConfig("accounts:\n" +
		"  - {name: a, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"  - {name: l, host: localhost, login: bob, token: \"env:HH_TOKEN\"}\n" +
		"  - {name: n, host: github.com, login: carol}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	ts.Getenv = func(k string) string {
		calls.Add(1)
		return ts.env[k]
	}
	conn := ts.Conn(c, config.StoreEntry{Account: "a"})
	au, err := conn.Auth("http://github.com/org/x.git")
	var oe *Error
	if au != nil || !errors.As(err, &oe) || oe.Code != "auth.insecure" || oe.Kind != KindInvalid {
		t.Fatalf("Auth(http) = %v, %v", au, err)
	}
	if !strings.Contains(err.Error(), "http://github.com/org/x.git uses plain http, so the token is not sent; use https") {
		t.Errorf("message = %v", err)
	}
	if calls.Load() != 0 {
		t.Errorf("the token was looked for %d times", calls.Load())
	}
	if au, err := conn.Auth("http://mirror.example.com/x.git"); au != nil || err != nil {
		t.Errorf("other host = %v, %v", au, err)
	}
	if au, err := conn.Auth("https://github.com/org/x.git"); err != nil || au == nil {
		t.Errorf("https = %v, %v", au, err)
	}
	lc := ts.Conn(c, config.StoreEntry{Account: "l"})
	for _, u := range []string{"http://localhost/x.git", "http://LOCALHOST:8080/x.git"} {
		if au, err := lc.Auth(u); err != nil || au == nil {
			t.Errorf("Auth(%q) = %v, %v", u, au, err)
		}
	}
	if au, err := ts.Conn(c, config.StoreEntry{Account: "n"}).Auth("http://github.com/org/x.git"); au != nil || err != nil {
		t.Errorf("token-less = %v, %v", au, err)
	}
}
