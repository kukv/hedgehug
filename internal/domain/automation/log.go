package automation

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// collect は、ログのすべてのルール(消したものを含む)と、消したルールの ID。
func collect(log []change.Record) (map[change.ID]*Rule, map[change.ID]bool) {
	deleted := map[change.ID]bool{}
	restored := change.Restored(log)
	for _, r := range log { // Resolve は削除の行を読み飛ばすので、生のログから集める
		if id, ok := strings.CutPrefix(r.F, prefix); ok && r.T == change.Delete && !restored(r) && !strings.Contains(id, "/") {
			deleted[change.ID(id)] = true
		}
	}
	rules := map[change.ID]*Rule{}
	for f, raw := range change.Resolve(log, change.Options{}).Values {
		parts := strings.Split(f, "/")
		if len(parts) != 3 || parts[0]+"/" != prefix {
			continue
		}
		id := change.ID(parts[1])
		r, ok := rules[id]
		if !ok {
			r = &Rule{ID: id}
			rules[id] = r
		}
		set(r, parts[2], raw)
	}
	return rules, deleted
}

func set(r *Rule, field string, raw json.RawMessage) {
	switch field {
	case "name":
		_ = json.Unmarshal(raw, &r.Name)
	case "trigger":
		_ = json.Unmarshal(raw, &r.Trigger)
	case "trigger_status":
		_ = json.Unmarshal(raw, &r.TriggerStatus)
	case "all_merged":
		_ = json.Unmarshal(raw, &r.AllMerged)
	case "action":
		_ = json.Unmarshal(raw, &r.Action)
	case "value":
		_ = json.Unmarshal(raw, &r.Value)
	case "enabled":
		_ = json.Unmarshal(raw, &r.Enabled)
	case "scope":
		_ = json.Unmarshal(raw, &r.Scope)
	}
}

// FromLog は、ワークスペースのログのルール。消したもの・名前のないものを除き、内部 ID の順(B4-3)。
func FromLog(log []change.Record) []Rule {
	rules, deleted := collect(log)
	var out []Rule
	for id, r := range rules {
		if r.Name != "" && !deleted[id] {
			out = append(out, *r)
		}
	}
	slices.SortFunc(out, func(a, b Rule) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return out
}

// Names は、消したものを含むすべてのルールの名前(B4-16)。
func Names(log []change.Record) map[change.ID]string {
	rules, _ := collect(log)
	out := make(map[change.ID]string, len(rules))
	for id, r := range rules {
		out[id] = r.Name
	}
	return out
}

// Drafts は、old から r への行。old がゼロ値なら作成で、すべての項目を書く。ほかは変わった項目だけ。
// scope は作成のときだけ、専用ルールのときだけ書く(編集で Scope は変えない。B5a-10)。
func Drafts(old, r Rule) []change.Draft {
	fresh := old.ID == ""
	fields := []struct {
		name     string
		from, to any
	}{
		{"name", old.Name, r.Name},
		{"trigger", old.Trigger, r.Trigger},
		{"trigger_status", old.TriggerStatus, r.TriggerStatus},
		{"all_merged", old.AllMerged, r.AllMerged},
		{"action", old.Action, r.Action},
		{"value", old.Value, r.Value},
		{"enabled", old.Enabled, r.Enabled},
	}
	var ds []change.Draft
	if fresh && r.Scope != "" {
		ds = append(ds, change.NewSet(Field(r.ID, "scope"), nil, r.Scope))
	}
	for _, f := range fields {
		switch {
		case fresh:
			ds = append(ds, change.NewSet(Field(r.ID, f.name), nil, f.to))
		case f.from != f.to:
			ds = append(ds, change.NewSet(Field(r.ID, f.name), f.from, f.to))
		}
	}
	return ds
}

// DeleteDraft は、ルール id の削除の行。
func DeleteDraft(id change.ID) change.Draft { return change.NewDelete(prefix + string(id)) }
