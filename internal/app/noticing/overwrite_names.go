package noticing

import (
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 通知の Field の、コメント・作業時間の頭(C4c1-10)。
const (
	commentField = "comment:"
	worklogField = "worklog:"
)

// 上書きの {項目名} の表(決定 C4c1-9)。アイテムのプロパティ(組み込みを含む)は、いまのワークスペースの設定の名前。
var (
	itemFields = map[string]string{
		"title": "タイトル", item.SprintField: "スプリント", item.ReleaseField: "リリース", "place": "場所", "rank": "順位",
		"kind": "種類", "relations": "関連", "links": "リンク", "links_off": "リンク",
	}
	projectFields = map[string]string{
		"title": "タイトル", "status": "ステータス", "owner": "オーナー", "start": "開始日", "end": "終了日",
		"repos": "リポジトリ", "place": "場所", "rank": "順位",
	}
	itemRecords    = map[string]string{commentField: "コメント", worklogField: "作業時間"}
	projectRecords = map[string]string{commentField: "コメント"}

	workspaceFields   = map[string]string{"name": "ワークスペース名", "prefix": "接頭辞", "initial_status": "初期ステータス", "initial_priority": "初期優先度"}
	workspacePrefixes = []struct{ prefix, name string }{
		{"workflow/", "ワークフロー"}, {"category_default/", "既定のステータス"}, {"template_default/", "既定のテンプレート"},
	}
	valueParts = map[string]string{
		"name": "名前", "color": "色", "rank": "並び順", "category": "分類", "type": "種類", "kinds": "使う種類", "hidden": "表示",
	}
	families = []family{
		{head: "statuses", depth: 2, parts: valueParts, other: "設定"},
		{head: "priorities", depth: 2, parts: valueParts, other: "設定"},
		{head: "systems", depth: 2, parts: valueParts, other: "設定"},
		{head: "properties", depth: 2, parts: valueParts, other: "設定"},
		{head: "choices", depth: 3, parts: valueParts, other: "設定"},
		{head: "templates", depth: 2, quote: "テンプレート"},
		{head: "rules", depth: 2, quote: "自動化"},
		{head: "sprints", depth: 2, parts: map[string]string{"name": "名前", "state": "状態", "start": "開始日", "end": "終了日", "goal": "目標"}},
		{head: "releases", depth: 2, parts: map[string]string{"name": "名前", "state": "状態", "date": "日付", "description": "説明"}},
	}
)

// workspaceSetting は、表にないワークスペースのログの項目の呼び方。
const workspaceSetting = "ワークスペースの設定"

// family は、名前を持つワークスペースの設定(値・プロパティ・選択肢・テンプレート・ルール・スプリント・リリース)の項目の呼び方。
type family struct {
	head  string
	depth int               // 名前を持つものを指す f の段数(頭を含む。choices/<プロパティ>/<ID> は 3)
	quote string            // 中の項目をまとめて「{quote}「{名前}」」と呼ぶもの
	parts map[string]string // 「{名前} の{部分}」の部分
	other string            // parts にない部分。空なら workspaceSetting
}

// name は、ワークスペースのログ log の項目 parts(f を / で分けたもの)の呼び方。この family の項目でなければ false。
func (fm family) name(log []change.Record, parts []string) (string, bool) {
	if len(parts) <= fm.depth || parts[0] != fm.head {
		return "", false
	}
	name := settingName(log, strings.Join(parts[:fm.depth], "/"))
	if fm.quote != "" {
		return fm.quote + "「" + name + "」", true
	}
	part, ok := fm.parts[parts[fm.depth]]
	if !ok {
		part = fm.other
	}
	if len(parts) != fm.depth+1 || part == "" {
		return workspaceSetting, true
	}
	return name + " の" + part, true
}

// settingName は、ワークスペースの設定 f(statuses/<ID> など)のいまの名前。削除済みなら削除の前の名前、
// 引けなければ内部 ID(f の最後の段)。
func settingName(log []change.Record, f string) string {
	if name := oneLine(workspace.NameOfDeleted(log, f)); name != "" {
		return name
	}
	return f[strings.LastIndex(f, "/")+1:]
}

// fieldName は、エンティティ ref の項目 f(通知の Field)の {項目名}(決定 C4c1-9)。表にない項目は、アイテム・
// プロジェクト・メンバーの一覧では false、ワークスペースでは「ワークスペースの設定」。読み取りの失敗(ないもの以外)はエラー。
func fieldName(r port.Reader, b batch, ref change.Ref, f string) (name string, ok bool, err error) {
	switch ref.Kind {
	case change.KindItem:
		if name, ok := named(f, itemFields, itemRecords); ok {
			return name, true, nil
		}
		return propertyName(r, b, ref, f)
	case change.KindProject:
		name, ok := named(f, projectFields, projectRecords)
		return name, ok, nil
	case change.KindMembers:
		if f == "members" {
			return "メンバー", true, nil
		}
		login, ok := strings.CutPrefix(f, "name/")
		return login + " の表示名", ok, nil
	case change.KindWorkspace:
		log, err := logOf(r, b, ref)
		if err != nil {
			return "", false, err
		}
		return workspaceFieldName(r, b, log, f)
	}
	return "", false, nil
}

// named は、表 fields の f か、頭 records(コメント・作業時間)で始まる f の呼び方。
func named(f string, fields, records map[string]string) (string, bool) {
	if name, ok := fields[f]; ok {
		return name, true
	}
	for prefix, name := range records {
		if strings.HasPrefix(f, prefix) {
			return name, true
		}
	}
	return "", false
}

// propertyName は、アイテム ref のプロパティの項目 f の、アイテムのいまのワークスペースの設定の名前。削除済みの
// 追加プロパティは削除の前の名前(引けなければ内部 ID)。プロパティの項目でなければ false。
func propertyName(r port.Reader, b batch, ref change.Ref, f string) (string, bool, error) {
	log, err := logOf(r, b, ref)
	if err != nil {
		return "", false, err
	}
	it, ok := item.FromLog(log)
	if !ok {
		return "", false, nil
	}
	st, err := r.Workspaces().Settings(it.Place.Workspace)
	if err = notFoundIsSkip(err); err != nil {
		return "", false, err
	}
	for _, p := range st.Properties {
		if item.Field(p) == f {
			return oneLine(p.Name), true, nil
		}
	}
	id, ok := strings.CutPrefix(f, item.FieldKey(""))
	if !ok {
		return "", false, nil
	}
	wsLog, err := logOf(r, b, change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
	if err != nil {
		return "", false, err
	}
	return settingName(wsLog, "properties/"+id), true, nil
}

// workspaceFieldName は、ワークスペースのログ log の項目 f の呼び方。表になければ「ワークスペースの設定」。
func workspaceFieldName(r port.Reader, b batch, log []change.Record, f string) (string, bool, error) {
	if name, ok := workspaceFields[f]; ok {
		return name, true, nil
	}
	for _, p := range workspacePrefixes {
		if strings.HasPrefix(f, p.prefix) {
			return p.name, true, nil
		}
	}
	if id, ok := projectRuleOf(f); ok {
		e, found, err := entityOf(r, b, change.Ref{Kind: change.KindProject, ID: id})
		if !found {
			e.text = string(id)
		}
		return e.text + " のルール", err == nil, err
	}
	parts := strings.Split(f, "/")
	for _, fm := range families {
		if name, ok := fm.name(log, parts); ok {
			return name, true, nil
		}
	}
	return workspaceSetting, true, nil
}

// projectRuleOf は、f がプロジェクト別のルールの項目(project_rules/<プロジェクトの内部 ID>/…)なら、その内部 ID。
func projectRuleOf(f string) (change.ID, bool) {
	parts := strings.SplitN(f, "/", 3)
	if len(parts) < 3 || parts[0] != "project_rules" || parts[1] == "" {
		return "", false
	}
	return change.ID(parts[1]), true
}
