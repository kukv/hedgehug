package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// whoami calls GET /user of the fake at base with the token tok.
func whoami(t *testing.T, base, tok string) (string, int) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/user", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var body struct {
		Login string `json:"login"`
	}
	_ = json.NewDecoder(res.Body).Decode(&body)
	return body.Login, res.StatusCode
}

func TestCommands(t *testing.T) {
	t.Parallel()
	s := fakegh.New(fakegh.State{Users: []string{"bob"}, Repos: []*fakegh.Repo{{Owner: "org-a", Name: "x"}}})
	srv := httptest.NewServer(s)
	defer srv.Close()
	for _, args := range [][]string{
		{"perm", "set", "org-a/x", "bob", "read", "--server", srv.URL},
		{"repo", "add", "org-a/y", "--topic", "hedgehug-store", "--server", srv.URL},
		{"token", "add", "t1", "--user", "bob", "--server", srv.URL},
	} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 {
			t.Errorf("fakegh %s: exit %d: %s", strings.Join(args, " "), code, errb.String())
		}
	}
	if r, _ := s.Repo("org-a/x"); r.Perms["bob"] != "read" {
		t.Errorf("perm = %v", r.Perms)
	}
	if r, ok := s.Repo("org-a/y"); !ok || r.Topics[0] != "hedgehug-store" {
		t.Errorf("repo = %+v", r)
	}
	if login, code := whoami(t, srv.URL, "t1"); code != 200 || login != "bob" {
		t.Errorf("token add: /user = %d %q, want 200 bob", code, login)
	}

	// device approve: the user_code of a device code, approved from the
	// command, makes the next poll give a token.
	res, err := http.PostForm(srv.URL+"/login/device/code", url.Values{"client_id": {"Iv23x"}})
	if err != nil {
		t.Fatal(err)
	}
	var dc struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	err = json.NewDecoder(res.Body).Decode(&dc)
	_ = res.Body.Close()
	if err != nil || dc.UserCode == "" {
		t.Fatalf("device code = %+v, %v", dc, err)
	}
	poll := func() (string, string) {
		res, err := http.PostForm(srv.URL+"/login/oauth/access_token", url.Values{
			"client_id": {"Iv23x"}, "device_code": {dc.DeviceCode},
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		var body struct {
			Error       string `json:"error"`
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body.Error, body.AccessToken
	}
	if e, _ := poll(); e != "authorization_pending" {
		t.Errorf("before approval: error = %q", e)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"device", "approve", dc.UserCode, "--user", "bob", "--server", srv.URL}, &out, &errb); code != 0 {
		t.Errorf("device approve: exit %d: %s", code, errb.String())
	}
	e, tok := poll()
	if e != "" || !strings.HasPrefix(tok, "ghu_") {
		t.Fatalf("after approval: error = %q, access_token = %q", e, tok)
	}
	if login, code := whoami(t, srv.URL, tok); code != 200 || login != "bob" {
		t.Errorf("device token before expire: /user = %d %q, want 200 bob", code, login)
	}
	if code := run([]string{"token", "expire", "bob", "--server", srv.URL}, &out, &errb); code != 0 {
		t.Errorf("token expire: exit %d: %s", code, errb.String())
	}
	if _, code := whoami(t, srv.URL, tok); code != 401 {
		t.Errorf("device token after expire: /user = %d, want 401", code)
	}

	errb.Reset()
	if code := run([]string{"device", "approve", "NOPE-0000", "--user", "bob", "--server", srv.URL}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "NOPE-0000") {
		t.Errorf("device approve of a code nobody has: exit %d: %s", code, errb.String())
	}
	if code := run([]string{"device", "approve", dc.UserCode, "--server", srv.URL}, &out, &errb); code != 2 {
		t.Errorf("device approve without --user: exit %d", code)
	}
	errb.Reset()
	if code := run([]string{"perm", "set", "org-a/x"}, &bytes.Buffer{}, &errb); code != 2 {
		t.Errorf("too few args: exit %d", code)
	}
	if code := run([]string{"nope"}, &bytes.Buffer{}, &errb); code != 2 {
		t.Errorf("unknown command: exit %d", code)
	}
}

// Each command takes only its own flags: one it does not use is an error.
func TestFlagsArePerCommand(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"serve", "--server", "x"},
		{"perm", "set", "org-a/x", "bob", "read", "--user", "bob"},
		{"repo", "add", "org-a/y", "--user", "bob"},
		{"token", "expire", "bob", "--topic", "t"},
		{"token", "expire", "bob", "--seed", "x"},
	} {
		var errb bytes.Buffer
		if code := run(args, &bytes.Buffer{}, &errb); code != 2 {
			t.Errorf("fakegh %s: exit %d, want 2: %s", strings.Join(args, " "), code, errb.String())
		}
	}
}

