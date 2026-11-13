// Package gitstore は、Store を git のリポジトリで持つ port.Stores の実装。
// git の言葉(コミット、ブランチ、clone)は、このパッケージの外に出さない。
package gitstore

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
	"golang.org/x/sys/unix"

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

	// beforeCommit は、ファイルを書いたあと、コミットの前に呼ぶ。テストで失敗を起こすためのもの。
	beforeCommit func() error
	// afterEntity は、1 つのエンティティのファイルを書いてインデックスに足すたびに呼ぶ。テストで失敗を起こすためのもの。
	afterEntity func() error
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

// lock は、Store ごとのファイルロックをかける。1 回の操作のあいだ持つ。画面のサーバーと CLI が
// 同じ Store に書いても、コミットは 1 つずつ順に作られる(保存形式「技術構成」)。
// flock はファイルを開くごとに働くので、同じプロセスの中の 2 つの操作も順になる。
func lock(root string) (unlock func(), err error) {
	f, err := os.OpenFile(filepath.Join(root, ".git", "hedgehug.lock"), os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // G304: パスは Store の根と決まったファイル名から作る
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	// unlock は 2 回呼んでもよい(閉じたあとのファイルに Flock しない)。
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
			_ = f.Close()
		})
	}, nil
}
