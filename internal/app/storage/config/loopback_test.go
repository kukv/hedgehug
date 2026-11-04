package config

import (
	"strings"
	"testing"
)

func TestLoadHostsPlainHTTP(t *testing.T) {
	t.Parallel()
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
