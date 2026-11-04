package github

import "testing"

func TestIsLoopback(t *testing.T) {
	t.Parallel()
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
