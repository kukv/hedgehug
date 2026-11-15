// Package apptest は、口のメモリ上の偽物と、口ごとの照合テストを持つ。どのテストからも import してよい。
package apptest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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

// ClockSkews は、ずれの警告の分をメモリに覚える偽物。
type ClockSkews struct {
	mu      sync.Mutex
	Minutes int
	// SkewErr と SetErr は、入れると Skew と SetSkew がそのエラーを返す(状態は変えない)。
	SkewErr error
	SetErr  error
}

func NewClockSkews() *ClockSkews { return &ClockSkews{} }

func (c *ClockSkews) Skew(context.Context) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.SkewErr != nil {
		return 0, c.SkewErr
	}
	return c.Minutes, nil
}

func (c *ClockSkews) SetSkew(_ context.Context, minutes int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.SetErr != nil {
		return c.SetErr
	}
	c.Minutes = minutes
	return nil
}

// Identity は、どの Store でも Login を「あなた」とする偽物。
type Identity struct {
	Login string
}

func (i Identity) Me(context.Context, port.StoreID) (string, error) { return i.Login, nil }

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
