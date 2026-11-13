package workspace_test

import (
	"slices"
	"strings"
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

// テンプレートの保存で、削除済みの子・Step のうち変更のないものは除き、変更のあるものだけを取り消す(決定 R5)。
// テンプレートの削除の行だけで覆われる子・Step は、テンプレートと一緒に戻るので除かない。
func TestRevivedTemplate(t *testing.T) {
	w := workspace.Workspace{ID: "01WS0000000000000000000000", Name: "Web", Prefix: "HH"}
	create, err := workspace.CreateWithSettings(w, workspace.DefaultSettings(seqIDs()))
	if err != nil {
		t.Fatal(err)
	}
	const tid, a, c, s change.ID = "01TPL000000000000000000001", "01CHILD00000000000000000A", "01CHILD00000000000000000C", "01STEP0000000000000000000S"
	tpl := workspace.Template{
		ID: tid, Name: "リリース", Kind: "Epic",
		Children: []workspace.TemplateChild{
			{ID: a, Kind: "Task", Title: "QA", Steps: []workspace.TemplateStep{{ID: s, Title: "確認", Assignee: workspace.StepParent}}},
			{ID: c, Kind: "Task", Title: "本番反映"},
		},
	}.Ranked()
	base := append([]change.Draft{create}, workspace.TemplateDrafts(workspace.Template{}, tpl)...)
	tf := workspace.TemplateField(tid)
	cf := workspace.TemplateField(tid, "children", string(c))
	sf := workspace.TemplateField(tid, "steps", string(s))
	// edit は、画面が送る、tpl を f で変えたもの(rank は送らない)。
	edit := func(f func(*workspace.Template)) workspace.Template {
		in := tpl
		in.Children = slices.Clone(in.Children)
		for i := range in.Children {
			in.Children[i].Rank = ""
			in.Children[i].Steps = slices.Clone(in.Children[i].Steps)
			for j := range in.Children[i].Steps {
				in.Children[i].Steps[j].Rank = ""
			}
		}
		f(&in)
		return in
	}
	same := edit(func(*workspace.Template) {})
	rename := edit(func(in *workspace.Template) { in.Name = "リリース 2" })
	cases := []struct {
		name        string
		deletes     []string
		in          workspace.Template
		wantShape   string
		wantTargets []string
	}{
		{"unchanged deleted child", []string{cf}, rename, "A[S]", []string{tf}},
		{"changed deleted child", []string{cf}, edit(func(in *workspace.Template) { in.Children[1].Title = "本番リリース" }), "A[S] C[]", []string{tf, cf}},
		{"unchanged deleted step", []string{sf}, same, "A[] C[]", []string{tf}},
		{"changed deleted step", []string{sf}, edit(func(in *workspace.Template) { in.Children[0].Steps[0].Title = "再確認" }), "A[S] C[]", []string{tf, sf}},
		{"deleted template", []string{tf}, rename, "A[S] C[]", []string{tf}},
		{"deleted child, then template", []string{cf, tf}, rename, "A[S]", []string{tf}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds := slices.Clone(base)
			for _, f := range tc.deletes {
				ds = append(ds, change.NewDelete(f))
			}
			got, targets := workspace.RevivedTemplate(rows(ds...), tc.in)
			if shape(got) != tc.wantShape || !slices.Equal(targets, tc.wantTargets) {
				t.Errorf("RevivedTemplate = %q, %v; want %q, %v", shape(got), targets, tc.wantShape, tc.wantTargets)
			}
		})
	}
}

// shape は、テンプレートの子と Step を、ID の末尾の 1 文字で「A[S] C[]」のように書いたもの。
func shape(t workspace.Template) string {
	var out []string
	for _, c := range t.Children {
		var ss []string
		for _, s := range c.Steps {
			ss = append(ss, string(s.ID[len(s.ID)-1:]))
		}
		out = append(out, string(c.ID[len(c.ID)-1:])+"["+strings.Join(ss, " ")+"]")
	}
	return strings.Join(out, " ")
}
