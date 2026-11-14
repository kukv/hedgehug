package inbox

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// slotState は、上書きの通知の戻す相手の状態(C4c2-4)。
type slotState int

const (
	slotNone  slotState = iota // 本文だけ(値もボタンも出さない)
	slotShown                  // 値だけ出す(ボタンを出さない)
	slotLive                   // 戻せる相手がある(有効か無効かは reason と、いまの値と Kept の比べで決める)
)

// slot は、上書きの通知の戻す相手(アイテムかプロジェクトの 1 項目)。
type slot struct {
	display string          // HH-1・HH-P1
	label   string          // 項目名
	summary string          // アクティビティの文(C4c2-8)
	now     json.RawMessage // いまの値(ログに書く形)
	// mine・kept・current は、あなたの値・残った値・いまの値の表示(C4c2-5)
	mine, kept, current string
	reason              string // 戻す先の値が使えない理由(C4c2-6)。空なら使える
	// drafts は、戻す保存の行(C4c2-8)。いまの値が Mine と同じなら行はない
	drafts func(tx port.Tx) ([]change.Draft, error)
	// write は、drafts の行を対象のアイテムかプロジェクトに書く
	write func(tx port.Tx, ds []change.Draft) error
}

// slotOf は、上書きの通知 n の戻す相手を、通知の対象と Field の組で決める(C4c2-3)。閲覧のみは見ない。
func slotOf(r port.Reader, n port.NoticeRow, now time.Time) (slot, slotState, error) {
	if n.Kind != port.NoticeOverwrite {
		return slot{}, slotNone, nil
	}
	switch {
	case n.Item != "" && n.Project == "":
		return itemSlot(r, n, now)
	case n.Project != "" && !strings.HasPrefix(n.Field, "project_rules/"):
		return projectSlot(r, n, now)
	}
	return slot{}, slotNone, nil
}

// fieldKind は、アイテムの値を出す項目の種類。
type fieldKind int

const (
	fieldTitle fieldKind = iota
	fieldSprint
	fieldRelease
	fieldProperty
)

// itemField は、アイテムの値を出す項目。prop はプロパティのときだけ使う。
type itemField struct {
	kind  fieldKind
	label string
	prop  workspace.Property
}

// fieldOf は、ログの項目名 f を、いまのワークスペースの設定 st で引く。値を出す項目でなければ ok が false。
func fieldOf(st workspace.Settings, f string) (itemField, bool) {
	switch f {
	case "title":
		return itemField{kind: fieldTitle, label: "タイトル"}, true
	case item.SprintField:
		return itemField{kind: fieldSprint, label: "スプリント"}, true
	case item.ReleaseField:
		return itemField{kind: fieldRelease, label: "リリース"}, true
	}
	for _, p := range st.Properties {
		if item.Field(p) == f {
			return itemField{kind: fieldProperty, label: p.Name, prop: p}, true
		}
	}
	return itemField{}, false
}

// wsView は、ワークスペースの設定とログ(値の表示と検査に使う)。
type wsView struct {
	ws  workspace.Workspace
	st  workspace.Settings
	log []change.Record
}

func viewOf(r port.Reader, id workspace.ID) (wsView, error) {
	ws, err := r.Workspaces().Get(id)
	if err != nil {
		return wsView{}, err
	}
	st, err := r.Workspaces().Settings(id)
	if err != nil {
		return wsView{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: id})
	return wsView{ws: ws, st: st, log: log}, err
}

// itemSlot は、アイテムの項目の戻す相手。Store にないか、項目がいまのワークスペースにないなら本文だけ。
func itemSlot(r port.Reader, n port.NoticeRow, now time.Time) (slot, slotState, error) {
	it, err := r.Items().Get(n.Item)
	if errors.Is(err, port.ErrNotFound) {
		return slot{}, slotNone, nil
	}
	if err != nil {
		return slot{}, slotNone, err
	}
	cur, err := viewOf(r, it.Place.Workspace)
	if err != nil {
		return slot{}, slotNone, err
	}
	f, ok := fieldOf(cur.st, n.Field)
	if !ok {
		return slot{}, slotNone, nil
	}
	from, err := noticeView(r, n, cur)
	if err != nil {
		return slot{}, slotNone, err
	}
	ms, err := r.Members().List()
	if err != nil {
		return slot{}, slotNone, err
	}
	show := func(w wsView, v json.RawMessage) string { return showItem(f, w, v, memberNames(ms), now) }
	s := slot{display: it.DisplayID(cur.ws.Prefix), label: f.label, now: itemValue(it, f)}
	s.summary = s.display + " の" + s.label + "を上書き前の値に戻す"
	s.mine, s.kept, s.current = show(from, n.Mine), show(from, n.Kept), show(cur, s.now)
	s.drafts = itemDrafts(it, f, n.Field, s.now, n.Mine)
	s.write = func(tx port.Tx, ds []change.Draft) error { return tx.WriteItems().Append(it.ID, ds...) }
	_, err = port.LiveWorkspace(r, it.Place.Workspace)
	switch {
	case it.Deleted != nil || errors.Is(err, port.ErrNotFound):
		return s, slotShown, nil
	case err != nil:
		return slot{}, slotNone, err
	}
	s.reason = itemReason(it, f, cur, n.Mine, ms)
	return s, slotLive, nil
}

