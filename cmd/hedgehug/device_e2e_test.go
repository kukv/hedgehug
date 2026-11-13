//go:build e2e

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestE2EDeviceAnswersByCode(t *testing.T) {
	gh := &apptest.GitHub{}
	d := newE2EDevice(gh)
	a, _ := d.StartDevice(t.Context(), "c")
	b, _ := d.StartDevice(t.Context(), "c")
	if a.UserCode != "0000-0001" || b.UserCode != "0000-0002" || a.DeviceCode == b.DeviceCode {
		t.Fatalf("codes: %+v %+v", a, b)
	}
	if g, _ := d.PollDevice(t.Context(), "c", a.DeviceCode); g.Status != port.DevicePending {
		t.Errorf("before an answer: %v", g.Status)
	}
	if err := d.answer(a.UserCode, "grant", "taro"); err != nil {
		t.Fatal(err)
	}
	g, _ := d.PollDevice(t.Context(), "c", a.DeviceCode)
	if login, err := gh.Viewer(t.Context(), g.Token.Access); g.Status != port.DeviceGranted || err != nil || login != "taro" {
		t.Errorf("grant: %+v %q %v", g, login, err)
	}
	for result, want := range map[string]port.DeviceStatus{"deny": port.DeviceDenied, "expire": port.DeviceExpired} {
		if err := d.answer(b.UserCode, result, ""); err != nil {
			t.Fatal(err)
		}
		if g, _ := d.PollDevice(t.Context(), "c", b.DeviceCode); g.Status != want {
			t.Errorf("%s: %v", result, g.Status)
		}
	}
	if d.answer("9999-9999", "grant", "x") == nil || d.answer(b.UserCode, "maybe", "") == nil {
		t.Error("an unknown code or result was accepted")
	}
}

func TestE2EDeviceHandler(t *testing.T) {
	d := newE2EDevice(&apptest.GitHub{})
	c, _ := d.StartDevice(t.Context(), "c")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := d.handler("tok", next)
	do := func(method, target, auth string) int {
		r := httptest.NewRequest(method, target, nil)
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	deny := "/e2e/device?code=" + c.UserCode + "&result=deny"
	for _, tc := range []struct {
		method, target, auth string
		want                 int
	}{
		{http.MethodGet, "/api/accounts", "", http.StatusTeapot},
		{http.MethodPost, deny, "Bearer nope", http.StatusUnauthorized},
		{http.MethodGet, deny, "Bearer tok", http.StatusMethodNotAllowed},
		{http.MethodPost, "/e2e/device?code=0000-9999&result=deny", "Bearer tok", http.StatusBadRequest},
		{http.MethodPost, deny, "Bearer tok", http.StatusNoContent},
	} {
		if got := do(tc.method, tc.target, tc.auth); got != tc.want {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.target, got, tc.want)
		}
	}
}

func TestE2ELoginsHaveTokens(t *testing.T) {
	tokens, users := e2eLogins()
	if users[tokens["gh-desktop-1-b"]] != "gh-desktop-1-b" || users[tokens["taro"]] != "taro" || users[tokens["e2e-bot"]] != "e2e-bot" || len(tokens) != 14 {
		t.Errorf("tokens %v users %v", tokens, users)
	}
}
