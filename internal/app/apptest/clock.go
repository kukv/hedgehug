// Package apptest は、口のメモリ上の偽物と、口ごとの照合テストを持つ。どのテストからも import してよい。
package apptest

import (
	"sync"
	"time"
)

// Clock は、呼ぶたびに Step だけ進む時計。
type Clock struct {
	mu  sync.Mutex
	now time.Time
	// Step は、Now を呼ぶたびに進む幅。変えるなら Now を呼ぶ前に変える(Now と同時に変えてはいけない)。
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
