package transfers

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// 移すプロジェクト(Web の「ログイン」)のアイテムと、ほかのプロジェクト(Mobile の「アプリ」)のアイテムとの関連は、
// 外との関連として両方の側から集める(別の Store への移動で解除するため。A5b-3)。
func TestReadProjectSourceCollectsRelationsToOtherProjects(t *testing.T) {
	const store port.StoreID = "S1"
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	if err := apptest.SampleMobile(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "関連を追加"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleMobileEpic, change.NewAdd("relations", item.Relation{Type: "relates", To: apptest.SampleTask}))
	}); err != nil {
		t.Fatal(err)
	}
	var src projectSource
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) {
		src, err = readProjectSource(r, apptest.SampleProject, port.LiveProject)
		return
	}); err != nil {
		t.Fatal(err)
	}
	want := transfer.Cut{From: apptest.SampleMobileEpic, To: apptest.SampleTask, Type: "relates"}
	if len(src.set.Outside) != 1 || src.set.Outside[0] != want {
		t.Errorf("outside = %+v, want [%+v]", src.set.Outside, want)
	}
}
