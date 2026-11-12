// Package gitstore は、Store を git のリポジトリで持つ port.Stores の実装。
// git の言葉(コミット、ブランチ、clone)は、このパッケージの外に出さない。
package gitstore

import (
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const (
	botName  = change.BotLogin
	botEmail = "bot@hedgehug.invalid"
)

// Stores は、取り付けた Store を git のリポジトリとして読み書きする。
type Stores struct {
	clock port.Clock
	ids   port.IDs

	mu   sync.Mutex
	dirs map[port.StoreID]string
}

func New(clock port.Clock, ids port.IDs) *Stores {
	return &Stores{clock: clock, ids: ids, dirs: map[port.StoreID]string{}}
}

// Attach は、dir の Store を開いて使えるようにし、その Store ID を返す。
func (s *Stores) Attach(dir string) (port.StoreID, error) {
	y, err := readStoreYAML(dir)
	if err != nil {
		return "", err
	}
	id := port.StoreID(y.Store)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirs[id] = dir
	return id, nil
}

func (s *Stores) dir(id port.StoreID) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.dirs[id]
	if !ok {
		return "", port.ErrStoreNotFound
	}
	return d, nil
}

func signature(name, email string, at time.Time) *object.Signature {
	return &object.Signature{Name: name, Email: email, When: at}
}
