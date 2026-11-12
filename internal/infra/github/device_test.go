package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func deviceServer(t *testing.T, answer func(form map[string]string) any) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Accept") != "application/json" {
			t.Errorf("%s %s accept %q", r.Method, r.URL.Path, r.Header.Get("Accept"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		form := map[string]string{"path": r.URL.Path}
		for k := range r.PostForm {
			form[k] = r.PostForm.Get(k)
		}
		_ = json.NewEncoder(w).Encode(answer(form))
	}))
	t.Cleanup(srv.Close)
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	return &Client{WebURL: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return now }}
}

func TestStartDevice(t *testing.T) {
	c := deviceServer(t, func(form map[string]string) any {
		if form["path"] != "/login/device/code" || form["client_id"] != "Iv23abc" {
			t.Errorf("form %v", form)
		}
		return map[string]any{"device_code": "dc", "user_code": "WDJB-MJHT", "verification_uri": "https://github.com/login/device", "expires_in": 900, "interval": 5}
	})
	got, err := c.StartDevice(context.Background(), "Iv23abc")
	want := port.DeviceCode{DeviceCode: "dc", UserCode: "WDJB-MJHT", VerificationURI: "https://github.com/login/device", ExpiresIn: 15 * time.Minute, Interval: 5 * time.Second}
	if err != nil || got != want {
		t.Errorf("StartDevice = %+v, %v", got, err)
	}
}

func TestStartDeviceRefused(t *testing.T) {
	c := deviceServer(t, func(map[string]string) any { return map[string]any{"error": "unauthorized_client"} })
	if _, err := c.StartDevice(context.Background(), "bad"); !ghcall.Is(err, ghcall.Unreachable) {
		t.Errorf("StartDevice = %v", err)
	}
}

func TestPollDevice(t *testing.T) {
	cases := map[string]port.DeviceStatus{
		"authorization_pending": port.DevicePending,
		"slow_down":             port.DeviceSlowDown,
		"expired_token":         port.DeviceExpired,
		"access_denied":         port.DeviceDenied,
		"incorrect_device_code": port.DeviceExpired,
	}
	for code, want := range cases {
		c := deviceServer(t, func(form map[string]string) any {
			if form["path"] != "/login/oauth/access_token" || form["device_code"] != "dc" ||
				form["grant_type"] != "urn:ietf:params:oauth:grant-type:device_code" {
				t.Errorf("form %v", form)
			}
			return map[string]any{"error": code}
		})
		if g, err := c.PollDevice(context.Background(), "Iv23abc", "dc"); err != nil || g.Status != want {
			t.Errorf("%s: %+v, %v", code, g, err)
		}
	}
}

func TestPollDeviceGranted(t *testing.T) {
	c := deviceServer(t, func(map[string]string) any {
		return map[string]any{"access_token": "tok-a", "refresh_token": "tok-r", "expires_in": 28800, "refresh_token_expires_in": 15897600}
	})
	g, err := c.PollDevice(context.Background(), "Iv23abc", "dc")
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	if err != nil || g.Status != port.DeviceGranted || g.Token.Access != "tok-a" || g.Token.Refresh != "tok-r" ||
		!g.Token.ExpiresAt.Equal(now.Add(8*time.Hour)) || !g.Token.RefreshExpiresAt.Equal(now.Add(15897600*time.Second)) {
		t.Errorf("PollDevice = %+v, %v", g, err)
	}
}

func TestPollDeviceNoEnd(t *testing.T) {
	c := deviceServer(t, func(map[string]string) any { return map[string]any{"access_token": "tok-a", "expires_in": 0} })
	g, err := c.PollDevice(context.Background(), "Iv23abc", "dc")
	if err != nil || !g.Token.ExpiresAt.IsZero() || !g.Token.RefreshExpiresAt.IsZero() {
		t.Errorf("PollDevice = %+v, %v", g, err)
	}
}

func TestRefreshDevice(t *testing.T) {
	c := deviceServer(t, func(form map[string]string) any {
		if form["grant_type"] != "refresh_token" || form["refresh_token"] != "tok-r" || form["client_id"] != "Iv23abc" {
			t.Errorf("form %v", form)
		}
		return map[string]any{"access_token": "tok-b", "refresh_token": "tok-r2", "expires_in": 28800}
	})
	got, err := c.RefreshDevice(context.Background(), "Iv23abc", "tok-r")
	if err != nil || got.Access != "tok-b" || got.Refresh != "tok-r2" {
		t.Errorf("RefreshDevice = %+v, %v", got, err)
	}
	bad := deviceServer(t, func(map[string]string) any { return map[string]any{"error": "bad_refresh_token"} })
	if _, err := bad.RefreshDevice(context.Background(), "Iv23abc", "tok-r"); !ghcall.Is(err, ghcall.TokenRejected) {
		t.Errorf("rejected: %v", err)
	}
}

func TestDeviceUnreachable(t *testing.T) {
	c := &Client{WebURL: "http://127.0.0.1:1", HTTP: http.DefaultClient, Now: time.Now}
	if _, err := c.PollDevice(context.Background(), "Iv23abc", "dc"); !ghcall.Is(err, ghcall.Unreachable) {
		t.Errorf("PollDevice = %v", err)
	}
}
