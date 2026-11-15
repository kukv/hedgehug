package noticing

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// restoreText は、取り消した削除 d の通知の本文(決定 C5b2-9)。d がワークスペースの値の削除で、修復の直前のログ before で
// 設定の項目が指している(既定にした人を添える)か、分類の最後の値なら 06 の本文、ほかは重なりの本文(変更した人を添える)。
func restoreText(r port.Reader, ref change.Ref, before []change.Record, d change.Record, target string, sides repair.Facts) (string, error) {
	if ref.Kind == change.KindWorkspace {
		if reason, ok := repair.ReasonOf(before, d.F); ok {
			return reasonText(r, before, d.F, target, reason)
		}
	}
	who, err := memberName(r, repair.Changer(before, d, sides))
	if err != nil {
		return "", err
	}
	return withParticle(target, "は") + " " + who + " が同時に変更したため、削除を取り消しました", nil
}

// reasonText は、値 f の削除を、reason のために取り消した通知の本文。
func reasonText(r port.Reader, before []change.Record, f, target string, reason repair.RestoreReason) (string, error) {
	if reason.Field == "" {
		return withParticle(target, "は") + workspace.CategoryName(reason.Category) + "の最後のステータスのため、削除を取り消しました", nil
	}
	// ReasonOf が true を返したので、f は値の項目(statuses/<ID>・priorities/<ID>)。
	_, id, _ := strings.Cut(f, "/")
	who, err := memberName(r, setter(before, reason.Field, id))
	if err != nil {
		return "", err
	}
	return withParticle(target, "は") + " " + who + " が同時に既定にしたため、削除を取り消しました", nil
}

// setter は、log の項目 field を値 id にした人(人の set の行の最後の行の by)。なければ空。
func setter(log []change.Record, field, id string) string {
	rows := rowsWhere(log, func(c change.Record) bool {
		return c.T == change.Set && c.F == field && sameValue(c.To, change.Value(id))
	})
	return lastSaver(rows, "")
}

// workspaceRows は、i 番目が修復の保存なら、そのワークスペースのログの行ごとに、修復の直前のログ before で fn が
// 返す通知を集める。
func workspaceRows(b batch, i int, fn func(ws workspace.ID, before []change.Record, row change.Record) ([]port.Notice, error)) ([]port.Notice, error) {
	if !b.saves[i].Repair {
		return nil, nil
	}
	var out []port.Notice
	for _, e := range b.saves[i].Entities {
		if e.Ref.Kind != change.KindWorkspace {
			continue
		}
		before := b.beforeSave(e.Ref, i)
		for _, row := range e.Rows {
			ns, err := fn(e.Ref.ID, before, row)
			if err != nil {
				return nil, err
			}
			out = append(out, ns...)
		}
	}
	return out, nil
}

// valueHit は、削除された値を指すものの修復 1 つの通知の材料。
type valueHit struct {
	del  change.Record           // 値を覆う、取り消されていない最初の削除の行(by が {削除した人}、op が鍵の末尾)
	key  string                  // 鍵の、削除の op の前まで
	base port.Notice             // 開く先(決定 C5b2-13)
	text func(who string) string // {削除した人} の名前からの本文
}

// valueRefs は、i 番目の修復の保存が、削除された値を指すプロジェクト別のルール・ワークスペースの種類ごとの既定・
// ルールを直した修復 (10) なら、その通知(06「削除された値を指すプロジェクト別のルール」。決定 C5b2-10〜C5b2-13)。
// 外した値が修復の直前のログで削除されている行のうち、外したものを入れた人があなたのものを、プロジェクト別のルールは
// (プロジェクト, 区画, 値)、既定は種類、ルールはルールごとに 1 件にする。使わないステータスの行では作らない。
func valueRefs(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	seen := map[string]bool{}
	return workspaceRows(b, i, func(ws workspace.ID, before []change.Record, row change.Record) ([]port.Notice, error) {
		hs, err := valueHits(r, b, ws, before, row, me)
		if err != nil {
			return nil, err
		}
		var out []port.Notice
		for _, h := range hs {
			n := h.base
			if n.Source = h.key + string(h.del.Op); seen[n.Source] {
				continue
			}
			seen[n.Source] = true
			who, err := memberName(r, h.del.By)
			if err != nil {
				return nil, err
			}
			n.Text = h.text(who)
			out = append(out, n)
		}
		return out, nil
	})
}

