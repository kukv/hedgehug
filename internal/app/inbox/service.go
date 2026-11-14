// Package inbox は、受信箱の一覧・未読数・通知の操作(06「受信箱」)。
package inbox

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Tab は、受信箱のタブ。
type Tab string

const (
	TabAll        Tab = "all"
	TabMention    Tab = "mention"
	TabAssign     Tab = "assign"
	TabWatch      Tab = "watch"
	TabOverwrite  Tab = "overwrite"
	TabAutomation Tab = "automation"
	TabRepair     Tab = "repair"
)

// tabs は、すべてのタブ。
var tabs = []Tab{TabAll, TabMention, TabAssign, TabWatch, TabOverwrite, TabAutomation, TabRepair}

// KindsOf は、タブに出す通知の種類。すべては nil。
func KindsOf(t Tab) []port.NoticeKind {
	switch t {
	case TabAll:
		return nil
	case TabAutomation:
		return []port.NoticeKind{port.NoticeAutomation, port.NoticeAutomationStopped}
	}
	return []port.NoticeKind{port.NoticeKind(t)}
}

// TabOf は、通知の種類を出すタブ(すべて以外)。
func TabOf(k port.NoticeKind) Tab {
	if k == port.NoticeAutomationStopped {
		return TabAutomation
	}
	return Tab(k)
}

// Service は、受信箱の usecase。読むのは表示中の Store の通知だけ(C4a-5)。
type Service struct {
	Book   port.InboxBook
	Stores port.Stores
	List   port.StoreList
	Events port.Events
	// ApplyIgnoringWorkflow は、止まった通知の「制約を無視して…」の保存。cmd が automations.Service.ApplyIgnoringWorkflow を渡す
	// (app の機能どうしを import しない)。
	ApplyIgnoringWorkflow func(ctx context.Context, store port.StoreID, id item.ID, rule, from, status change.ID) error
}

// Counts は、タブごとの未読数。7 つのタブすべての鍵を持つ。
func (s Service) Counts(ctx context.Context) (map[Tab]int, error) {
	infos, err := s.visible(ctx)
	if err != nil {
		return nil, err
	}
	return s.counts(ctx, ids(infos))
}

// visible は、表示中の Store を返す。完全に削除したものの通知を、先に消す(C4a-6)。
func (s Service) visible(ctx context.Context) ([]port.StoreInfo, error) {
	infos, err := s.List.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if err := s.prune(ctx, info.ID); err != nil {
			return nil, err
		}
	}
	return infos, nil
}

// purgedEntities は、通知の item か workspace に入りうる、完全な削除の記録の種類。
var purgedEntities = []string{"item", "project", "workspace"}

// prune は、Store の purged.jsonl にあるアイテム・プロジェクト・ワークスペースの通知を消す。
func (s Service) prune(ctx context.Context, store port.StoreID) error {
	var targets []change.ID
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		ps, err := r.Purged().List()
		for _, p := range ps {
			if slices.Contains(purgedEntities, p.Entity) {
				targets = append(targets, p.Target)
			}
		}
		return err
	})
	if err != nil || len(targets) == 0 {
		return err
	}
	return s.Book.Prune(ctx, store, targets)
}

// counts は、stores の未読数をタブごとに数える。
func (s Service) counts(ctx context.Context, stores []port.StoreID) (map[Tab]int, error) {
	out := make(map[Tab]int, len(tabs))
	for _, t := range tabs {
		out[t] = 0
	}
	if len(stores) == 0 {
		return out, nil
	}
	byKind, err := s.Book.Unread(ctx, stores)
	if err != nil {
		return nil, err
	}
	for k, n := range byKind {
		out[TabAll] += n
		out[TabOf(k)] += n
	}
	return out, nil
}

func ids(infos []port.StoreInfo) []port.StoreID {
	out := make([]port.StoreID, len(infos))
	for k, info := range infos {
		out[k] = info.ID
	}
	return out
}
