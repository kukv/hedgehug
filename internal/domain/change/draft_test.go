package change_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestDescCommentWorklogDrafts(t *testing.T) {
	if d := change.NewDesc(); d.T != change.Desc || d.F != "" || d.From != nil || d.To != nil {
		t.Errorf("NewDesc = %+v", d)
	}
	c := change.NewComment("C1", nil, map[string]string{"body": "確認"})
	if c.T != change.Comment || c.F != "C1" || string(c.From) != "null" || string(c.To) != `{"body":"確認"}` {
		t.Errorf("NewComment = %+v", c)
	}
	del := change.NewWorklog("W1", map[string]any{"hours": 100}, nil)
	if del.T != change.Worklog || del.F != "W1" || string(del.To) != "null" {
		t.Errorf("NewWorklog (delete) = %+v", del)
	}
	// desc の行は f・from・to を持たず、削除の to は null のまま書いて読み戻せる。
	n := 0
	meta := change.Meta{Op: "o", At: time.Date(2026, 11, 13, 9, 0, 0, 0, jst), Actor: change.User("taro")}
	recs := change.Stamp(nil, []change.Draft{change.NewDesc(), del}, meta, func() change.ID { n++; return change.ID("r" + string(rune('0'+n))) })
	want := []string{
		`{"id":"r1","op":"o","at":"2026-11-13T09:00:00.000+09:00","by":"taro","actor":"user","t":"desc","prev":[]}`,
		`{"id":"r2","op":"o","at":"2026-11-13T09:00:00.000+09:00","by":"taro","actor":"user","t":"worklog","f":"W1","from":{"hours":100},"to":null,"prev":[]}`,
	}
	for i, r := range recs {
		b, err := json.Marshal(r)
		if err != nil || string(b) != want[i] {
			t.Fatalf("line %d = %s, %v\nwant %s", i, b, err, want[i])
		}
		var back change.Record
		if err := json.Unmarshal(b, &back); err != nil || back.T != r.T || string(back.To) != string(r.To) {
			t.Errorf("round trip %d: %+v, %v", i, back, err)
		}
	}
}
