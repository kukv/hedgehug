package config

import (
	"fmt"
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

// Host returns the host name with its defaults filled in: the API at
// https://api.github.com for github.com and https://<host>/api/v3 elsewhere,
// the web at https://<host>.
func (c *Config) Host(name string) Host {
	h := Host{Host: name}
	for _, x := range c.Hosts {
		if x.Host == name {
			h = x
		}
	}
	if h.API == "" {
		h.API = "https://" + name + "/api/v3"
		if name == "github.com" {
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
	if name == "github.com" {
		return true
	}
	for _, x := range c.Hosts {
		if x.Host == name {
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

// loadHosts reads hosts: a host each, with an optional api, web and client_id.
func (c *Config) loadHosts(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "hosts: want a list")
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
			if v := valueOf(it, f.key); v != nil && v.Tag != "!!null" {
				*f.dst = v.Value
			}
		}
		if h.Host == "" {
			return c.errAt(it, "hosts: host is missing")
		}
		for _, f := range []struct{ key, v string }{{"api", h.API}, {"web", h.Web}} {
			if f.v != "" && !webURL(f.v) {
				return c.errAt(it, fmt.Sprintf("hosts: %s %q: want an http or https URL", f.key, f.v))
			}
		}
		if slices.ContainsFunc(c.Hosts, func(x Host) bool { return x.Host == h.Host }) {
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
		return c.errAt(seq, "url_rewrites: want a list")
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return c.errAt(it, "url_rewrites: want a mapping with from and to")
		}
		from, to := valueOf(it, "from"), valueOf(it, "to")
		if from == nil || from.Value == "" || to == nil || to.Value == "" {
			return c.errAt(it, "url_rewrites: want a mapping with from and to")
		}
		c.URLRewrites = append(c.URLRewrites, URLRewrite{From: from.Value, To: to.Value})
	}
	return nil
}

// loadRepoAccounts reads repo_accounts: a match pattern and an account each.
func (c *Config) loadRepoAccounts(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "repo_accounts: want a list")
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return c.errAt(it, "repo_accounts: want a mapping with match and account")
		}
		m, a := valueOf(it, "match"), valueOf(it, "account")
		if m == nil || m.Value == "" || a == nil || a.Value == "" {
			return c.errAt(it, "repo_accounts: want a mapping with match and account")
		}
		if _, err := path.Match(m.Value, ""); err != nil {
			return c.errAt(m, fmt.Sprintf("repo_accounts: match %q: %v", m.Value, err))
		}
		if !c.hasAccount(a.Value) {
			return c.errAt(a, fmt.Sprintf("repo_accounts: account %q is not in accounts", a.Value))
		}
		c.RepoAccounts = append(c.RepoAccounts, RepoAccount{Match: m.Value, Account: a.Value})
	}
	return nil
}
