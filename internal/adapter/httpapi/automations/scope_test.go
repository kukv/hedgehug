package automations_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/automations"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appautomations "github.com/kukv/hedgehug/internal/app/automations"
)

func TestRuleScope(t *testing.T) {
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	automations.Register(api, appautomations.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}, Clock: clock})
	url := wsBase + "/" + string(apptest.SampleWorkspace) + "/automation/rules"
	rule := map[string]any{
		"name": "着手で未割り当て", "trigger": "status", "triggerStatus": string(apptest.SampleStatusInProgress),
		"allMerged": false, "action": "assignee", "value": "@none", "scope": string(apptest.SampleProject),
	}
	res := api.Post(url, rule)
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"scope":"`+string(apptest.SampleProject)+`"`) {
		t.Fatalf("scoped: %d %s", res.Code, res.Body)
	}
	// 一覧は、持ち主のプロジェクトの表示 ID を添える(削除済みでないので scopeDeleted は出さない)。
	list := api.Get(wsBase + "/" + string(apptest.SampleWorkspace) + "/automation").Body.String()
	if !strings.Contains(list, `"scopeLabel":"HH-P1"`) || strings.Contains(list, `"scopeDeleted"`) {
		t.Errorf("list: %s", list)
	}
	rule["scope"] = "01NOPE00000000000000000000"
	if res := api.Post(url, rule); res.Code != http.StatusNotFound {
		t.Errorf("unknown scope: %d %s", res.Code, res.Body)
	}
}
