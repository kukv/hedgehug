package gitrepo

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// go-git's file transport runs git-upload-pack and git-receive-pack. Hedgehug
// must not need git, so the file scheme is served in-process instead.
func init() {
	client.InstallProtocol("file", server.NewClient(fileLoader{}))
}

// driveRE matches the /C:/... that a file URL of a Windows path turns into.
var driveRE = regexp.MustCompile(`^/[A-Za-z]:[/\\]`)

type fileLoader struct{}

// Load opens the bare repository at the endpoint's path.
func (fileLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	p := ep.Path
	if driveRE.MatchString(p) {
		p = p[1:]
	}
	p = filepath.FromSlash(p)
	if !isDir(filepath.Join(p, "objects")) {
		return nil, transport.ErrRepositoryNotFound
	}
	return filesystem.NewStorage(osfs.New(p), cache.NewObjectLRUDefault()), nil
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
