package apptest

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Inbox は port.InboxBook の偽物。足した通知を覚える。Err を入れると、Add はそれを返して覚えない。
// Clock が nil なら、time.Unix(0, 0) から 1 秒ずつ進める。
type Inbox struct {
	mu    sync.Mutex
	rows  []port.NoticeRow
	next  port.NoticeID
	Err   error
	Clock port.Clock
}

func (i *Inbox) Add(_ context.Context, n port.Notice) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.Err != nil {
		return i.Err
	}
	if i.Clock == nil {
		c := NewClock(time.Unix(0, 0))
		c.Step = time.Second
		i.Clock = c
	}
	i.next++
	i.rows = append(i.rows, port.NoticeRow{ID: i.next, Notice: n, At: i.Clock.Now()})
	return nil
}

// Notices は、足した通知を足した順に返す。
func (i *Inbox) Notices() []port.Notice {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := make([]port.Notice, len(i.rows))
	for k, r := range i.rows {
		out[k] = r.Notice
	}
	return out
}

func matches(r port.NoticeRow, stores []port.StoreID, kinds []port.NoticeKind) bool {
	return slices.Contains(stores, r.Store) && (len(kinds) == 0 || slices.Contains(kinds, r.Kind))
}

func (i *Inbox) List(_ context.Context, q port.NoticeQuery) ([]port.NoticeRow, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	var out []port.NoticeRow
	for _, r := range i.rows {
		if !matches(r, q.Stores, q.Kinds) {
			continue
		}
		if c := q.Before; c != nil && !r.At.Before(c.At) && (!r.At.Equal(c.At) || r.ID >= c.ID) {
			continue
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b port.NoticeRow) int {
		if c := b.At.Compare(a.At); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (i *Inbox) Unread(_ context.Context, stores []port.StoreID) (map[port.NoticeKind]int, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := map[port.NoticeKind]int{}
	for _, r := range i.rows {
		if !r.Read && matches(r, stores, nil) {
			out[r.Kind]++
		}
	}
	return out, nil
}

// find は、id の行の位置を返す(i.mu を持って呼ぶ)。
func (i *Inbox) find(id port.NoticeID) (int, error) {
	k := slices.IndexFunc(i.rows, func(r port.NoticeRow) bool { return r.ID == id })
	if k < 0 {
		return 0, port.ErrNotFound
	}
	return k, nil
}

func (i *Inbox) Get(_ context.Context, id port.NoticeID) (port.NoticeRow, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	k, err := i.find(id)
	if err != nil {
		return port.NoticeRow{}, err
	}
	return i.rows[k], nil
}

func (i *Inbox) SetRead(_ context.Context, id port.NoticeID, read bool) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	k, err := i.find(id)
	if err != nil {
		return err
	}
	i.rows[k].Read = read
	return nil
}

func (i *Inbox) ReadAll(_ context.Context, stores []port.StoreID, kinds []port.NoticeKind) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	for k, r := range i.rows {
		if matches(r, stores, kinds) {
			i.rows[k].Read = true
		}
	}
	return nil
}

func (i *Inbox) Resolve(_ context.Context, id port.NoticeID, res port.NoticeResult) error {
	if res != port.NoticeApplied && res != port.NoticeKept {
		return port.ErrInvalidOperation
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	k, err := i.find(id)
	if err != nil {
		return err
	}
	if i.rows[k].Result != "" {
		return refusal.New("この通知はすでに処理されています")
	}
	i.rows[k].Read, i.rows[k].Result = true, res
	return nil
}

func (i *Inbox) Prune(_ context.Context, store port.StoreID, targets []change.ID) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.rows = slices.DeleteFunc(i.rows, func(r port.NoticeRow) bool {
		return r.Store == store && (slices.Contains(targets, r.Item) || (r.Item == "" && slices.Contains(targets, r.Workspace)))
	})
	return nil
}

func (i *Inbox) ForgetStore(_ context.Context, store port.StoreID) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.rows = slices.DeleteFunc(i.rows, func(r port.NoticeRow) bool { return r.Store == store })
	return nil
}

// JudgedSaves は port.JudgedSaves の偽物。
type JudgedSaves struct {
	mu    sync.Mutex
	saves map[port.StoreID][]port.SaveKey
}

func NewJudgedSaves() *JudgedSaves { return &JudgedSaves{saves: map[port.StoreID][]port.SaveKey{}} }

func (j *JudgedSaves) Started(_ context.Context, id port.StoreID) (bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.saves[id]) > 0, nil
}

func (j *JudgedSaves) Known(_ context.Context, id port.StoreID, k port.SaveKey) (port.Match, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	m := port.NotJudged
	for _, s := range j.saves[id] {
		if s.Version == k.Version {
			return port.SameVersion, nil
		}
		if k.Op != "" && s.Op == k.Op && s.Author == k.Author && s.At.Equal(k.At) {
			m = port.SameSave
		}
	}
	return m, nil
}

func (j *JudgedSaves) Mark(_ context.Context, id port.StoreID, keys []port.SaveKey) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, k := range keys {
		if !slices.ContainsFunc(j.saves[id], func(s port.SaveKey) bool { return s.Version == k.Version }) {
			j.saves[id] = append(j.saves[id], k)
		}
	}
	return nil
}

func (j *JudgedSaves) ForgetStore(_ context.Context, id port.StoreID) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.saves, id)
	return nil
}
