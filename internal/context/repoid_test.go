package context

import "testing"

func TestSplit(t *testing.T) {
	for in, want := range map[string][3]string{
		"https://github.com/Org-A/HH-Shared.git": {"github.com", "Org-A", "HH-Shared"},
		"https://GitHub.com/org-a/api/":          {"github.com", "org-a", "api"},
		"http://127.0.0.1:7777/x/y.git":          {"127.0.0.1", "x", "y"},
		"https://ghe.example.com:8443/x/y":       {"ghe.example.com", "x", "y"},
		"git@github.com:org-a/api.git":           {"github.com", "org-a", "api"},
		"git@github.com:Org-A/API":               {"github.com", "Org-A", "API"},
		"ssh://git@github.com:22/org-a/api.git":  {"github.com", "org-a", "api"},
		"ssh://ghe.example.com/org-a/api":        {"ghe.example.com", "org-a", "api"},
		"git://github.com/org-a/api":             {"github.com", "org-a", "api"},
		"https://u:pw@github.com/org-a/api.git":  {"github.com", "org-a", "api"},
		"git@GitHub.com:org-a/api.git/":          {"github.com", "org-a", "api"},
		"https://github.com/org-a/api.git/":      {"github.com", "org-a", "api"},
	} {
		h, o, n, ok := Split(in)
		if !ok || [3]string{h, o, n} != want {
			t.Errorf("Split(%q) = %q %q %q %v, want %v", in, h, o, n, ok, want)
		}
	}
	for _, in := range []string{"/tmp/x.git", "file:///tmp/x", "https://github.com/only", "https://github.com/a/b/c", `C:\x\y`, "", "https://github.com/a/.git", "https://github.com//b", "https:///a/b", "git@github.com:only"} {
		if h, o, n, ok := Split(in); ok {
			t.Errorf("Split(%q) = %q %q %q, want none", in, h, o, n)
		}
	}
}

func TestRepoID(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/Org-A/API.git":      "github.com/org-a/api",
		"http://github.com/org-a/api/":          "github.com/org-a/api",
		"ssh://git@github.com:22/org-a/api.git": "github.com/org-a/api",
		"git@github.com:org-a/api.git":          "github.com/org-a/api",
		"git://github.com/org-a/api":            "github.com/org-a/api",
		"https://ghe.example.com:8443/x/y":      "ghe.example.com/x/y",
	} {
		if got, ok := RepoID(in); !ok || got != want {
			t.Errorf("RepoID(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"/tmp/x.git", "file:///tmp/x", "https://github.com/only", `C:\x\y`} {
		if got, ok := RepoID(in); ok {
			t.Errorf("RepoID(%q) = %q, want none", in, got)
		}
	}
}
