package history_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	apphistory "github.com/kukv/hedgehug/internal/app/history"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dhistory "github.com/kukv/hedgehug/internal/domain/history"
)

const store port.StoreID = "S1"

func setup(t *testing.T) (apphistory.Service, *apptest.Stores) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	return apphistory.Service{
		Stores: s, StoreList: apptest.StoreList{Stores: []port.StoreInfo{{ID: store, Name: "main"}}},
		Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)),
	}, s
}

func write(t *testing.T, s port.Stores, login string, fn func(tx port.Tx) error) {
	t.Helper()
	op := port.Operation{Actor: change.User(login), Summary: "テスト"}
	if err := s.Write(t.Context(), store, op, fn); err != nil {
		t.Fatal(err)
	}
}

func texts(rows []dhistory.Row) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Who+": "+r.Text)
	}
	return out
}

func TestItemHistory(t *testing.T) {
	svc, s := setup(t)
	write(t, s, "hanako", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン"))
	})
	// 値の削除と同じ保存のアイテムの行は出さない(B2-9)。
	write(t, s, "taro", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete("systems/"+string(apptest.SampleSystemWeb))),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone)),
		)
	})
	write(t, s, "taro", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusDone, apptest.SampleStatusInProgress))
	})
	rows, err := svc.Item(t.Context(), store, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"taro: ステータスを Done → InProgress", "hanako: タイトルを「ログイン」に変更", "taro: 作成"}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
	if _, err := svc.Item(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないアイテム: %v", err)
	}
}

func TestProjectHistory(t *testing.T) {
	svc, s := setup(t)
	write(t, s, "hanako", func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject,
			change.NewSet("title", "ログイン", "認証"),
			change.NewSet("owner", "taro", "hanako"),
		)
	})
	rows, err := svc.Project(t.Context(), store, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"hanako: オーナーを hanako に変更", "hanako: タイトルを 認証 に変更", "taro: 作成"}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
	if _, err := svc.Project(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないプロジェクト: %v", err)
	}
}
