package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Inbox は port.Inbox の偽物。足した通知を覚える。Err を入れると、Add はそれを返して覚えない。
type Inbox struct {
	mu      sync.Mutex
	notices []port.Notice
	Err     error
}

func (i *Inbox) Add(_ context.Context, n port.Notice) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.Err != nil {
		return i.Err
	}
	i.notices = append(i.notices, n)
	return nil
}

// Notices は、足した通知を足した順に返す。
func (i *Inbox) Notices() []port.Notice {
	i.mu.Lock()
	defer i.mu.Unlock()
	return slices.Clone(i.notices)
}
