// Package apptest は、口のメモリ上の偽物と、口ごとの照合テストを持つ。どのテストからも import してよい。
package apptest

import (
	"fmt"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Clock は、呼ぶたびに Step だけ進む時計。
type Clock struct {
	mu   sync.Mutex
	now  time.Time
	Step time.Duration
}

func NewClock(start time.Time) *Clock { return &Clock{now: start, Step: time.Millisecond} }

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now
	c.now = c.now.Add(c.Step)
	return t
}

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
