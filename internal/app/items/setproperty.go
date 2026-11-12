package items

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SetProperty は、1 つのプロパティの値を変える(02「入力の方法と保存」)。前と同じなら何も保存しない。
// アクティビティは「{ID} の{プロパティ名}を {値} に変更」。遷移の制約は後半(A3b)でここに足す(決定 2)。
func (s Service) SetProperty(ctx context.Context, store port.StoreID, id item.ID, prop change.ID, value json.RawMessage) (item.Item, error) {
	ec, err := s.editContext(ctx, store, id)
	if err != nil {
		return item.Item{}, err
	}
	p, err := ec.property(prop)
	if err != nil {
		return item.Item{}, err
	}
	if p.Type == workspace.TypeSystems || p.Type == workspace.TypeMulti {
		return item.Item{}, port.ErrNotFound // 入り切りは AddValue・RemoveValue
	}
	to, err := item.ParseInput(p, value, ec.settings)
	if errors.Is(err, item.ErrNoSuchValue) {
		return item.Item{}, port.ErrNotFound
	}
	if err != nil {
		return item.Item{}, err
	}
	if err := checkDatePair(ec.item, p, to); err != nil {
		return item.Item{}, err
	}
	summary := fmt.Sprintf("%s の%sを %s に変更", ec.display, p.Name, item.Display(p, to, ec.settings, ec.names, s.Clock.Now()))
	return s.writeValue(ctx, store, id, summary, func(it item.Item) []change.Draft {
		from := item.Current(it, p)
		if bytes.Equal(from, to) {
			return nil
		}
		return []change.Draft{change.NewSet(item.Field(p), from, to)}
	})
}

// checkDatePair は、開始日か終了日を変えるとき、保存済みのもう片方と比べる(決定 16)。
func checkDatePair(it item.Item, p workspace.Property, to json.RawMessage) error {
	if !p.Builtin || (p.Key != "start" && p.Key != "end") {
		return nil
	}
	var v string
	_ = json.Unmarshal(to, &v)
	if p.Key == "start" {
		return item.CheckDates(v, it.End)
	}
	return item.CheckDates(it.Start, v)
}

// writeValue は、保存の中でいまのアイテムを読み直し、drafts が返す行を 1 回で保存する。行がなければ何も残さない。
func (s Service) writeValue(ctx context.Context, store port.StoreID, id item.ID, summary string, drafts func(item.Item) []change.Draft) (item.Item, error) {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return item.Item{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		return tx.WriteItems().Append(id, drafts(it)...)
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, store, id)
}
