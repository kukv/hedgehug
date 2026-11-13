package workspace

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// 別の画面で削除された設定への保存は、変更を残して削除を取り消す(06「他の書き手との重なり」)。
// 下の関数は、その取り消しで使う、ワークスペースのログの読み方。

// Covering は、項目名 f を覆う削除の行(f が同じか、行の f + "/" が f の頭に当たるもの)のうち、
// 取り消されていないものを、ログの順に返す。テンプレートの子なら、子とテンプレートの両方の行。
func Covering(log []change.Record, f string) []change.Record {
	restored := change.Restored(log)
	var out []change.Record
	for _, r := range log {
		if r.T == change.Delete && !restored(r) && (r.F == f || strings.HasPrefix(f, r.F+"/")) {
			out = append(out, r)
		}
	}
	return out
}

// SettingsReviving は、fs のそれぞれを覆う削除の行がないものとして、ログから設定を作る。
// 取り消す前の設定を、削除済みのものの名前を含めて読むのに使う。log は変えない。
func SettingsReviving(log []change.Record, fs ...string) (Settings, bool) {
	var drop []change.ID
	for _, f := range fs {
		for _, r := range Covering(log, f) {
			drop = append(drop, r.ID)
		}
	}
	kept := make([]change.Record, 0, len(log))
	for _, r := range log {
		if !slices.Contains(drop, r.ID) {
			kept = append(kept, r)
		}
	}
	return SettingsFromLog(kept)
}

// RevivedTemplate は、テンプレートの保存(画面が送ったテンプレート in)で取り消すものを決める(決定 R5)。
// in の子・Step のうち、それ自身の削除の行があるもの(テンプレートの削除の行だけで覆われるものは、テンプレートと
// 一緒に戻るので数えない)を、削除の前の内容と比べ、行を書かないものを in から除く。除いた in と、取り消す
// 項目名の頭(テンプレート自身と、残した削除済みの子・Step)を返す。
func RevivedTemplate(log []change.Record, in Template) (Template, []string) {
	tf := TemplateField(in.ID)
	var gone []string
	for _, f := range sentFields(in) {
		if slices.ContainsFunc(Covering(log, f), func(r change.Record) bool { return r.F != tf }) {
			gone = append(gone, f)
		}
	}
	targets := []string{tf}
	if len(gone) == 0 {
		return in, targets
	}
	st, _ := SettingsReviving(log, append([]string{tf}, gone...)...)
	old, _ := st.Template(in.ID)
	ds := TemplateDrafts(old, NormalizeTemplate(st, MergeTemplate(old, in)).Ranked())
	drop := map[string]bool{}
	for _, f := range gone {
		if slices.ContainsFunc(ds, func(d change.Draft) bool { return strings.HasPrefix(d.F, f+"/") }) {
			targets = append(targets, f)
		} else {
			drop[f] = true
		}
	}
	return without(in, drop), targets
}

// sentFields は、in の子と Step(子の Step も)の項目名の頭。
func sentFields(in Template) []string {
	var fs []string
	for _, c := range in.Children {
		fs = append(fs, TemplateField(in.ID, "children", string(c.ID)))
		for _, s := range c.Steps {
			fs = append(fs, TemplateField(in.ID, "steps", string(s.ID)))
		}
	}
	for _, s := range in.Steps {
		fs = append(fs, TemplateField(in.ID, "steps", string(s.ID)))
	}
	return fs
}

// without は、t から、項目名の頭が drop にある子と Step を除いたもの(入力の配列は書き換えない)。
func without(t Template, drop map[string]bool) Template {
	steps := func(ss []TemplateStep) []TemplateStep {
		return slices.DeleteFunc(slices.Clone(ss), func(s TemplateStep) bool {
			return drop[TemplateField(t.ID, "steps", string(s.ID))]
		})
	}
	t.Children = slices.DeleteFunc(slices.Clone(t.Children), func(c TemplateChild) bool {
		return drop[TemplateField(t.ID, "children", string(c.ID))]
	})
	for i := range t.Children {
		t.Children[i].Steps = steps(t.Children[i].Steps)
	}
	t.Steps = steps(t.Steps)
	return t
}

// CheckRevivedName は、取り消した f(削除の行の f)のプロパティ・値の名前が、同じ一覧のほかのものと重ならないかを
// 確かめる(作成・名前の変更と同じ比べ方)。テンプレート・ルールなどほかのものと、見つからないものは nil。
func CheckRevivedName(st Settings, f string) error {
	parts := strings.Split(f, "/")
	if len(parts) == 2 && parts[0] == "properties" {
		p, ok := st.Property(change.ID(parts[1]))
		if !ok {
			return nil
		}
		return CheckPropertyName(st, p.ID, p.Name)
	}
	var l List
	switch {
	case len(parts) == 2 && slices.Contains(valueLists, parts[0]):
		l = List{Kind: ListKind(parts[0])}
	case len(parts) == 3 && parts[0] == string(ListChoices):
		l = List{Kind: ListChoices, Property: change.ID(parts[1])}
	default:
		return nil
	}
	vs, _ := st.Values(l)
	i := slices.IndexFunc(vs, func(v Value) bool { return v.ID == change.ID(parts[len(parts)-1]) })
	if i < 0 {
		return nil
	}
	return CheckValueName(vs, vs[i].ID, vs[i].Name)
}
