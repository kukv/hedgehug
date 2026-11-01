package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

// The userinfo of a URL is not echoed by the error.
func TestConnPlainHTTPHidesUserinfo(t *testing.T) {
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "tok-1"
	ts.appendConfig("accounts:\n  - {name: a, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Conn(c, config.StoreEntry{Account: "a"}).Auth("http://user:pw@github.com/org/x.git")
	var oe *Error
	if !errors.As(err, &oe) || oe.Code != "auth.insecure" {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "pw") || strings.Contains(err.Error(), "user") {
		t.Errorf("message leaks userinfo: %v", err)
	}
	if got := oe.Args["url"]; got != "http://github.com/org/x.git" {
		t.Errorf("Args[url] = %v", got)
	}
}

// Loopback over http gets the token for both IPv4 and IPv6 literals.
func TestConnLoopbackLiterals(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		ts := newTestSession(t)
		ts.env["HH_TOKEN"] = "tok-1"
		ts.appendConfig("accounts:\n  - {name: a, host: \"" + host + "\", login: alice, token: \"env:HH_TOKEN\"}\n")
		c, _, err := ts.Config()
		if err != nil {
			t.Fatal(err)
		}
		u := "http://127.0.0.1:7777/x.git"
		if host == "::1" {
			u = "http://[::1]:7777/x.git"
		}
		if au, err := ts.Conn(c, config.StoreEntry{Account: "a"}).Auth(u); err != nil || au == nil {
			t.Errorf("Auth(%q) = %v, %v", u, au, err)
		}
	}
}

// A url_rewrites rule is judged after the rewrite: an https URL rewritten to
// a plain http one on another host is refused.
func TestConnRewriteToPlainHTTP(t *testing.T) {
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "tok-1"
	ts.appendConfig("accounts:\n  - {name: a, host: mirror.example.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"url_rewrites:\n  - {from: \"https://github.com/\", to: \"http://mirror.example.com/\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	conn := ts.Conn(c, config.StoreEntry{Account: "a"})
	u := conn.Rewrite("https://github.com/org/x.git")
	_, err = conn.Auth(u)
	var oe *Error
	if !errors.As(err, &oe) || oe.Code != "auth.insecure" {
		t.Errorf("Auth(%q) = %v", u, err)
	}
}
