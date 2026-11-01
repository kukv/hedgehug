package config

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Host is where a GitHub host serves its API and its web pages, and the
// client ID of the GitHub App for the device flow there.
type Host struct {
	Host     string
	API      string // the REST API base, without a trailing slash
	Web      string // the web base, for the device flow
	ClientID string // empty: the built-in one of internal/auth, on github.com
}

// URLRewrite points the URLs that start with From at To, right before a
// fetch, push or clone.
type URLRewrite struct{ From, To string }

// RepoAccount picks the account for the repositories whose repo ID matches
// Match (a path.Match pattern; "*" matches every ID). Phase 5 uses it.
type RepoAccount struct{ Match, Account string }

// SameHost reports whether a and b name the same host. Hosts are compared
// without case and kept as they are written.
func SameHost(a, b string) bool { return strings.EqualFold(a, b) }

// SameAccount reports whether a and b are the same account ID
// (<host>/<login>), compared without case.
func SameAccount(a, b string) bool { return strings.EqualFold(a, b) }

// text is the trimmed value of a scalar node. ok is false for a node that
// is not a scalar; a missing node and a null are the empty string.
func text(n *yaml.Node) (s string, ok bool) {
	if n == nil || n.Tag == "!!null" {
		return "", true
	}
	if n.Kind != yaml.ScalarNode {
		return "", false
	}
	return strings.TrimSpace(n.Value), true
}

// Host returns the host name with its defaults filled in: the API at
// https://api.github.com for github.com and https://<host>/api/v3 elsewhere,
// the web at https://<host>.
func (c *Config) Host(name string) Host {
	h := Host{Host: name}
	for _, x := range c.Hosts {
		if SameHost(x.Host, name) {
			h = x
		}
	}
	if h.API == "" {
		h.API = "https://" + name + "/api/v3"
		if SameHost(name, "github.com") {
			h.API = "https://api.github.com"
		}
	}
	if h.Web == "" {
		h.Web = "https://" + name
	}
	h.API = strings.TrimSuffix(h.API, "/")
	h.Web = strings.TrimSuffix(h.Web, "/")
	return h
}

// IsGitHubHost reports whether name is github.com or a host of hosts.
func (c *Config) IsGitHubHost(name string) bool {
	if SameHost(name, "github.com") {
		return true
	}
	for _, x := range c.Hosts {
		if SameHost(x.Host, name) {
			return true
		}
	}
	return false
}

// Rewrite applies the url_rewrites rule with the longest From that u
// starts with; u is returned as it is when none does.
func (c *Config) Rewrite(u string) string {
	best := -1
	for i, r := range c.URLRewrites {
		if strings.HasPrefix(u, r.From) && (best < 0 || len(r.From) > len(c.URLRewrites[best].From)) {
			best = i
		}
	}
	if best < 0 {
		return u
	}
	return c.URLRewrites[best].To + strings.TrimPrefix(u, c.URLRewrites[best].From)
}

// Unrewrite turns a rewritten URL or path back: the rule with the longest
// To that p starts with gives its From. p is returned as it is when none does.
func (c *Config) Unrewrite(p string) string {
	best := -1
	for i, r := range c.URLRewrites {
		if strings.HasPrefix(p, r.To) && (best < 0 || len(r.To) > len(c.URLRewrites[best].To)) {
			best = i
		}
	}
	if best < 0 {
		return p
	}
	return c.URLRewrites[best].From + strings.TrimPrefix(p, c.URLRewrites[best].To)
}

// webURL reports whether s is an http or https URL with a host.
func webURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// IsLoopback reports whether host (a name or an IP, with or without the
// brackets of an IPv6 literal) is the local machine: 127.0.0.0/8, ::1 or
// localhost. A token is sent over plain http only there.
func IsLoopback(host string) bool {
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// PlainHTTP reports whether s is an http URL (not https) whose host is not
// loopback.
func PlainHTTP(s string) bool {
	u, err := url.Parse(s)
	return err == nil && strings.EqualFold(u.Scheme, "http") && !IsLoopback(u.Hostname())
}

// loadHosts reads hosts: a host each, with an optional api, web and client_id.
func (c *Config) loadHosts(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "hosts must be a list")
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return c.errAt(it, "hosts: want a mapping with host")
		}
		var h Host
		for _, f := range []struct {
			key string
			dst *string
		}{{"host", &h.Host}, {"api", &h.API}, {"web", &h.Web}, {"client_id", &h.ClientID}} {
			v, ok := text(valueOf(it, f.key))
			if !ok {
				return c.errAt(it, "hosts: "+f.key+" must be a string")
			}
			*f.dst = v
		}
		if h.Host == "" {
			return c.errAt(it, "hosts: host is missing")
		}
		for _, f := range []struct{ key, v string }{{"api", h.API}, {"web", h.Web}} {
			if f.v != "" && !webURL(f.v) {
				return c.errAt(it, fmt.Sprintf("hosts: %s %q: want an http or https URL", f.key, f.v))
			}
			if PlainHTTP(f.v) {
				return c.errAt(it, fmt.Sprintf("hosts: %s %q uses plain http, so the token would be sent in the clear; use https", f.key, f.v))
			}
		}
		if slices.ContainsFunc(c.Hosts, func(x Host) bool { return SameHost(x.Host, h.Host) }) {
			return c.errAt(it, fmt.Sprintf("hosts: %q is listed twice", h.Host))
		}
		c.Hosts = append(c.Hosts, h)
	}
	return nil
}

// loadRewrites reads url_rewrites: a from and a to each.
func (c *Config) loadRewrites(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "url_rewrites must be a list")
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return c.errAt(it, "url_rewrites: want a mapping with from and to")
		}
		from, ok1 := text(valueOf(it, "from"))
		to, ok2 := text(valueOf(it, "to"))
		if !ok1 || !ok2 {
			return c.errAt(it, "url_rewrites: from and to must be strings")
		}
		if from == "" || to == "" {
			return c.errAt(it, "url_rewrites: want a mapping with from and to")
		}
		c.URLRewrites = append(c.URLRewrites, URLRewrite{From: from, To: to})
	}
	return nil
}

// loadRepoAccounts reads repo_accounts: a match pattern and an account each.
func (c *Config) loadRepoAccounts(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "repo_accounts must be a list")
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return c.errAt(it, "repo_accounts: want a mapping with match and account")
		}
		mn, an := valueOf(it, "match"), valueOf(it, "account")
		m, ok1 := text(mn)
		a, ok2 := text(an)
		if !ok1 || !ok2 {
			return c.errAt(it, "repo_accounts: match and account must be strings")
		}
		if m == "" || a == "" {
			return c.errAt(it, "repo_accounts: want a mapping with match and account")
		}
		if _, err := path.Match(m, ""); err != nil {
			return c.errAt(mn, fmt.Sprintf("repo_accounts: match %q: %v", m, err))
		}
		if !c.hasAccount(a) {
			return c.errAt(an, fmt.Sprintf("repo_accounts: account %q is not in accounts", a))
		}
		c.RepoAccounts = append(c.RepoAccounts, RepoAccount{Match: m, Account: a})
	}
	return nil
}
