package gitrepo

import (
	"errors"
	"fmt"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// LogEntry is one commit as read back from the history.
type LogEntry struct {
	Hash     plumbing.Hash
	Author   Signature
	When     time.Time
	Subject  string
	Trailers []Trailer
}

// Trailer returns the value of the first trailer named key.
func (e LogEntry) Trailer(key string) (string, bool) {
	for _, t := range e.Trailers {
		if t.Key == key {
			return t.Value, true
		}
	}
	return "", false
}

// TrailerValues returns the values of every trailer named key, in the order
// they are written.
func (e LogEntry) TrailerValues(key string) []string {
	var out []string
	for _, t := range e.Trailers {
		if t.Key == key {
			out = append(out, t.Value)
		}
	}
	return out
}

// Log returns the commits reachable from HEAD, newest first. With prefixes,
// only commits that touch a path starting with one of them are returned. A
// limit above zero caps the number of entries. A repository without commits
// has an empty log.
func (r *Repo) Log(prefixes []string, limit int) ([]LogEntry, error) {
	opts := &git.LogOptions{}
	if len(prefixes) > 0 {
		opts.PathFilter = func(p string) bool {
			for _, pre := range prefixes {
				if strings.HasPrefix(p, pre) {
					return true
				}
			}
			return false
		}
	}
	it, err := r.r.Log(opts)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var out []LogEntry
	err = it.ForEach(func(c *object.Commit) error {
		out = append(out, entry(c))
		if limit > 0 && len(out) >= limit {
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LogFrom returns the commits from from along the first parents, newest
// first: the history of the line from was made on, without the commits a
// merge brought in. A limit above zero caps the number of entries. The zero
// hash has an empty log.
func (r *Repo) LogFrom(from plumbing.Hash, limit int) ([]LogEntry, error) {
	var out []LogEntry
	for h := from; !h.IsZero(); {
		c, err := r.r.CommitObject(h)
		if err != nil {
			return nil, fmt.Errorf("commit %s: %w", h, err)
		}
		out = append(out, entry(c))
		if limit > 0 && len(out) >= limit {
			break
		}
		h = plumbing.ZeroHash
		if len(c.ParentHashes) > 0 {
			h = c.ParentHashes[0]
		}
	}
	return out, nil
}

func entry(c *object.Commit) LogEntry {
	subject, trailers := parseMessage(c.Message)
	return LogEntry{
		Hash:     c.Hash,
		Author:   Signature{Name: c.Author.Name, Email: c.Author.Email},
		When:     c.Author.When,
		Subject:  subject,
		Trailers: trailers,
	}
}

// parseMessage is the inverse of message: the subject is the first line and
// the trailers are the "Key: Value" lines of the last paragraph, if every
// line of it is one and a blank line follows the subject.
func parseMessage(msg string) (string, []Trailer) {
	msg = strings.TrimRight(msg, "\n")
	subject, rest, found := strings.Cut(msg, "\n")
	if !found {
		return subject, nil
	}
	if !strings.HasPrefix(rest, "\n") {
		return subject, nil // no blank line after the subject
	}
	paras := strings.Split(strings.TrimLeft(rest, "\n"), "\n\n")
	last := paras[len(paras)-1]
	var trailers []Trailer
	for _, line := range strings.Split(last, "\n") {
		k, v, ok := strings.Cut(line, ": ")
		if !ok || k == "" || strings.ContainsAny(k, " \t") {
			return subject, nil
		}
		trailers = append(trailers, Trailer{Key: k, Value: v})
	}
	return subject, trailers
}
