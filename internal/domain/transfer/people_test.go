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

	add, people := transfer.People(from, []item.Item{it}, logs, fromMembers, toMembers)

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
	add, people := transfer.People(workspace.Settings{}, []item.Item{it}, nil,
		[]member.Member{{Login: "taro", Name: "太郎"}}, []member.Member{{Login: "Taro", Name: "Taro"}})
	if len(add) != 0 || people["taro"] != "Taro" {
		t.Errorf("add = %+v, map = %v", add, people)
	}
}
