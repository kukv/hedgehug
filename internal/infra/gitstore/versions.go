package gitstore

import (
	"context"
	"maps"

	"github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Versions は、取り付けた Store ごとの先端のコミット(port.StoreVersions)。保存・同期・ほかのプログラムの
// コミットのどれでも変わる。読めない Store は含めない。
func (s *Stores) Versions(context.Context) (map[port.StoreID]string, error) {
	s.mu.Lock()
	dirs := maps.Clone(s.dirs)
	s.mu.Unlock()
	out := make(map[port.StoreID]string, len(dirs))
	for id, dir := range dirs {
		repo, err := git.PlainOpen(dir)
		if err != nil {
			continue
		}
		head, err := repo.Head()
		if err != nil {
			continue
		}
		out[id] = head.Hash().String()
	}
	return out, nil
}
