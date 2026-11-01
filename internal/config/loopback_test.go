package config

import (
	"strings"
	"testing"
)

func TestIsLoopback(t *testing.T) {
	for h, want := range map[string]bool{
		"127.0.0.1": true, "127.1.2.3": true, "::1": true, "[::1]": true,
		"localhost": true, "LocalHost": true,
		"github.com": false, "10.0.0.1": false, "127.0.0.1.evil.test": false,
		"localhost.evil.test": false, "": false, "::2": false,
		"0x7f.0.0.1": false, "::ffff:127.0.0.1": true, "::1%lo": false, "localhost.": false,
	} {
		if got := IsLoopback(h); got != want {
			t.Errorf("IsLoopback(%q) = %v, want %v", h, got, want)
		}
	}
}

func TestLoadHostsPlainHTTP(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{"api", "hosts:\n  - {host: ghe.example.com, api: \"http://ghe.example.com/api/v3\"}\n", ":2: hosts: api \"http://ghe.example.com/api/v3\" uses plain http"},
		{"web", "hosts:\n  - {host: ghe.example.com, web: \"HTTP://ghe.example.com\"}\n", ":2: hosts: web \"HTTP://ghe.example.com\" uses plain http"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPaths(t)
			writeConfig(t, p, tc.yaml)
			_, err := Load(p)
			if err == nil || !strings.Contains(err.Error(), p.ConfigFile+tc.want) {
				t.Errorf("err = %v, want one with %q", err, p.ConfigFile+tc.want)
			}
		})
	}
	for _, y := range []string{
		"hosts:\n  - {host: l, api: \"http://127.0.0.1:7777\", web: \"http://localhost:7778\"}\n",
		"hosts:\n  - {host: l, api: \"http://[::1]:7777\"}\n",
		"hosts:\n  - {host: l, api: \"https://ghe.example.com/api/v3\"}\n",
	} {
		p := testPaths(t)
		writeConfig(t, p, y)
		if _, err := Load(p); err != nil {
			t.Errorf("%q: %v", y, err)
		}
	}
}
