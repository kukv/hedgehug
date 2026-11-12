package account

import (
	"testing"
	"time"
)

func TestDeviceTokenFresh(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		tok  DeviceToken
		want bool
	}{
		{"no end", DeviceToken{Access: "a"}, true},
		{"ends in 6 minutes", DeviceToken{Access: "a", ExpiresAt: now.Add(6 * time.Minute)}, true},
		{"ends in 5 minutes", DeviceToken{Access: "a", ExpiresAt: now.Add(5 * time.Minute)}, false},
		{"ended", DeviceToken{Access: "a", ExpiresAt: now.Add(-time.Second)}, false},
	}
	for _, c := range cases {
		if got := c.tok.Fresh(now); got != c.want {
			t.Errorf("%s: Fresh = %v", c.name, got)
		}
	}
}

func TestDeviceTokenRefreshExpired(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	if (DeviceToken{}).RefreshExpired(now) {
		t.Error("no end of the refresh token is not expired")
	}
	if !(DeviceToken{RefreshExpiresAt: now}).RefreshExpired(now) {
		t.Error("the refresh token that ends now is expired")
	}
	if (DeviceToken{RefreshExpiresAt: now.Add(time.Hour)}).RefreshExpired(now) {
		t.Error("the refresh token that ends later is not expired")
	}
}
