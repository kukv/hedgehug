package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/remote/auth"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

type deviceStarted struct {
	ID              string    `json:"id"`
	UserCode        string    `json:"user_code"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresAt       time.Time `json:"expires_at"`
}

func TestDeviceFlowDone(t *testing.T) {
	t.Parallel()
	zero := 0
	ts, k, _ := newAccountsServer(t, accountsSetup{gh: fakegh.State{DeviceInterval: &zero, AutoApprove: "bob"}})
	es := ts.events()

	var st deviceStarted
	r := ts.call("POST", "/api/auth/device", map[string]any{}, 201, &st)
	if st.ID == "" || st.UserCode == "" || !strings.HasPrefix(st.VerificationURI, "http") || !st.ExpiresAt.After(time.Now()) {
		t.Errorf("started = %+v", st)
	}
	payload := es.next(t, "auth.device")
	var ev authDeviceEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.ID != st.ID || ev.State != "done" || ev.Account != "bob" || ev.Code != "" || ev.Message != "" {
		t.Errorf("event = %+v", ev)
	}

	// the account is in config.yaml with the device flow, and the tokens are in the keychain.
	c, _, err := ts.session().Config()
	if err != nil {
		t.Fatal(err)
	}
	acct, ok := c.Account("bob")
	if !ok || acct.Token.Kind != "device" || acct.ID() != "github.com/bob" {
		t.Fatalf("account = %+v, %v", acct, ok)
	}
	dt, err := auth.LoadDeviceToken(acct, k)
	if err != nil || dt.AccessToken == "" || dt.RefreshToken == "" {
		t.Fatalf("keychain = %+v, %v", dt, err)
	}
	// 細部 12: the tokens are in no response, no event and no line of the log.
	for _, s := range []string{dt.AccessToken, dt.RefreshToken} {
		noSecret(t, s, r.Body, payload, ts.logText())
	}
	var list []accountRow
	g := ts.call("GET", "/api/accounts", nil, 200, &list)
	noSecret(t, dt.AccessToken, g.Body)
	if len(list) != 2 || list[1].Name != "bob" || list[1].State != "ok" || list[1].Token != "device" {
		t.Errorf("accounts = %+v", list)
	}
}

// Only one flow runs at a time; the second is refused until the first ends,
// and a cancel ends it, with an event.
func TestDeviceFlowOneAtATimeAndCancel(t *testing.T) {
	t.Parallel()
	five := 5
	ts, _, _ := newAccountsServer(t, accountsSetup{gh: fakegh.State{DeviceInterval: &five}})
	es := ts.events()

	var first deviceStarted
	ts.call("POST", "/api/auth/device", map[string]any{}, 201, &first)
	if p := ts.problem("POST", "/api/auth/device", map[string]any{}, 409); p.Type != "auth.device_running" {
		t.Errorf("second: %+v", p)
	}
	if p := ts.problem("DELETE", "/api/auth/device/not-"+first.ID, nil, 404); p.Type != "auth.device_not_found" {
		t.Errorf("other id: %+v", p)
	}
	ts.call("DELETE", "/api/auth/device/"+first.ID, nil, 204, nil)
	var ev authDeviceEvent
	if err := json.Unmarshal(es.next(t, "auth.device"), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.ID != first.ID || ev.State != "error" || ev.Code != "auth.canceled" || ev.Message == "" || ev.Account != "" {
		t.Errorf("event = %+v", ev)
	}
	if c, _, err := ts.session().Config(); err != nil || len(c.Accounts) != 1 {
		t.Errorf("accounts after the cancel = %+v, %v", c.Accounts, err)
	}
	// the cancel is answered after the flow ended: the next one starts.
	var second deviceStarted
	ts.call("POST", "/api/auth/device", map[string]any{}, 201, &second)
	if second.ID == first.ID {
		t.Errorf("the id %s was used twice", second.ID)
	}
	ts.call("DELETE", "/api/auth/device/"+second.ID, nil, 204, nil)
	if p := ts.problem("DELETE", "/api/auth/device/"+second.ID, nil, 404); p.Type != "auth.device_not_found" {
		t.Errorf("cancel twice: %+v", p)
	}
}

// A person who approves it by hand (the code on the page of GitHub) ends it
// too; this is the way the E2E of 4f-2 goes.
func TestDeviceFlowApprovedByHand(t *testing.T) {
	t.Parallel()
	one := 1
	ts, _, fg := newAccountsServer(t, accountsSetup{gh: fakegh.State{DeviceInterval: &one}})
	es := ts.events()
	var st deviceStarted
	ts.call("POST", "/api/auth/device", map[string]any{"name": "carol-gh"}, 201, &st)
	if err := fg.ApproveDevice(st.UserCode, "carol"); err != nil {
		t.Fatal(err)
	}
	var ev authDeviceEvent
	if err := json.Unmarshal(es.next(t, "auth.device"), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.State != "done" || ev.Account != "carol-gh" {
		t.Errorf("event = %+v", ev)
	}
}

// What is wrong before the code is shown is the answer of the start.
func TestDeviceFlowStartErrors(t *testing.T) {
	t.Parallel()
	t.Run("no client id", func(t *testing.T) {
		ts, _, _ := newAccountsServer(t, accountsSetup{noClientID: true})
		for range 2 { // the failed start leaves nothing running: not 409 the second time
			p := ts.problem("POST", "/api/auth/device", map[string]any{}, 422)
			if p.Type != "auth.client_id" || p.Args["host"] != "github.com" {
				t.Errorf("problem = %+v", p)
			}
		}
	})
	t.Run("account of gh", func(t *testing.T) {
		ts, _, _ := newAccountsServer(t, accountsSetup{})
		ts.call("POST", "/api/accounts", map[string]any{"name": "g", "login": "gee", "token": "gh"}, 201, nil)
		p := ts.problem("POST", "/api/auth/device", map[string]any{"name": "g"}, 422)
		if p.Type != "auth.no_device_flow" || p.Args["uses"] != "gh" {
			t.Errorf("problem = %+v", p)
		}
	})
	t.Run("account of the keychain", func(t *testing.T) {
		ts, _, _ := newAccountsServer(t, accountsSetup{gh: fakegh.State{Tokens: map[string]string{"t-alice": "alice"}}})
		ts.call("PUT", "/api/accounts/alice/token", map[string]any{"token": "t-alice"}, 200, nil)
		if p := ts.problem("POST", "/api/auth/device", map[string]any{"name": "alice"}, 400); p.Type != "auth.need_token" {
			t.Errorf("problem = %+v", p)
		}
	})
	t.Run("host of another account", func(t *testing.T) {
		ts, _, _ := newAccountsServer(t, accountsSetup{})
		if p := ts.problem("POST", "/api/auth/device", map[string]any{"name": "alice", "host": "ghe.example.com"}, 400); p.Type != "auth.host" {
			t.Errorf("problem = %+v", p)
		}
	})
}

// Shutdown ends the flow that waits for the user, and returns once it has.
func TestDeviceFlowStopsWithShutdown(t *testing.T) {
	t.Parallel()
	five := 5
	ts, _, _ := newAccountsServer(t, accountsSetup{gh: fakegh.State{DeviceInterval: &five}})
	var st deviceStarted
	ts.call("POST", "/api/auth/device", map[string]any{}, 201, &st)
	done := make(chan struct{})
	go func() {
		_ = ts.srv.Shutdown(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return: the device flow is still running")
	}
	if ts.srv.device.get(st.ID) != nil {
		t.Error("the device flow is registered after Shutdown")
	}
}
