package workspace_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// rows は、drafts を 1 行ずつ別の操作として並べる(prev は Stamp が付ける)。
func rows(drafts ...change.Draft) []change.Record {
	var log []change.Record
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	for i, d := range drafts {
		id := change.ID(fmt.Sprintf("01REC%021d", i))
		meta := change.Meta{Op: id, At: at.Add(time.Duration(i) * time.Second), Actor: change.User("taro")}
		log = append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return id })...)
	}
	return log
}

// 作成後に足したプロパティと選択肢、値の追加と削除が、設定に出る(B2-3〜B2-5)。
func TestSettingsAddsAndDeletes(t *testing.T) {
	ids := seqIDs()
	w := workspace.Workspace{ID: "01WS0000000000000000000000", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(ids)
	create, err := workspace.CreateWithSettings(w, s)
	if err != nil {
		t.Fatal(err)
	}
	cust := workspace.Property{
		ID: "01PROP0000000000000000000A", Key: "customer", Name: "顧客", Type: workspace.TypeSingle,
		Kinds: []string{"Story", "Task"}, Rank: workspace.NextRank(s.Properties[len(s.Properties)-1].Rank),
	}
	l := workspace.List{Kind: workspace.ListChoices, Property: cust.ID}
	a := workspace.Value{ID: "01CHOICE00000000000000000A", Name: "A社", Color: "gray", Rank: "V"}
	b := workspace.Value{ID: "01CHOICE00000000000000000B", Name: "B社", Color: "slate", Rank: "W"}
	review := workspace.Value{ID: "01STATUS00000000000000000R", Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	drafts := []change.Draft{create}
	drafts = append(drafts, workspace.PropertyDrafts(cust)...)
	drafts = append(drafts, workspace.ValueDrafts(l, a)...)
	drafts = append(drafts, workspace.ValueDrafts(l, b)...)
	drafts = append(drafts, workspace.ValueDrafts(workspace.ParseList("statuses"), review)...)
	drafts = append(drafts,
		change.NewDelete(l.Ref(b.ID)),
		change.NewDelete("systems/"+string(s.Systems[0].ID)),
	)
	got, ok := workspace.SettingsFromLog(rows(drafts...))
	if !ok {
		t.Fatal("not created")
	}
	p, _ := got.Property(cust.ID)
	cust.Choices = []workspace.Value{a}
	if !reflect.DeepEqual(p, cust) {
		t.Errorf("property = %+v\nwant %+v", p, cust)
	}
	if last := got.Statuses[len(got.Statuses)-1]; last != review {
		t.Errorf("statuses = %+v", got.Statuses)
	}
	if len(got.Systems) != 5 || got.Systems[0].Name != "API" {
		t.Errorf("systems = %+v", got.Systems)
	}
	// プロパティを消すと、その選択肢ごと消える。
	got, _ = workspace.SettingsFromLog(rows(append(drafts, change.NewDelete("properties/"+string(cust.ID)))...))
	if _, ok := got.Property(cust.ID); ok {
		t.Error("deleted property is still there")
	}
}

// 削除の行を取り消す restore の行(f が同じで、to が削除の op)があれば、そのステータスは一覧に戻る(C3a-7)。
func TestSettingsReadsRestoredDelete(t *testing.T) {
	w := workspace.Workspace{ID: "01WS0000000000000000000000", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(seqIDs())
	create, err := workspace.CreateWithSettings(w, s)
	if err != nil {
		t.Fatal(err)
	}
	review := workspace.Value{ID: "01STATUS00000000000000000R", Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	list := workspace.ParseList("statuses")
	drafts := []change.Draft{create}
	drafts = append(drafts, workspace.ValueDrafts(list, review)...)
	drafts = append(drafts, change.NewDelete(list.Ref(review.ID)))
	log := rows(drafts...)
	del := log[len(log)-1]
	got, _ := workspace.SettingsFromLog(log)
	if last := got.Statuses[len(got.Statuses)-1]; last == review {
		t.Fatalf("deleted status is still listed: %+v", got.Statuses)
	}
	got, _ = workspace.SettingsFromLog(rows(append(drafts, change.NewRestore(list.Ref(review.ID), del.Op))...))
	if last := got.Statuses[len(got.Statuses)-1]; last != review {
		t.Errorf("restored status is not listed: %+v", got.Statuses)
	}
}
