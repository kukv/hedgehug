package syncing

import (
	"context"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Watch は、Store の版の変化を見張り、変わった Store を Events に流す(06「画面への反映」、決定 C2-20)。
// 書き手(このタブ、別のタブ、同期)を問わない。
type Watch struct {
	Versions port.StoreVersions
	Events   port.Events

	last map[port.StoreID]string
}

// Step は、版を読み、前の Step から変わった Store(増えた・消えたものを含む)を、Store ID の順に流す。
// 最初の Step は覚えるだけ。
func (w *Watch) Step(ctx context.Context) error {
	now, err := w.Versions.Versions(ctx)
	if err != nil {
		return err
	}
	if w.last != nil {
		var changed []port.StoreID
		for id, v := range now {
			if w.last[id] != v {
				changed = append(changed, id)
			}
		}
		for id := range w.last {
			if _, ok := now[id]; !ok {
				changed = append(changed, id)
			}
		}
		slices.Sort(changed)
		for _, id := range changed {
			w.Events.Publish(port.Event{Kind: port.EventStore, Store: id})
		}
	}
	w.last = now
	return nil
}

// Run は、ctx が終わるまで、every ごとに Step を呼ぶ。Step の失敗は、次の Step でやり直す。
func (w *Watch) Run(ctx context.Context, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		_ = w.Step(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
