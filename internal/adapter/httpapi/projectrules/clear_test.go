package projectrules_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/projectrules"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojectrules "github.com/kukv/hedgehug/internal/app/projectrules"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

// 空のテンプレートは、プロジェクトの既定をワークスペースと同じに戻す。
func TestEmptyTemplateDefaultClearsIt(t *testing.T) {
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	projectrules.Register(api, appprojectrules.Service{Stores: s, Identity: apptest.Identity{Login: "taro"}})
	defaults := func() map[string]string {
		var out map[string]string
		err := s.Read(t.Context(), "S1", func(r port.Reader) error {
			log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
			out = projectrule.FromLog(log).Of(apptest.SampleProject).TemplateDefaults
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	url := rules + "/template-defaults/Task"
	if res := api.Put(url, map[string]any{"template": "@none"}); res.Code != http.StatusNoContent || defaults()["Task"] != "@none" {
		t.Fatalf("set: %d %s %v", res.Code, res.Body, defaults())
	}
	if res := api.Put(url, map[string]any{"template": ""}); res.Code != http.StatusNoContent {
		t.Fatalf("clear: %d %s", res.Code, res.Body)
	}
	if got := defaults(); len(got) != 0 {
		t.Errorf("not cleared: %v", got)
	}
}
