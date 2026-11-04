package github

import (
	"net"
	"net/url"
	"strings"
)

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
