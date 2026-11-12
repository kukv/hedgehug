// Package item は、アイテム(Epic・Story・Task・Bug・Step)を持つ。
package item

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ID はアイテムの内部 ID。
type ID = change.ID

// Kind はアイテムの種類。
type Kind string

const (
	Epic  Kind = "Epic"
	Story Kind = "Story"
	Task  Kind = "Task"
	Bug   Kind = "Bug"
	Step  Kind = "Step"
)

// CanPlace は、child の種類を parent の種類の下に置けるか(01「用語と階層」の表)。
// parent が nil なら、プロジェクトの直下。
func CanPlace(child Kind, parent *Kind) bool {
	switch child {
	case Epic:
		return parent == nil
	case Story, Task, Bug:
		return parent != nil && *parent == Epic
	case Step:
		return parent != nil && (*parent == Story || *parent == Task || *parent == Bug)
	}
	return false
}

// Place は、アイテムの場所(ワークスペース・プロジェクト・親・番号)。1 つの項目として変える。
type Place struct {
	Workspace workspace.ID `json:"workspace"`
	Project   project.ID   `json:"project"`
	Parent    *ID          `json:"parent"`
	Number    int          `json:"number"`
}

// Stamp は、作成の時刻と、作成した人。
type Stamp struct {
	At time.Time
	By string
}

// Item は、土台が扱う項目だけを持つ。ステータス・担当者などはトラック A・B が足す。
type Item struct {
	ID       ID
	Place    Place
	Kind     Kind
	Rank     string
	Title    string
	Created  Stamp
	Watchers []string // ウォッチ中の人のログイン名

	// 値は内部 ID で指す(保存形式「アイテムの例」)。初期値と変更の操作は A3 が足す(B2-1)。
	Status   change.ID                 // なければ空
	Priority change.ID                 // なければ空
	Systems  []change.ID               // 関連システム
	Choices  map[change.ID][]change.ID // 選択式の追加プロパティの値。単一選択は 1 つ。なければ nil

	// 選択式でない値。日付は YYYY-MM-DD。なければ空か nil。
	Assignee string                        // ログイン名
	Start    string                        // 開始日
	End      string                        // 終了日
	Points   *float64                      // ストーリーポイント
	Estimate *float64                      // 見積もり時間
	Fields   map[change.ID]json.RawMessage // 追加プロパティの値(ログの set の値のまま)。選択式の値は Choices にも入る
}

// DisplayID は、画面の ID(HH-12)。
func (it Item) DisplayID(prefix string) string {
	return prefix + "-" + strconv.Itoa(it.Place.Number)
}

// Create は、アイテムの作成の変更。値のない項目は書かない(読むときは空として扱う)。
// 作成の時刻と人は、変更記録の at と by から取る。種類と親の組み合わせは検査しない。呼ぶ側が先に CanPlace で確かめる。
func Create(it Item) change.Draft {
	to := map[string]any{
		"id": it.ID, "place": it.Place, "kind": it.Kind, "rank": it.Rank, "title": it.Title,
		"watchers": it.Watchers,
	}
	put := func(k string, v any, ok bool) {
		if ok {
			to[k] = v
		}
	}
	put("status", it.Status, it.Status != "")
	put("priority", it.Priority, it.Priority != "")
	put("assignee", it.Assignee, it.Assignee != "")
	put("start", it.Start, it.Start != "")
	put("end", it.End, it.End != "")
	put("points", it.Points, it.Points != nil)
	put("estimate", it.Estimate, it.Estimate != nil)
	put("systems", it.Systems, len(it.Systems) > 0)
	for p, raw := range it.Fields {
		to[FieldKey(p)] = raw
	}
	return change.NewCreate(to)
}

// FromLog は、アイテムのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Item, bool) {
	s := change.Resolve(log, change.Options{Multi: multi(log)})
	if s.Created == nil {
		return Item{}, false
	}
	it := Item{Created: Stamp{At: s.Created.At, By: s.Created.By}}
	_ = json.Unmarshal(s.Values["id"], &it.ID)
	_ = json.Unmarshal(s.Values["place"], &it.Place)
	_ = json.Unmarshal(s.Values["kind"], &it.Kind)
	_ = json.Unmarshal(s.Values["rank"], &it.Rank)
	_ = json.Unmarshal(s.Values["title"], &it.Title)
	for _, raw := range s.Lists["watchers"] {
		var login string
		if json.Unmarshal(raw, &login) == nil {
			it.Watchers = append(it.Watchers, login)
		}
	}
	readValues(&it, s)
	return it, true
}
