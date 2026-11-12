package apptest

import (
	"fmt"
	"sync"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// IDs は、辞書順に並ぶ 26 文字の ID を順に返す。
type IDs struct {
	mu sync.Mutex
	n  int
}

func NewIDs() *IDs { return &IDs{} }

func (g *IDs) New() change.ID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return change.ID(fmt.Sprintf("01TEST%020d", g.n))
}
