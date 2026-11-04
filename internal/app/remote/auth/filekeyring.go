package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// fileKeyringWait is how long a write waits for another hh process that is
// writing the same file.
const fileKeyringWait = 5 * time.Second

// FileKeyring is a Keyring kept in a JSON file at path, for the E2E build,
// where there is no OS keychain to use (CI) or to spare (a developer's own
// Keychain). It reads the file on every call and writes it whole on every
// change, so that several hh processes of one test share it.
func FileKeyring(path string) Keyring { return fileKeyring{path: path} }

type fileKeyring struct{ path string }

func fileKeyringKey(service, user string) string { return service + "\x00" + user }

// load reads the entries; a missing file is an empty keyring.
func (k fileKeyring) load() (map[string]string, error) {
	b, err := osfs.ReadFile(k.path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("the file keyring: %w", err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("the file keyring %s: %w", k.path, err)
	}
	return m, nil
}

func (k fileKeyring) Get(service, user string) (string, error) {
	m, err := k.load()
	if err != nil {
		return "", err
	}
	v, ok := m[fileKeyringKey(service, user)]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (k fileKeyring) Set(service, user, secret string) error {
	return k.update(func(m map[string]string) error {
		m[fileKeyringKey(service, user)] = secret
		return nil
	})
}

func (k fileKeyring) Delete(service, user string) error {
	return k.update(func(m map[string]string) error {
		key := fileKeyringKey(service, user)
		if _, ok := m[key]; !ok {
			return ErrNotFound
		}
		delete(m, key)
		return nil
	})
}

// update runs fn on the entries under a lock and writes them back when fn
// succeeds. The file is written through a temporary file, which is 0600.
func (k fileKeyring) update(fn func(map[string]string) error) error {
	return osfs.WithFileLock(k.path+".lock", fileKeyringWait, func() error {
		m, err := k.load()
		if err != nil {
			return err
		}
		if err := fn(m); err != nil {
			return err
		}
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return osfs.WriteAtomic(k.path, b)
	})
}
