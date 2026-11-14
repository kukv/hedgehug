package transfer_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 02「保存」のメンバー: 担当者・ユーザー型の値・ウォッチ・作成者・コメントと作業時間の書き手・履歴の操作者のうち、
// 移動先のメンバーにいない人(大文字小文字だけが違う人は同じ人)が、1 人 1 回だけ足される。
func TestPeopleAddsEachMissingPersonOnce(t *testing.T) {
	from := workspace.Settings{Properties: []workspace.Property{
		{ID: "PR", Key: "reviewer", Name: "レビュアー", Type: workspace.TypeUser},
		{ID: "PM", Key: "memo", Name: "メモ", Type: workspace.TypeText},
	}}
	it := item.Item{
		ID: "I1", Assignee: "Taro", Watchers: []string{"hanako", "HANAKO"}, Created: item.Stamp{By: "jiro"},
		Fields: map[change.ID]json.RawMessage{"PR": json.RawMessage(`"saburo"`), "PM": json.RawMessage(`"mika"`)}, // テキストは人ではない
	}
	logs := [][]change.Record{{
		{By: "shiro", T: change.Comment},                 // コメントの書き手
		{By: "shiro", T: change.Worklog},                 // 作業時間の書き手(同じ人は 1 回だけ)
		{By: change.BotLogin, T: change.Set, F: "title"}, // 人でない実行者は数えない
		{By: "bad name", T: change.Set, F: "title"},      // ログイン名に使えない文字
		{By: "goro", T: change.Set, F: "title"},          // 履歴の操作者
	}}
	fromMembers := []member.Member{{Login: "hanako", Name: "花子"}, {Login: "jiro", Name: "次郎"}, {Login: "Taro", Name: "太郎"}, {Login: "goro", Name: "五郎"}}
	toMembers := []member.Member{{Login: "taro", Name: "taro"}, {Login: "jiro", Name: "jiro"}}

	add, people := transfer.People(from, []item.Item{it}, logs, nil, fromMembers, toMembers)

	wantAdd := []member.Member{{Login: "goro", Name: "五郎"}, {Login: "hanako", Name: "花子"}, {Login: "saburo", Name: "saburo"}, {Login: "shiro", Name: "shiro"}}
	if !reflect.DeepEqual(add, wantAdd) {
		t.Errorf("add = %+v, want %+v", add, wantAdd)
	}
	wantMap := transfer.PeopleMap{"taro": "taro", "jiro": "jiro", "hanako": "hanako", "goro": "goro", "saburo": "saburo", "shiro": "shiro"}
	if !reflect.DeepEqual(people, wantMap) {
		t.Errorf("map = %v, want %v (移動先にいる人は移動先の綴りに置き換える)", people, wantMap)
	}
}

func TestPeopleWithNobodyMissing(t *testing.T) {
	it := item.Item{ID: "I1", Assignee: "taro", Created: item.Stamp{By: "taro"}}
	add, people := transfer.People(workspace.Settings{}, []item.Item{it}, nil, nil,
		[]member.Member{{Login: "taro", Name: "太郎"}}, []member.Member{{Login: "Taro", Name: "Taro"}})
	if len(add) != 0 || people["taro"] != "Taro" {
		t.Errorf("add = %+v, map = %v", add, people)
	}
}

// 移動元のメンバーにいない人が複数の綴りで現れたら、辞書順で先頭の綴りで足す。
func TestPeopleSpellsAnUnknownPersonWithTheFirstInDictionaryOrder(t *testing.T) {
	it := item.Item{ID: "I1", Assignee: "shiro", Watchers: []string{"Shiro"}}
	add, people := transfer.People(workspace.Settings{}, []item.Item{it}, nil, nil, nil, nil)
	want := []member.Member{{Login: "Shiro", Name: "Shiro"}}
	if !reflect.DeepEqual(add, want) || people["shiro"] != "Shiro" {
		t.Errorf("add = %+v, map = %v", add, people)
	}
}

// 組み込みのプロパティは、ユーザー型でも人として数えない。
func TestPeopleIgnoresBuiltinUserProperties(t *testing.T) {
	from := workspace.Settings{Properties: []workspace.Property{{ID: "PA", Key: "assignee", Type: workspace.TypeUser, Builtin: true}}}
	it := item.Item{ID: "I1", Fields: map[change.ID]json.RawMessage{"PA": json.RawMessage(`"ghost"`)}}
	add, people := transfer.People(from, []item.Item{it}, nil, nil, nil, nil)
	if len(add) != 0 || len(people) != 0 {
		t.Errorf("add = %+v, map = %v", add, people)
	}
}

// プロジェクトのオーナーや専用ルールの担当者(extra)も人として数える。ログイン名に使えない値と空は数えない。
func TestPeopleCountsExtra(t *testing.T) {
	toMembers := []member.Member{{Login: "taro", Name: "taro"}}
	add, people := transfer.People(workspace.Settings{}, nil, nil, []string{"Hanako", "@none", ""}, nil, toMembers)
	want := []member.Member{{Login: "Hanako", Name: "Hanako"}}
	if !reflect.DeepEqual(add, want) {
		t.Errorf("add = %+v, want %+v", add, want)
	}
	if !reflect.DeepEqual(people, transfer.PeopleMap{"hanako": "Hanako"}) {
		t.Errorf("map = %v (@none と空の人は入らない)", people)
	}
}