// valueHits は、ワークスペース ws の修復の行 row が直した、削除された値を指すもののうち、あなたに知らせるもの。
func valueHits(r port.Reader, b batch, ws workspace.ID, before []change.Record, row change.Record, me string) ([]valueHit, error) {
	var h valueHit
	var ok bool
	switch {
	case strings.HasPrefix(row.F, "project_rules/"):
		return projectHits(r, b, ws, before, row, me)
	case strings.HasPrefix(row.F, "template_default/"):
		h, ok = defaultHit(ws, before, row, me)
	case strings.HasPrefix(row.F, "rules/"):
		h, ok = ruleHit(ws, before, row, me)
	}
	if !ok {
		return nil, nil
	}
	return []valueHit{h}, nil
}

// refSection は、プロジェクト別のルールの区画。
type refSection struct {
	key   string // 鍵の区画(workflow・templates・template_default/<種類>)
	label string // 本文の区画の名前
	list  string // 外した値の一覧(statuses・templates)
}

// sectionOf は、プロジェクト別のルールの項目の残り rest(project_rules/<P>/ のあと)の区画。行 row が値を外す行
// (ワークフローの制約・使うテンプレートの remove、種類ごとの既定の null への set)でなければ、使わないステータスと
// 外すルールも含めて false。
func sectionOf(rest string, row change.Record) (refSection, bool) {
	remove := row.T == change.Remove
	kind, isDefault := strings.CutPrefix(rest, "template_default/")
	switch {
	case strings.HasPrefix(rest, "workflow/") && remove:
		return refSection{"workflow", "ワークフローの制約", "statuses"}, true
	case rest == "templates" && remove:
		return refSection{"templates", "使うテンプレート", "templates"}, true
	case isDefault && row.T == change.Set && change.Absent(row.To):
		return refSection{rest, kind + "の既定のテンプレート", "templates"}, true
	}
	return refSection{}, false
}

// projectHits は、プロジェクト別のルールの行 row の通知の材料。外した値(ワークフローの行は、外した To と項目の From)の
// うち削除されているものごとに 1 つ。持ち主のプロジェクトが保存のあとでワークスペース ws になければ(修復 (9) の形)作らない。
func projectHits(r port.Reader, b batch, ws workspace.ID, before []change.Record, row change.Record, me string) ([]valueHit, error) {
	p, rest, _ := strings.Cut(strings.TrimPrefix(row.F, "project_rules/"), "/")
	sec, ok := sectionOf(rest, row)
	if !ok || !same(putter(before, row), me) {
		return nil, nil
	}
	ids := []string{idOf(row.From)}
	if from, ok := strings.CutPrefix(rest, "workflow/"); ok {
		ids = append(ids, from)
	}
	ref := change.Ref{Kind: change.KindProject, ID: project.ID(p)}
	log, err := logOf(r, b, ref)
	if err != nil {
		return nil, err
	}
	pr, ok := project.FromLog(log)
	if !ok || pr.Place.Workspace != ws {
		return nil, nil
	}
	e, ok, err := entityOf(r, b, ref)
	if !ok {
		return nil, err
	}
	head := "プロジェクト「" + oneLine(pr.Title) + "」の" + sec.label + "から、"
	var out []valueHit
	for _, id := range ids {
		f := sec.list + "/" + id
		del, ok := deletion(before, f)
		if id == "" || !ok {
			continue
		}
		name := oneLine(workspace.NameOfDeleted(before, f))
		out = append(out, valueHit{del, "repair:valueref:" + p + ":" + sec.key + ":", e.base, func(who string) string {
			return head + who + " が「" + name + "」を削除したため、「" + name + "」を外しました"
		}})
	}
	return out, nil
}

// defaultHit は、ワークスペースの種類ごとの既定を null に戻した行 row の通知の材料。宛先は、その既定を外した値にした人。
func defaultHit(ws workspace.ID, before []change.Record, row change.Record, me string) (valueHit, bool) {
	kind := strings.TrimPrefix(row.F, "template_default/")
	id := idOf(row.From)
	if row.T != change.Set || !change.Absent(row.To) || id == "" || !same(setter(before, row.F, id), me) {
		return valueHit{}, false
	}
	f := workspace.TemplateField(change.ID(id))
	del, ok := deletion(before, f)
	if !ok {
		return valueHit{}, false
	}
	name := oneLine(workspace.NameOfDeleted(before, f))
	return valueHit{del, "repair:valueref:" + string(ws) + ":" + row.F + ":", port.Notice{Workspace: ws, Section: "templates"}, func(who string) string {
		return kind + "の既定のテンプレートから、" + who + " が「" + name + "」を削除したため、「なし」に戻しました"
	}}, true
}

