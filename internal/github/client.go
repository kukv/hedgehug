// Package github is a client of the GitHub REST API: the endpoints Hedgehug
// uses to find, create and check its store repositories, and the device flow.
package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
)

// The errors of a call, by the status of the answer.
var (
	ErrUnauthorized = errors.New("GitHub refused the token (401)")
	ErrForbidden    = errors.New("GitHub denied it (403)")
	ErrNotFound     = errors.New("GitHub has no such thing, or it is not visible to the token (404)")
	ErrExists       = errors.New("a repository of that name exists")
)

// RateLimitError means the rate limit is used up until Reset.
type RateLimitError struct{ Reset time.Time }

func (e *RateLimitError) Error() string {
	return "GitHub API rate limit; try again after " + e.Reset.Format(time.RFC3339)
}

// Describe says in err which account did what, and for a 401 what to run: a
// failure of the API is shown to the person as
// "github.com/alice-corp cannot read the permission of org-a/hh-shared: …
// (run hedgehug auth login work)". accountID is <host>/<login>, accountName
// the name in config.yaml, doing what was being done.
func Describe(err error, accountID, accountName, doing string) error {
	if err == nil {
		return nil
	}
	err = fmt.Errorf("%s cannot %s: %w", accountID, doing, err)
	if errors.Is(err, ErrUnauthorized) {
		err = fmt.Errorf("%w (run hedgehug auth login %s)", err, accountName)
	}
	return err
}

// Client calls the REST API of one host with one token.
type Client struct {
	api, token string
	hc         *http.Client
}

// defaultHTTP is hc, or a client with a 30-second timeout when hc is nil (a
// copy of hc with no timeout of its own gets the 30 seconds too), in
// either case refusing a redirect to another origin than the request's:
// the token must not follow it. hc is copied, not changed; a policy it has
// of its own is applied after this one.
func defaultHTTP(hc *http.Client) *http.Client {
	var c http.Client
	if hc == nil {
		c.Timeout = 30 * time.Second
	} else {
		c = *hc
		if c.Timeout == 0 {
			c.Timeout = 30 * time.Second
		}
	}
	own := c.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if originOf(req.URL) != originOf(via[0].URL) {
			return fmt.Errorf("the redirect from %s to %s is not followed: it is another origin, and the token stays here",
				via[0].URL.Redacted(), req.URL.Redacted())
		}
		if own != nil {
			return own(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &c
}

// New makes a client of the host h with token. hc nil is a client with a
// 30-second timeout.
func New(h config.Host, token string, hc *http.Client) *Client {
	return &Client{api: h.API, token: token, hc: defaultHTTP(hc)}
}

// do calls method on the API path with the JSON of in (nil for none), and
// decodes the answer into out (nil to drop it). It returns the headers of the
// answer.
func (c *Client) do(method, url string, in, out any) (http.Header, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.hc.Do(req) //nolint:gosec // G704: the URL is the API base of a host in config.yaml
	if err != nil {
		return nil, requestErr(method, url, err)
	}
	defer func() { _ = res.Body.Close() }()
	data, readErr := io.ReadAll(res.Body)
	if err := statusError(res, data); err != nil {
		return res.Header, fmt.Errorf("%s %s: %w", method, url, err)
	}
	if readErr != nil {
		return res.Header, fmt.Errorf("%s %s: reading the answer: %w", method, url, readErr)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return res.Header, fmt.Errorf("%s %s: %w", method, url, err)
		}
	}
	return res.Header, nil
}

// requestErr is the error of a request that got no answer, as "METHOD url:
// cause". The *url.Error of net/http carries the method and the URL itself, so
// it is replaced by its cause, which errors.Is and errors.As still reach.
func requestErr(method, u string, err error) error {
	var ue *neturl.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return fmt.Errorf("%s %s: %w", method, u, err)
}

func statusError(res *http.Response, data []byte) error {
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	if res.StatusCode == http.StatusTooManyRequests || (res.StatusCode == http.StatusForbidden && (res.Header.Get("X-RateLimit-Remaining") == "0" || res.Header.Get("Retry-After") != "")) {
		reset := time.Now().Add(time.Minute)
		if s, err := strconv.ParseInt(res.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			reset = time.Unix(s, 0)
		} else if s, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil {
			reset = time.Now().Add(time.Duration(s) * time.Second)
		}
		return &RateLimitError{Reset: reset}
	}
	var msg struct {
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(data, &msg)
	switch res.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		if msg.Message == "" {
			return ErrForbidden
		}
		return fmt.Errorf("%w: %s", ErrForbidden, msg.Message)
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnprocessableEntity:
		for _, e := range msg.Errors {
			if strings.Contains(e.Message, "already exists") {
				return ErrExists
			}
		}
	}
	return fmt.Errorf("GitHub answered %d: %s", res.StatusCode, msg.Message)
}

// sameOrigin reports whether u has the origin of the API base: the scheme,
// the host without case and the port, where the port of the scheme is the
// default one when it is not written.
func (c *Client) sameOrigin(u string) bool {
	a, err1 := neturl.Parse(c.api)
	b, err2 := neturl.Parse(u)
	return err1 == nil && err2 == nil && originOf(a) == originOf(b)
}

func originOf(u *neturl.URL) string {
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[scheme]
	}
	return scheme + "://" + strings.ToLower(u.Hostname()) + ":" + port
}

var nextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// getAll gets every page of a list from url and appends its items to out.
func getAll[T any](c *Client, url string) ([]T, error) {
	var all []T
	for url != "" {
		var page []T
		h, err := c.do("GET", url, nil, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		next := ""
		if m := nextLink.FindStringSubmatch(h.Get("Link")); m != nil {
			next = m[1]
			if !c.sameOrigin(next) {
				return nil, fmt.Errorf("GET %s: the next page %s is not on the API host; not sending the token there", url, next)
			}
		}
		url = next
	}
	return all, nil
}
