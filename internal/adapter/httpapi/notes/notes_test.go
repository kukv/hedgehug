package notes_test

import (
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
)

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{
		Descriptions: appdescriptions.Service{Stores: s, Identity: apptest.Identity{Login: "taro"}},
	})
	return api
}