// projectLabels は、プロジェクトの値を出す項目と項目名(C4c2-3)。
var projectLabels = map[string]string{"title": "タイトル", "status": "ステータス", "owner": "オーナー", "start": "開始日", "end": "終了日"}

// projectSlot は、プロジェクトの項目の戻す相手。Store にないか、値を出す項目でなければ本文だけ。
func projectSlot(r port.Reader, n port.NoticeRow, now time.Time) (slot, slotState, error) {
	label, ok := projectLabels[n.Field]
	if !ok {
		return slot{}, slotNone, nil
	}
	p, err := r.Projects().Get(n.Project)
	if errors.Is(err, port.ErrNotFound) {
		return slot{}, slotNone, nil
	}
	if err != nil {
		return slot{}, slotNone, err
	}
	ws, err := r.Workspaces().Get(p.Place.Workspace)
	if err != nil {
		return slot{}, slotNone, err
	}
	ms, err := r.Members().List()
	if err != nil {
		return slot{}, slotNone, err
	}
	show := func(v json.RawMessage) string {
		if n.Field == "title" {
			return oneLine(textOf(v))
		}
		return showProject(n.Field, v, memberNames(ms), now)
	}
	cur, mine := projectValue(p, n.Field), textOf(n.Mine)
	s := slot{display: p.DisplayID(ws.Prefix), label: label, now: change.Value(cur)}
	s.summary = oneLine(p.Title) + " の" + label + "を上書き前の値に戻す"
	s.mine, s.kept, s.current = show(n.Mine), show(n.Kept), show(s.now)
	s.drafts = func(port.Tx) ([]change.Draft, error) {
		if cur == mine {
			return nil, nil
		}
		return []change.Draft{change.NewSet(n.Field, cur, mine)}, nil
	}
	s.write = func(tx port.Tx, ds []change.Draft) error { return tx.WriteProjects().Append(p.ID, ds...) }
	_, err = port.LiveWorkspace(r, p.Place.Workspace)
	switch {
	case p.Deleted != nil || errors.Is(err, port.ErrNotFound):
		return s, slotShown, nil
	case err != nil:
		return slot{}, slotNone, err
	}
	s.reason = projectReason(p, n.Field, mine, ms)
	return s, slotLive, nil
}

// noticeView は、X と Y を引く通知のワークスペース(C4c2-5)。いまのワークスペースと同じか、Store にないなら cur。
func noticeView(r port.Reader, n port.NoticeRow, cur wsView) (wsView, error) {
	if n.Workspace == "" || n.Workspace == cur.ws.ID {
		return cur, nil
	}
	w, err := viewOf(r, n.Workspace)
	if errors.Is(err, port.ErrNotFound) {
		return cur, nil
	}
	return w, err
}

// showItem は、アイテムの項目 f の値 v の表示。w は値の名前を引くワークスペース。
func showItem(f itemField, w wsView, v json.RawMessage, names item.Names, now time.Time) string {
	switch f.kind {
	case fieldTitle:
		return oneLine(textOf(v))
	case fieldSprint:
		return showPlan(v, w.log, "sprints/")
	case fieldRelease:
		return showPlan(v, w.log, "releases/")
	}
	return showProperty(f.prop, v, w.st, w.log, names, now)
}

// itemValue は、アイテムの項目 f のいまの値(ログに書く形)。
func itemValue(it item.Item, f itemField) json.RawMessage {
	switch f.kind {
	case fieldTitle:
		return change.Value(it.Title)
	case fieldSprint:
		return refValue(it.Sprint)
	case fieldRelease:
		return refValue(it.Release)
	}
	return item.Current(it, f.prop)
}

// projectValue は、プロジェクトの項目 f のいまの値。空は ""(SetOwner・SetDate と同じ形)。
func projectValue(p project.Project, f string) string {
	switch f {
	case "title":
		return p.Title
	case "status":
		return string(p.Status)
	case "owner":
		return p.Owner
	case "start":
		return p.Start
	}
	return p.End
}

// isMember は、ログイン名がメンバーにいるか(大文字と小文字を区別しない)。
func isMember(ms []member.Member, login string) bool {
	for _, m := range ms {
		if strings.EqualFold(m.Login, login) {
			return true
		}
	}
	return false
}

// refValue は、参照の値。空は null。
func refValue(id change.ID) json.RawMessage {
	if id == "" {
		return json.RawMessage("null")
	}
	return change.Value(id)
}

// textOf は、文字列の値を読む。null や文字列でない値は空。
func textOf(v json.RawMessage) string {
	var s string
	_ = json.Unmarshal(v, &s)
	return s
}

// oneLine は、空白(改行を含む)を 1 つにつめた 1 行の文字列。
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