// ruleHit は、ルールを無効にした行 row(set rules/<R>/enabled の true → false)の通知の材料。値は、ルールが指す削除された
// ステータスのうち automation.Statuses の順で最初のもの。宛先は、そのルールの行を最後に保存した人。
func ruleHit(ws workspace.ID, before []change.Record, row change.Record, me string) (valueHit, bool) {
	id, ok := strings.CutSuffix(strings.TrimPrefix(row.F, "rules/"), "/enabled")
	off := row.T == change.Set && sameValue(row.From, change.Value(true)) && sameValue(row.To, change.Value(false))
	if !ok || !off || !same(ruleSaver(before, "rules/"+id), me) {
		return valueHit{}, false
	}
	rules := automation.FromLog(before)
	i := slices.IndexFunc(rules, func(x automation.Rule) bool { return x.ID == change.ID(id) })
	if i < 0 {
		return valueHit{}, false
	}
	for _, s := range automation.Statuses(rules[i]) {
		f := "statuses/" + string(s)
		if del, ok := deletion(before, f); ok {
			rule, name := oneLine(rules[i].Name), oneLine(workspace.NameOfDeleted(before, f))
			return valueHit{del, "repair:valueref:rule:" + id + ":", port.Notice{Workspace: ws, Rule: rules[i].ID, Section: "automation"}, func(who string) string {
				return "自動化「" + rule + "」は、" + who + " が「" + name + "」を削除したため、無効にしました"
			}}, true
		}
	}
	return valueHit{}, false
}

// deletion は、修復の直前のログ before で値の項目 f を覆う、取り消されていない最初の削除の行。なければ false。
func deletion(before []change.Record, f string) (change.Record, bool) {
	if ds := workspace.Covering(before, f); len(ds) > 0 {
		return ds[0], true
	}
	return change.Record{}, false
}

// idOf は、行の値 raw(JSON の文字列)の ID。文字列でなければ空。
func idOf(raw json.RawMessage) string {
	var id string
	_ = json.Unmarshal(raw, &id)
	return id
}

// finishedInitial は、i 番目の修復の保存が、完了・中止の分類にされた初期ステータス X を未着手の既定に戻したなら、その通知
// (06「完了・中止の分類にされた初期ステータス」。決定 C5b2-11〜C5b2-13)。X を初期ステータスにした人の set の最後の行が
// あなたのものなら 1 件。{変更した人} は X の分類の set の最後の行の人。
func finishedInitial(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	return workspaceRows(b, i, func(ws workspace.ID, before []change.Record, row change.Record) ([]port.Notice, error) {
		if row.T != change.Set || row.F != "initial_status" {
			return nil, nil
		}
		return initialNotice(r, ws, before, row, me)
	})
}

// initialNotice は、初期ステータスを戻した修復の行 row の通知。
func initialNotice(r port.Reader, ws workspace.ID, before []change.Record, row change.Record, me string) ([]port.Notice, error) {
	x := idOf(row.From)
	st, ok := workspace.SettingsFromLog(before)
	i := slices.IndexFunc(st.Statuses, func(v workspace.Value) bool { return string(v.ID) == x })
	if !ok || i < 0 || !st.Finished(st.Statuses[i].ID) {
		return nil, nil
	}
	set := lastSaved(rowsWhere(before, func(c change.Record) bool {
		return c.T == change.Set && c.F == "initial_status" && sameValue(c.To, change.Value(x))
	}), "")
	if set == nil || !same(set.By, me) {
		return nil, nil
	}
	category := "statuses/" + x + "/category"
	who, err := memberName(r, lastSaver(rowsWhere(before, func(c change.Record) bool {
		return c.T == change.Set && c.F == category
	}), ""))
	if err != nil {
		return nil, err
	}
	todo := oneLine(workspace.NameOfDeleted(before, "statuses/"+idOf(row.To)))
	text := withParticle("「"+oneLine(st.Statuses[i].Name)+"」", "は") + " " + who + " が" +
		workspace.CategoryName(st.Statuses[i].Category) + "の分類にしたため、初期ステータスを「" + todo + "」に戻しました"
	return []port.Notice{{Workspace: ws, Section: "statuses", Text: text, Source: "repair:initial:" + string(ws) + ":" + string(set.ID)}}, nil
}
