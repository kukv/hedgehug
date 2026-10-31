package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/fakegh"
)

func TestCommands(t *testing.T) {
	s := fakegh.New(fakegh.State{Users: []string{"bob"}, Repos: []*fakegh.Repo{{Owner: "org-a", Name: "x"}}})
	srv := httptest.NewServer(s)
	defer srv.Close()
	for _, args := range [][]string{
		{"perm", "set", "org-a/x", "bob", "read", "--server", srv.URL},
		{"repo", "add", "org-a/y", "--topic", "hedgehug-store", "--server", srv.URL},
		{"token", "add", "t1", "--user", "bob", "--server", srv.URL},
		{"token", "expire", "bob", "--server", srv.URL},
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
	poll := func() string {
		res, err := http.PostForm(srv.URL+"/login/oauth/access_token", url.Values{
			"client_id": {"Iv23x"}, "device_code": {dc.DeviceCode},
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		b, _ := io.ReadAll(res.Body)
		return string(b)
	}
	if got := poll(); !strings.Contains(got, "authorization_pending") {
		t.Errorf("before approval = %s", got)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"device", "approve", dc.UserCode, "--user", "bob", "--server", srv.URL}, &out, &errb); code != 0 {
		t.Errorf("device approve: exit %d: %s", code, errb.String())
	}
	if got := poll(); !strings.Contains(got, "ghu_") {
		t.Errorf("after approval = %s", got)
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
