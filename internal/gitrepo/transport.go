package gitrepo

import (
	"context"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// go-git's file transport runs git-upload-pack and git-receive-pack. Hedgehug
// must not need git, so the file scheme is served in-process instead.
func init() {
	client.InstallProtocol("file", fileTransport{server.NewClient(fileLoader{})})
}

// fileTransport is go-git's in-process server with one fix: that server fails
// with "object not found" when the fetching clone says it has a commit the
// remote does not, which is every fetch after a local commit. Such commits are
// dropped from the request, as git's own server ignores them.
type fileTransport struct{ transport.Transport }

func (t fileTransport) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	s, err := t.Transport.NewUploadPackSession(ep, auth)
	if err != nil {
		return nil, err
	}
	sto, err := fileLoader{}.Load(ep)
	if err != nil {
		_ = s.Close()
		return nil, err
	}
	return knownHaves{s, sto}, nil
}

type knownHaves struct {
	transport.UploadPackSession
	sto storer.Storer
}

func (s knownHaves) UploadPack(ctx context.Context, req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	r := *req
	r.Haves = slices.DeleteFunc(slices.Clone(req.Haves), func(h plumbing.Hash) bool {
		return s.sto.HasEncodedObject(h) != nil
	})
	return s.UploadPackSession.UploadPack(ctx, &r)
}

type fileLoader struct{}

// Load opens the bare repository at the endpoint's path.
func (fileLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	p := filepath.FromSlash(ep.Path)
	if !isDir(filepath.Join(p, "objects")) {
		return nil, transport.ErrRepositoryNotFound
	}
	return newCachedStorage(filesystem.NewStorage(osfs.New(p), cache.NewObjectLRUDefault())), nil
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