func TestServeCommandErrors(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "nope.json")
	var errb bytes.Buffer
	if code := run([]string{"serve", "--seed", missing}, &bytes.Buffer{}, &errb); code != 1 || !strings.Contains(errb.String(), "nope.json") {
		t.Errorf("serve with a missing seed: exit %d: %s", code, errb.String())
	}
	errb.Reset()
	if code := run([]string{"serve", "extra"}, &bytes.Buffer{}, &errb); code != 2 {
		t.Errorf("serve with an extra argument: exit %d: %s", code, errb.String())
	}
}

// serveOn answers the requests of the handler on the listener it is given.
func TestServeOn(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	st := fakegh.State{Users: []string{"bob"}, Tokens: map[string]string{"t1": "bob"}}
	errc := make(chan error, 1)
	go func() { errc <- serveOn(ln, fakegh.New(st)) }()
	base := "http://" + ln.Addr().String()
	if login, code := whoami(t, base, "t1"); code != 200 || login != "bob" {
		t.Errorf("GET /user = %d %q, want 200 bob", code, login)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; !errors.Is(err, net.ErrClosed) {
		t.Errorf("serveOn returned %v, want the listener closed", err)
	}
}

func TestRateSet(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(fakegh.New(fakegh.State{}))
	t.Cleanup(srv.Close)
	var out, errs strings.Builder
	if code := run([]string{"rate", "set", "10", "5000", "--reset", "60", "--server", srv.URL}, &out, &errs); code != 0 {
		t.Fatalf("rate set: %d %s", code, errs.String())
	}
	if code := run([]string{"rate", "set", "x", "5000", "--server", srv.URL}, &out, &errs); code != 2 {
		t.Errorf("rate set x: code = %d, want 2", code)
	}
}

func TestPullAndIssueCommands(t *testing.T) {
	t.Parallel()
	s := fakegh.New(fakegh.State{Repos: []*fakegh.Repo{{Owner: "o", Name: "r"}}})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	for _, args := range [][]string{
		{"pr", "open", "o/r", "--branch", "hh/WEB-1-a", "--title", "A", "--server", srv.URL},
		{"pr", "review", "o/r#1", "--request", "--server", srv.URL},
		{"pr", "review", "o/r#1", "--state", "approved", "--server", srv.URL},
		{"pr", "checks", "o/r#1", "success", "--server", srv.URL},
		{"pr", "comment", "o/r#1", "--body", "see WEB-2", "--server", srv.URL},
		{"pr", "merge", "o/r#1", "--server", srv.URL},
		{"issue", "open", "o/r", "--title", "I", "--server", srv.URL},
		{"issue", "edit", "o/r#2", "--title", "J", "--server", srv.URL},
		{"issue", "comment", "add", "o/r#2", "--body", "first", "--author", "bob", "--server", srv.URL},
		{"issue", "comment", "add", "o/r#2", "--body", "second", "--server", srv.URL},
		{"issue", "comment", "edit", "o/r", "1", "--body", "edited", "--server", srv.URL},
		{"issue", "comment", "rm", "o/r", "2", "--server", srv.URL},
		{"issue", "close", "o/r#2", "--reason", "not_planned", "--server", srv.URL},
	} {
		var out, errs strings.Builder
		if code := run(args, &out, &errs); code != 0 {
			t.Fatalf("%v: %d %s", args, code, errs.String())
		}
	}
	r, ok := s.Repo("o/r")
	if !ok || r.Pulls[0].State != "merged" || r.Issues[0].State != "closed" || r.Issues[0].Title != "J" {
		t.Errorf("repo = %+v", r)
	}
	if cs := r.Issues[0].Comments; len(cs) != 1 || cs[0].ID != 1 || cs[0].Author != "bob" || cs[0].Body != "edited" {
		t.Errorf("comments = %+v", cs)
	}
}
