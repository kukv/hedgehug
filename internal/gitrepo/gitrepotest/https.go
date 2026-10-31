// Package gitrepotest serves https remotes from local bare repositories in
// tests, in-process, and records the auth each session was given.
package gitrepotest

import (
	"net/url"
	"sync"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

type loader struct {
	route func(string) (string, bool)
}

func (l loader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	dir, ok := l.route(ep.String())
	if !ok {
		return nil, transport.ErrRepositoryNotFound
	}
	return filesystem.NewStorage(osfs.New(dir), cache.NewObjectLRUDefault()), nil
}

type recorder struct {
	transport.Transport
	mu    *sync.Mutex
	auths *[]transport.AuthMethod
	deny  func(string) bool
}

func (r recorder) record(a transport.AuthMethod) {
	if r.auths != nil {
		r.mu.Lock()
		*r.auths = append(*r.auths, a)
		r.mu.Unlock()
	}
}

func (r recorder) NewUploadPackSession(ep *transport.Endpoint, a transport.AuthMethod) (transport.UploadPackSession, error) {
	r.record(a)
	return r.Transport.NewUploadPackSession(ep, a)
}

func (r recorder) NewReceivePackSession(ep *transport.Endpoint, a transport.AuthMethod) (transport.ReceivePackSession, error) {
	r.record(a)
	if r.deny != nil && r.deny(ep.String()) {
		return nil, transport.ErrAuthorizationFailed
	}
	return r.Transport.NewReceivePackSession(ep, a)
}

// InstallHTTPS serves http and https URLs from the bare repositories route
// gives, records the auth of each session in auths (when not nil), and fails
// the pushes to the URLs deny reports. The usual transports are back after
// the test. The tests that call it must not run in parallel.
func InstallHTTPS(t testing.TB, route func(u string) (dir string, ok bool), auths *[]transport.AuthMethod, deny func(u string) bool) {
	t.Helper()
	tr := recorder{Transport: server.NewClient(loader{route: route}), mu: &sync.Mutex{}, auths: auths, deny: deny}
	client.InstallProtocol("https", tr)
	client.InstallProtocol("http", tr)
	t.Cleanup(func() {
		client.InstallProtocol("https", githttp.DefaultClient)
		client.InstallProtocol("http", githttp.DefaultClient)
	})
}

// Under routes the URLs under base (such as https://github.com/) to the
// same path under dir.
func Under(base, dir string) func(string) (string, bool) {
	return func(u string) (string, bool) {
		bu, err1 := url.Parse(base)
		uu, err2 := url.Parse(u)
		if err1 != nil || err2 != nil || bu.Host != uu.Host {
			return "", false
		}
		return dir + uu.Path, true
	}
}
