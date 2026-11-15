package repairing

import (
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// entry は、重なりを数える範囲の中の 1 つ。id は内部 ID、f は値を持つ項目、v は前後の空白を除いた値。
type entry struct {
	id   change.ID
	f, v string
}

// scope は、重なりを数える範囲 1 つ(決定 C5c1-2)。kind は件名の {種類}(C5c1-7)、renames は比べ方と新しい値の決め方。
type scope struct {
	kind    string
	renames func([]repair.Named, repair.Facts) []repair.Rename
	entries []entry
}

// exactNames は、完全一致で比べる名前の直し。
func exactNames(all []repair.Named, f repair.Facts) []repair.Rename {
	return repair.NameRenames(all, f, false)
}

// foldNames は、大文字と小文字を区別せずに比べる名前の直し(値の名前。C5c1-2)。
func foldNames(all []repair.Named, f repair.Facts) []repair.Rename {
	return repair.NameRenames(all, f, true)
}

// settingsRenames は、ワークスペース ws の設定(プロパティの名前・キー、値、テンプレート、ルール、
// スプリント・リリース)の、重なった名前・キーの直し。log は ws のログ。順は C5c1-6 のとおり。
// どれもワークスペースのログに書く。名前を決めた行が見つからないものは数えない。
func settingsRenames(ws workspace.ID, log []change.Record, facts repair.Facts) []renameFix {
	ref := change.Ref{Kind: change.KindWorkspace, ID: ws}
	var out []renameFix
	for _, sc := range settingsScopes(log) {
		var all []repair.Named
		for _, e := range sc.entries {
			row, ok := repair.NamingRow(log, e.f)
			if !ok {
				continue
			}
			// 設定のものは同じログの中にあるので、Ref は内部 ID だけで見分ける。
			all = append(all, repair.Named{Ref: change.Ref{ID: e.id}, Field: e.f, Value: e.v, Naming: row})
		}
		for _, n := range sc.renames(all, facts) {
			out = append(out, renameFix{ref: ref, kind: sc.kind, rename: n})
		}
	}
	return out
}

// settingsScopes は、ログ log の、重なりを数える範囲を、直す順に返す。プロパティの名前 → キー →
// 値(ステータス・優先度・関連システム・選択肢をプロパティの並び順)→ テンプレート → ルール → スプリント → リリース。
func settingsScopes(log []change.Record) []scope {
	s, ok := workspace.SettingsFromLog(log)
	if !ok {
		return nil
	}
	var names, keys []entry
	for _, p := range s.Properties {
		base := "properties/" + string(p.ID) + "/"
		names = append(names, entry{p.ID, base + "name", workspace.NormalizeName(p.Name)})
		if !p.Builtin {
			keys = append(keys, entry{p.ID, base + "key", p.Key})
		}
	}
	out := []scope{{"プロパティ", exactNames, names}, {"プロパティのキー", repair.KeyRenames, keys}}
	out = append(out, valueScopes(s)...)
	var templates []entry
	for _, t := range s.Templates {
		templates = append(templates, entry{t.ID, workspace.TemplateField(t.ID, "name"), workspace.NormalizeName(t.Name)})
	}
	return append(append(out, scope{"テンプレート", exactNames, templates}), logScopes(log)...)
}

// valueScopes は、値の一覧ごとの範囲。ステータス・優先度・関連システム、選択肢をプロパティの並び順。
func valueScopes(s workspace.Settings) []scope {
	lists := []struct {
		kind string
		l    workspace.List
	}{
		{"ステータス", workspace.List{Kind: workspace.ListStatuses}},
		{"優先度", workspace.List{Kind: workspace.ListPriorities}},
		{"関連システム", workspace.List{Kind: workspace.ListSystems}},
	}
	for _, p := range s.Properties {
		lists = append(lists, struct {
			kind string
			l    workspace.List
		}{"選択肢", workspace.List{Kind: workspace.ListChoices, Property: p.ID}})
	}
	var out []scope
	for _, x := range lists {
		vs, ok := s.Values(x.l)
		if !ok {
			continue
		}
		var es []entry
		for _, v := range vs {
			es = append(es, entry{v.ID, x.l.Field(v.ID, "name"), workspace.NormalizeName(v.Name)})
		}
		out = append(out, scope{x.kind, foldNames, es})
	}
	return out
}

// logScopes は、ルール(専用ルール・無効を含む)、スプリント・リリース(完了・アーカイブを含む)の範囲。
func logScopes(log []change.Record) []scope {
	var rules, sprints, releases []entry
	for _, r := range automation.FromLog(log) {
		rules = append(rules, entry{r.ID, automation.Field(r.ID, "name"), workspace.NormalizeName(r.Name)})
	}
	p := planning.FromLog(log)
	for _, s := range p.LiveSprints() {
		sprints = append(sprints, entry{s.ID, "sprints/" + string(s.ID) + "/name", planning.NormalizeName(s.Name)})
	}
	for _, r := range p.LiveReleases() {
		releases = append(releases, entry{r.ID, "releases/" + string(r.ID) + "/name", planning.NormalizeName(r.Name)})
	}
	return []scope{{"自動化", exactNames, rules}, {"スプリント", exactNames, sprints}, {"リリース", exactNames, releases}}
}
