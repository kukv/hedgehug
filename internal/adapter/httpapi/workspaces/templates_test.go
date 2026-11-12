package workspaces_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type templatesBody struct {
	Settings struct {
		Statuses []struct {
			ID string `json:"id"`
		} `json:"statuses"`
		Workflow         map[string][]string `json:"workflow"`
		Templates        []json.RawMessage   `json:"templates"`
		TemplateDefaults map[string]string   `json:"templateDefaults"`
	} `json:"settings"`
}

func TestWorkflowAndTemplatesAPI(t *testing.T) {
	api := newAPI(t)
	ws := create(t, api, "Web", "HH")
	var d templatesBody
	_ = json.Unmarshal(api.Get(base+"/"+ws).Body.Bytes(), &d)
	todo, doing := d.Settings.Statuses[0].ID, d.Settings.Statuses[1].ID
	if res := api.Put(base+"/"+ws+"/workflow/"+todo+"/"+doing, map[string]any{"allowed": true}); res.Code != http.StatusNoContent {
		t.Fatalf("transition: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+ws+"/workflow/"+todo+"/"+todo, map[string]any{"allowed": true}); res.Code != http.StatusNotFound {
		t.Errorf("self: %d", res.Code)
	}
	in := map[string]any{
		"name": "調査", "kind": "Task", "title": "調査: ", "description": "", "values": map[string]any{},
		"children": []any{}, "steps": []any{map[string]any{"id": "", "title": "再現", "assignee": "@none"}},
	}
	res := api.Post(base+"/"+ws+"/templates", in)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	tpl := idOf(t, res.Body.Bytes())
	if res := api.Post(base+"/"+ws+"/templates", in); res.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(res.Body.String(), "同じ名前のテンプレートがあります") {
		t.Errorf("duplicate: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+ws+"/template-defaults/Task", map[string]any{"template": tpl}); res.Code != http.StatusNoContent {
		t.Fatalf("default: %d %s", res.Code, res.Body)
	}
	_ = json.Unmarshal(api.Get(base+"/"+ws).Body.Bytes(), &d)
	if len(d.Settings.Workflow[todo]) != 1 || len(d.Settings.Templates) != 1 || d.Settings.TemplateDefaults["Task"] != tpl {
		t.Errorf("settings = %+v", d.Settings)
	}
	in["name"] = "調査 2"
	if res := api.Put(base+"/"+ws+"/templates/"+tpl, in); res.Code != http.StatusNoContent {
		t.Errorf("update: %d %s", res.Code, res.Body)
	}
	if res := api.Delete(base + "/" + ws + "/templates/" + tpl); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/nope/save-as-template",
		map[string]any{"name": "x", "title": "x", "structure": true}); res.Code != http.StatusNotFound {
		t.Errorf("save-as-template unknown item: %d %s", res.Code, res.Body)
	}
}
