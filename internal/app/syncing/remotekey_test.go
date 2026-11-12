package syncing

import "testing"

func TestRemoteKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com/O/Repo.git", "github.com/o/repo"},
		{"https://github.com/o/repo/", "github.com/o/repo"},
		{"https://x-access-token:abc@github.com/o/repo", "github.com/o/repo"},
		{"git@github.com:o/repo.git", "github.com/o/repo"},
		{"ssh://git@github.com/o/repo", "github.com/o/repo"},
		{" https://github.com/o/repo ", "github.com/o/repo"},
		{"https://ghe.example.com/team/o/repo.git", "ghe.example.com/team/o/repo"},
	}
	for _, c := range cases {
		if got := remoteKey(c.in); got != c.want {
			t.Errorf("remoteKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if remoteKey("https://github.com/o/a") == remoteKey("https://github.com/o/b") {
		t.Error("different repositories have the same key")
	}
}
