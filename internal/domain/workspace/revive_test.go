package workspace_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestCovering(t *testing.T) {
	// rows の行 i の op は 01REC<i>。行 3 の delete statuses/B を、行 4 の restore で取り消す。
	log := rows(
		change.NewDelete("statuses/A"),
		change.NewDelete("templates/T"),
		change.NewDelete("templates/T/children/C"),
		change.NewDelete("statuses/B"),
		change.NewRestore("statuses/B", "01REC000000000000000000003"),
	)
	fs := func(rs []change.Record) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.F)
		}
		return out
	}
	cases := []struct {
		f    string
		want []string
	}{
		{"statuses/A", []string{"statuses/A"}},
		{"statuses/A/color", []string{"statuses/A"}},
		{"templates/T/children/C", []string{"templates/T", "templates/T/children/C"}},
		{"statuses/B", nil},
		{"statuses/AB", nil},
	}
	for _, c := range cases {
		if got := fs(workspace.Covering(log, c.f)); !slices.Equal(got, c.want) {
			t.Errorf("Covering(%q) = %v, want %v", c.f, got, c.want)
		}
	}
}

func TestSettingsReviving(t *testing.T) {
	w := workspace.Workspace{ID: "01WS0000000000000000000000", Name: "Web", Prefix: "HH"}
	create, err := workspace.CreateWithSettings(w, workspace.DefaultSettings(seqIDs()))
	if err != nil {
		t.Fatal(err)
	}
	review := workspace.Value{ID: "01STATUS00000000000000000R", Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	l := workspace.ParseList("statuses")
	drafts := append([]change.Draft{create}, workspace.ValueDrafts(l, review)...)
	log := rows(append(drafts, change.NewDelete(l.Ref(review.ID)))...)
	hasReview := func(vs []workspace.Value) bool {
		return slices.ContainsFunc(vs, func(v workspace.Value) bool { return v.ID == review.ID })
	}
	if got, ok := workspace.SettingsReviving(log, l.Ref(review.ID)); !ok || !hasReview(got.Statuses) {
		t.Errorf("SettingsReviving: statuses = %v, %v", names(got.Statuses), ok)
	}
	if now, _ := workspace.SettingsFromLog(log); hasReview(now.Statuses) {
		t.Errorf("SettingsFromLog: statuses = %v", names(now.Statuses))
	}
}

func TestCheckRevivedName(t *testing.T) {
	const live, revived, other change.ID = "01LIVE00000000000000000001", "01REVIVED00000000000000001", "01OTHER0000000000000000001"
	choices := workspace.List{Kind: workspace.ListChoices, Property: "01PROP0000000000000000000A"}
	st := workspace.Settings{
		Statuses: []workspace.Value{{ID: live, Name: "Review"}, {ID: revived, Name: "review"}, {ID: other, Name: "Done"}},
		Properties: []workspace.Property{
			{ID: choices.Property, Name: "顧客", Type: workspace.TypeSingle, Choices: []workspace.Value{{ID: live, Name: "A社"}, {ID: revived, Name: "A社"}}},
			{ID: revived, Name: "顧客"},
			{ID: other, Name: "部署"},
		},
	}
	cases := []struct {
		f    string
		want string
	}{
		{"statuses/" + string(revived), "同じ名前の値があります"},
		{"statuses/" + string(other), ""},
		{choices.Ref(revived), "同じ名前の値があります"},
		{"properties/" + string(revived), "同じ名前のプロパティがあります"},
		{"properties/" + string(other), ""},
		{"priorities/" + string(revived), ""}, // 見つからない
		{"templates/" + string(revived), ""},
	}
	for _, c := range cases {
		if got := refusedWith(workspace.CheckRevivedName(st, c.f)); got != c.want {
			t.Errorf("CheckRevivedName(%q) = %q, want %q", c.f, got, c.want)
		}
	}
}
