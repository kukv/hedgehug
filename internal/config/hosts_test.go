package config

import "testing"

func TestRewrite(t *testing.T) {
	c := &Config{URLRewrites: []URLRewrite{
		{From: "https://github.com/", To: "/r/"},
		{From: "https://github.com/org-a/", To: "/a/"},
	}}
	for in, want := range map[string]string{
		"https://github.com/kukv/x.git":  "/r/kukv/x.git",
		"https://github.com/org-a/y.git": "/a/y.git", // the longest from wins
		"https://gitlab.com/z.git":       "https://gitlab.com/z.git",
	} {
		if got := c.Rewrite(in); got != want {
			t.Errorf("Rewrite(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"/r/kukv/x.git": "https://github.com/kukv/x.git",
		"/a/y.git":      "https://github.com/org-a/y.git", // the longest to wins
		"/elsewhere/z":  "/elsewhere/z",
	} {
		if got := c.Unrewrite(in); got != want {
			t.Errorf("Unrewrite(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsGitHubHost(t *testing.T) {
	c := &Config{Hosts: []Host{{Host: "ghe.example.com"}}}
	if !c.IsGitHubHost("github.com") || !c.IsGitHubHost("ghe.example.com") || c.IsGitHubHost("gitlab.com") {
		t.Error("IsGitHubHost is wrong")
	}
	if h := c.Host("github.com"); h.API != "https://api.github.com" || h.Web != "https://github.com" {
		t.Errorf("github.com default = %+v", h)
	}
}
