package transfer_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/transfer"
)

var (
	jst = time.FixedZone("JST", 9*3600)
	at  = time.Date(2026, 11, 14, 9, 0, 0, 0, jst)
)

func TestRecordJSON(t *testing.T) {
	cross := transfer.Record{
		ID: "T1", Op: "O1", At: at, By: "taro", Entity: transfer.EntityItem, Target: "I1", Old: "HH-12", New: "WEB-31",
		ToStore: "S2", ToStoreName: "team-b", ToRemote: "https://github.com/org/team-b",
	}
	want := `{"id":"T1","op":"O1","at":"2026-11-14T09:00:00.000+09:00","by":"taro","entity":"item","target":"I1","old":"HH-12","new":"WEB-31","to_store":"S2","to_store_name":"team-b","to_remote":"https://github.com/org/team-b"}`
	if got := string(cross.JSON()); got != want {
		t.Errorf("cross-store JSON =\n%s\nwant\n%s", got, want)
	}
	// 同じ Store の別のワークスペースへの移動は、to_store に自分の Store ID だけを書く。
	same := transfer.Record{ID: "T2", Op: "O1", At: at, By: "taro", Entity: transfer.EntityItem, Target: "I1", Old: "HH-12", New: "MB-3", ToStore: "S1"}
	wantSame := `{"id":"T2","op":"O1","at":"2026-11-14T09:00:00.000+09:00","by":"taro","entity":"item","target":"I1","old":"HH-12","new":"MB-3","to_store":"S1"}`
	if got := string(same.JSON()); got != wantSame {
		t.Errorf("same-store JSON =\n%s\nwant\n%s", got, wantSame)
	}
}

func TestParseRecordsSkipsUnreadableLines(t *testing.T) {
	cross := transfer.Record{ID: "T1", Op: "O1", At: at, By: "taro", Entity: transfer.EntityProject, Target: "P1", Old: "HH-P1", New: "WEB-P4", ToStore: "S2", ToStoreName: "team-b"}
	in := string(cross.JSON()) + "\n" +
		"not json\n" +
		`{"id":"T3","at":"bad"}` + "\n" +
		`{"at":"2026-11-14T09:00:00.000+09:00","target":"I9"}` + "\n\n" +
		// entity のない行は item として読む。
		`{"id":"T4","op":"O2","at":"2026-11-14T09:00:00.000+09:00","by":"taro","target":"I2","old":"HH-1","new":"HH-2","to_store":"S2"}` + "\n"
	got := transfer.ParseRecords([]byte(in))
	if len(got) != 2 {
		t.Fatalf("records = %+v, want 2", got)
	}
	if !got[0].At.Equal(at) {
		t.Errorf("at = %v", got[0].At)
	}
	got[0].At = cross.At
	if !reflect.DeepEqual(got[0], cross) {
		t.Errorf("first = %+v, want %+v", got[0], cross)
	}
	if got[1].ID != "T4" || got[1].Entity != transfer.EntityItem || got[1].Old != "HH-1" || got[1].ToStore != "S2" {
		t.Errorf("second = %+v", got[1])
	}
}
