// Package context decides where a new task goes when the command does not
// say, and reads remote URLs. Phase 3a has only the remote URLs.
package context

import (
	"net/url"
	"regexp"
	"strings"
)

// scpLike is user@host:owner/repo, the short form of ssh.
var scpLike = regexp.MustCompile(`^[^@/]+@([^:/]+):(.+)$`)

// Split reads a remote URL: https, http, ssh, git and user@host:path. The
// host is in lower case and has no port; the owner and the repo are as
// written, and the repo has no .git or trailing slash. A local path, or a URL
// that does not end in an owner and a repo, is not ok.
//
// This is the one place that reads a remote URL: store init (is this a
// GitHub host, and which repository to make), the read-only check and the
// member list use the owner and the repo from here, and RepoID lower-cases
// it.
func Split(u string) (host, owner, repo string, ok bool) {
	var p string
	if m := scpLike.FindStringSubmatch(u); m != nil && !strings.Contains(u, "://") {
		host, p = m[1], m[2]
	} else {
		x, err := url.Parse(u)
		if err != nil {
			return "", "", "", false
		}
		switch x.Scheme {
		case "https", "http", "ssh", "git":
		default:
			return "", "", "", false
		}
		host, p = x.Hostname(), x.Path
	}
	parts := strings.Split(strings.Trim(strings.TrimSuffix(strings.Trim(p, "/"), ".git"), "/"), "/")
	if host == "" || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	return strings.ToLower(host), parts[0], parts[1], true
}

// RepoID turns a remote URL into <host>/<owner>/<repo> in lower case, without
// .git, a trailing slash or a port. A local path, or a URL without an owner
// and a repo, has none.
func RepoID(u string) (string, bool) {
	host, owner, repo, ok := Split(u)
	if !ok {
		return "", false
	}
	return strings.ToLower(host + "/" + owner + "/" + repo), true
}
