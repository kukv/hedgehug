package query_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func ptr(f float64) *float64 { return &f }

func TestParseDate(t *testing.T) {
	cases := map[string]string{
		"today": "2026-11-15", "+3d": "2026-11-18", "3d": "2026-11-18", "-7d": "2026-11-08",
		"+3D": "2026-11-18", "TODAY": "2026-11-15",
		"2026-12-01": "2026-12-01", "abc": "", "2026-13-01": "", "d": "", "99999999999d": "",
	}
	for in, want := range cases {
		got, ok := query.ParseDate(in, today)
		if ok != (want != "") || got != want {
			t.Errorf("ParseDate(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
}

func TestCompare(t *testing.T) {
	f := facts(t, func(f *query.Facts) {
		f.Item.Points, f.Item.Estimate = ptr(3), ptr(1.5)
		f.Item.Start, f.Item.End = "2026-11-10", "2026-11-14"
	})
	check(t, f, map[string]bool{
		"points>=3": true, "points>3": false, "points<=2": false, "points:3": true, "points:none": false,
		"estimate<=2": true, "estimate>2": false, "points:3,5": true, "points>=abc": false,
		"end<today": true, "end>=today": false, "start>-7d": true, "start<-7d": false,
		"end:abc": false, "end:2026-11-14": true, "end:none": false, "kind>story": true, "kind<story": true,
	})
	check(t, facts(t), map[string]bool{
		"points>=3": false, "points:none": true, "end<today": false, "start:none": true, "estimate:none": true,
	})
	check(t, facts(t, func(f *query.Facts) { f.Item.Points = ptr(2.5) }), map[string]bool{"points>=3": false})
}

func TestCompareCreatedUpdated(t *testing.T) {
	f := facts(t, func(f *query.Facts) {
		f.Created = time.Date(2026, 11, 15, 0, 30, 0, 0, time.Local)
		f.Updated = time.Date(2026, 11, 10, 23, 30, 0, 0, time.Local)
	})
	check(t, f, map[string]bool{
		"created:today": true, "created:2026-11-15": true, "created<today": false,
		"updated>-7d": true, "updated<-3d": true, "updated:today": false,
	})
}

func TestMatchNegationKeepsEmpty(t *testing.T) {
	f := facts(t, func(f *query.Facts) { f.Item.Assignee = "" })
	check(t, f, map[string]bool{"-assignee:@me": true, "-points>3": true, "-end:abc": true, "-has:end": true})
}

func TestSprintRelease(t *testing.T) {
	cur := facts(t, func(f *query.Facts) { f.SprintName, f.SprintCurrent, f.ReleaseName = "Sprint 3", true, "v2.0.0" })
	check(t, cur, map[string]bool{
		"sprint:current": true, "sprint:next": false, `sprint:"Sprint 3"`: true, "sprint:none": false,
		"sprint:x,current": true, "release:v2.0.0": true, "release:none": false, "release:v3": false,
	})
	next := facts(t, func(f *query.Facts) { f.SprintName, f.SprintNext = "Sprint 4", true })
	check(t, next, map[string]bool{"sprint:next": true, "sprint:current": false, "release:none": true})
	check(t, facts(t), map[string]bool{"sprint:none": true, "release:none": true, "sprint:current": false})
	// 否定は、値のないアイテムを一致する側に入れる。
	check(t, cur, map[string]bool{
		"-sprint:current": false, `-sprint:"Sprint 3"`: false, "-sprint:next": true, "-sprint:none": true,
		"-release:v2.0.0": false, "-release:v3": true, "-release:none": true,
	})
	check(t, facts(t), map[string]bool{"-sprint:current": true, "-sprint:none": false, "-release:v2.0.0": true, "-release:none": false})
}

func TestIsHas(t *testing.T) {
	f := facts(t, func(f *query.Facts) {
		f.Item.Blocked, f.Invalid, f.Overwritten, f.HasPR = true, true, true, true
		f.Item.End = "2026-11-14"
		f.SprintName, f.ReleaseName = "S", "R"
	})
	check(t, f, map[string]bool{
		"is:blocked": true, "is:overdue": true, "is:overwritten": true, "is:invalid": true,
		"is:deleted": false, "is:blocked,foo": true, "has:end": true, "has:sprint": true, "has:release": true,
		"has:pr": true, "has:start": false, "has:points": false, "has:assignee": true, "has:systems": true,
	})
	check(t, facts(t, func(f *query.Facts) { f.Item.End = today }), map[string]bool{"is:overdue": false, "is:blocked": false})
	check(t, facts(t, func(f *query.Facts) { f.Item.End, f.Item.Status = "2026-11-01", "s2" }), map[string]bool{"is:overdue": false})
	check(t, facts(t, func(f *query.Facts) { f.Item.End = "" }), map[string]bool{"is:overdue": false})
	// 中止(canceled)は終わったものなので、期限切れにしない。
	canceled := facts(t, func(f *query.Facts) {
		f.Settings.Statuses = append(f.Settings.Statuses, workspace.Value{ID: "s9", Name: "中止", Category: workspace.CategoryCanceled})
		f.Item.End, f.Item.Status = "2026-11-01", "s9"
	})
	check(t, canceled, map[string]bool{"is:overdue": false, "-is:overdue": true})
}

func TestIsAndHasUnknownKeepsNegation(t *testing.T) {
	f := facts(t)
	check(t, f, map[string]bool{"is:foo": false, "-is:foo": true, "has:foo": false, "-has:foo": true})
}

func TestMentions(t *testing.T) {
	f := facts(t, func(f *query.Facts) {
		f.Me = "suzuki"
		f.Description = "@tanaka お願い"
		f.Comments = []string{"ok", "cc @suzuki"}
	})
	check(t, f, map[string]bool{
		"mentions:tanaka": true, "mentions:@tanaka": true, "mentions:@me": true, "mentions:sato": false,
		"mentions:none": false,
	})
	check(t, facts(t, func(f *query.Facts) { f.Description, f.Comments = "なし", []string{"ok"} }),
		map[string]bool{"mentions:none": true, "mentions:@me": false})
	check(t, facts(t, func(f *query.Facts) { f.Description = "mail@example.com" }), map[string]bool{"mentions:none": false})
	check(t, facts(t, func(f *query.Facts) { f.Me, f.Description = "", "@x" }), map[string]bool{"mentions:@me": false})
	// キーワードは `@me` と `none` だけ。`me` はログイン名 me、`@none` はログイン名 none。
	check(t, f, map[string]bool{"mentions:me": false})
	check(t, facts(t, func(f *query.Facts) { f.Me, f.Description = "suzuki", "@me @none" }),
		map[string]bool{"mentions:me": true, "mentions:@me": false, "mentions:@none": true, "mentions:none": false})
}

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// customFacts は、種類ごとの追加プロパティ(キー sel・tags・num・due・who・ok・memo・link)を持つアイテム。
func customFacts(t *testing.T, mut ...func(*query.Facts)) query.Facts {
	t.Helper()
	choices := []workspace.Value{{ID: "c1", Name: "Alpha"}, {ID: "c2", Name: "Beta"}}
	prop := func(key string, typ workspace.PropertyType) workspace.Property {
		return workspace.Property{ID: change.ID("id-" + key), Key: key, Name: key, Type: typ, Choices: choices}
	}
	return facts(t, append([]func(*query.Facts){func(f *query.Facts) {
		f.Settings.Properties = []workspace.Property{
			{ID: "end", Key: "end", Builtin: true, Type: workspace.TypeDate},
			prop("sel", workspace.TypeSingle), prop("tags", workspace.TypeMulti), prop("num", workspace.TypeNumber),
			prop("due", workspace.TypeDate), prop("who", workspace.TypeUser), prop("ok", workspace.TypeCheck),
			prop("memo", workspace.TypeText), prop("link", workspace.TypeURL),
		}
		f.Item.Choices = map[change.ID][]change.ID{"id-sel": {"c1"}, "id-tags": {"c1", "c2"}}
		f.Item.Fields = map[change.ID]json.RawMessage{
			"id-num": raw(4), "id-due": raw("2026-11-20"), "id-who": raw("tanaka"), "id-ok": raw(true),
			"id-memo": raw("Hello World"), "id-link": raw("https://example.com/x"),
		}
	}}, mut...)...)
}

func checkCustom(t *testing.T, f query.Facts, cases map[string]bool) {
	t.Helper()
	for in, want := range cases {
		q, errs := query.Parse(in, func(string) bool { return true })
		if len(errs) > 0 {
			t.Fatalf("%q: %v", in, errs)
		}
		if got := query.Match(q, f, today).Match; got != want {
			t.Errorf("%q = %v, want %v", in, got, want)
		}
	}
}

func TestCustomProperties(t *testing.T) {
	checkCustom(t, customFacts(t), map[string]bool{
		"sel:alpha": true, "sel:beta": false, "sel:none": false, "sel:beta,alpha": true, "SEL:Alpha": true,
		"tags:beta": true, "tags:gamma": false, "tags:none": false,
		"num>=4": true, "num>4": false, "num:none": false, "num:abc": false,
		"due<2026-12-01": true, "due:2026-11-20": true, "due>today": true, "due:none": false,
		"who:@me": true, "who:tanaka": true, "who:suzuki": false, "who:none": false,
		"ok:true": true, "ok:false": false, "ok:x": false,
		"memo:world": true, "memo:none": false, "memo:zzz": false, "link:example": true,
		"has:sel": true, "has:num": true, "has:ok": true, "has:memo": true, "has:tags": true,
		"-sel:alpha": false, "-sel:beta": true,
	})
	empty := customFacts(t, func(f *query.Facts) { f.Item.Choices, f.Item.Fields = nil, nil })
	checkCustom(t, empty, map[string]bool{
		"sel:none": true, "tags:none": true, "num:none": true, "due:none": true, "who:none": true,
		"ok:false": true, "ok:true": false, "memo:none": true, "link:none": true,
		"has:sel": false, "has:num": false, "has:ok": false, "has:tags": false, "has:memo": false,
	})
	off := customFacts(t, func(f *query.Facts) { f.Item.Fields["id-ok"] = raw(false) })
	checkCustom(t, off, map[string]bool{"ok:false": true, "ok:true": false, "has:ok": false})
}

func TestCustomPropertyMissingKey(t *testing.T) {
	f := customFacts(t, func(f *query.Facts) { f.Settings.Properties = nil })
	checkCustom(t, f, map[string]bool{"sel:alpha": false, "-sel:alpha": true, "sel:none": false, "has:sel": false})
}

func TestBuiltinWinsOverCustomSpelling(t *testing.T) {
	f := customFacts(t, func(f *query.Facts) {
		f.Item.End = "2026-11-14"
		f.Settings.Properties = append(f.Settings.Properties,
			workspace.Property{ID: "id-end", Key: "end", Type: workspace.TypeText})
		f.Item.Fields["id-end"] = raw("zzz")
	})
	checkCustom(t, f, map[string]bool{"end<today": true, "end:zzz": false})
}
