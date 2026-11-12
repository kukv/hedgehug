package github_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestSignInAPI(t *testing.T) {
	gh := &apptest.GitHub{Users: map[string]string{"tok-saburo": "saburo"}}
	svc := newServices(gh)
	device := svc.SignIn.Device.(*apptest.DeviceFlow)
	device.Polls = []port.DeviceGrant{{Status: port.DevicePending}, {Status: port.DeviceGranted, Token: account.DeviceToken{Access: "tok-saburo"}}}
	_, api := humatest.New(t)
	ghapi.Register(api, svc)

	res := api.Post("/api/sign-ins", map[string]string{"name": "副業", "login": "saburo"})
	if res.Code != http.StatusCreated || strings.Contains(res.Body.String(), "device-1") {
		t.Fatalf("POST: %d %s (the device code must not be shown)", res.Code, res.Body)
	}
	var code struct {
		ID        string `json:"id"`
		UserCode  string `json:"user_code"`
		ExpiresIn int    `json:"expires_in"`
		Interval  int    `json:"interval"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &code); err != nil {
		t.Fatal(err)
	}
	if code.UserCode != "WDJB-MJHT" || code.ExpiresIn != 900 || code.Interval != 5 {
		t.Errorf("code %+v", code)
	}
	if res = api.Get("/api/sign-ins/" + code.ID); !strings.Contains(res.Body.String(), `"state":"waiting"`) {
		t.Errorf("1st GET: %s", res.Body)
	}
	res = api.Get("/api/sign-ins/" + code.ID)
	if !strings.Contains(res.Body.String(), `"state":"done"`) || !strings.Contains(res.Body.String(), `"login":"saburo"`) ||
		strings.Contains(res.Body.String(), "tok-saburo") {
		t.Errorf("2nd GET: %s", res.Body)
	}
	if res = api.Get("/api/sign-ins/" + code.ID); res.Code != http.StatusNotFound {
		t.Errorf("after done: %d", res.Code)
	}
	res = api.Post("/api/sign-ins", map[string]string{"name": "x", "login": "x"})
	var other struct{ ID string }
	if err := json.Unmarshal(res.Body.Bytes(), &other); err != nil {
		t.Fatal(err)
	}
	if res = api.Delete("/api/sign-ins/" + other.ID); res.Code != http.StatusNoContent {
		t.Errorf("DELETE: %d", res.Code)
	}
}
