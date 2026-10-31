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

func defaultHTTP(hc *http.Client) *http.Client {
	if hc == nil {
		return &http.Client{Timeout: 30 * time.Second}
	}
	return hc
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
		return nil, err
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

// sameOrigin reports whether u has the scheme and host of the API base.
func (c *Client) sameOrigin(u string) bool {
	a, err1 := neturl.Parse(c.api)
	b, err2 := neturl.Parse(u)
	return err1 == nil && err2 == nil && a.Scheme == b.Scheme && a.Host == b.Host
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
