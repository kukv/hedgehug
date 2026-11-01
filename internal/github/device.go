package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DeviceCode is the start of the device flow.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is the answer of the token endpoint. Error is set, with no
// tokens, while the user has not approved or when it failed.
type TokenResponse struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
}

func postForm(hc *http.Client, u string, form url.Values, out any) error {
	req, err := http.NewRequest("POST", u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := defaultHTTP(hc).Do(req) //nolint:gosec // G704: web is the web base of a host in config.yaml
	if err != nil {
		return requestErr("POST", u, err)
	}
	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("POST %s: reading the answer: %w", u, err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s: %d %s", u, res.StatusCode, strings.TrimSpace(string(data)))
	}
	return json.Unmarshal(data, out)
}

// RequestDeviceCode starts the device flow of the App clientID at web.
func RequestDeviceCode(hc *http.Client, web, clientID string) (*DeviceCode, error) {
	var raw struct {
		DeviceCode
		Error string `json:"error"`
	}
	if err := postForm(hc, web+"/login/device/code", url.Values{"client_id": {clientID}}, &raw); err != nil {
		return nil, err
	}
	if raw.Error != "" || raw.DeviceCode.DeviceCode == "" {
		return nil, fmt.Errorf("the device flow did not start: %s", raw.Error)
	}
	return &raw.DeviceCode, nil
}

// PollToken asks once whether the device code was approved.
func PollToken(hc *http.Client, web, clientID, deviceCode string) (*TokenResponse, error) {
	var tr TokenResponse
	err := postForm(hc, web+"/login/oauth/access_token", url.Values{
		"client_id": {clientID}, "device_code": {deviceCode},
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
	}, &tr)
	return &tr, err
}

// RefreshToken trades a refresh token for a new pair of tokens.
func RefreshToken(hc *http.Client, web, clientID, refresh string) (*TokenResponse, error) {
	var tr TokenResponse
	err := postForm(hc, web+"/login/oauth/access_token", url.Values{
		"client_id": {clientID}, "refresh_token": {refresh}, "grant_type": {"refresh_token"},
	}, &tr)
	return &tr, err
}
