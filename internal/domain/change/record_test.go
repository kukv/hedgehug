package change_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

var jst = time.FixedZone("JST", 9*3600)

// 保存形式「アイテムの例」の 1 行と同じ並びで書く。
func TestRecordMarshal(t *testing.T) {
	r := change.Record{
		ID: "01JH7Q", Op: "01JH7Q",
		At:    time.Date(2026, 11, 12, 10, 3, 21, 512_000_000, jst),
		By:    "taro",
		Actor: change.User("taro"),
		T:     change.Set, F: "status",
		From: change.Value("S1"), To: change.Value("S3"),
		Prev: []change.ID{"01JF3K"},
	}
	got, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"01JH7Q","op":"01JH7Q","at":"2026-11-12T10:03:21.512+09:00","by":"taro","actor":"user","t":"set","f":"status","from":"S1","to":"S3","prev":["01JF3K"]}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestRecordRoundTrip(t *testing.T) {
	root := true
	in := change.Record{
		ID: "a", Op: "o", At: time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, jst), By: "hanako",
		Actor: change.Actor{Kind: change.ActorAutomation, Login: "hanako", Rule: "R1"},
		T:     change.Set, F: "release", From: change.Value("REL1"), To: json.RawMessage("null"),
		Prev: []change.ID{"x", "y"}, Root: &root,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out change.Record
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	b2, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(b2) {
		t.Errorf("round trip changed the line:\n%s\n%s", b, b2)
	}
	if !out.At.Equal(in.At) || out.Actor != in.Actor || string(out.To) != "null" {
		t.Errorf("decoded %+v", out)
	}
}

func TestCreateHasEmptyPrev(t *testing.T) {
	b, err := json.Marshal(change.Record{ID: "a", Op: "a", At: time.Unix(0, 0).In(jst), Actor: change.User("t"), By: "t", T: change.Create, To: change.Value(map[string]string{"id": "a"})})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"a","op":"a","at":"1970-01-01T09:00:00.000+09:00","by":"t","actor":"user","t":"create","to":{"id":"a"},"prev":[]}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}

func TestRecordUnmarshalErrors(t *testing.T) {
	cases := []struct{ name, in string }{
		{"invalid JSON", `{"id":`},
		{"bad at", `{"id":"a","op":"a","at":"yesterday","by":"t","actor":"user","t":"set","prev":[]}`},
		{"unknown actor", `{"id":"a","op":"a","at":"2026-01-02T03:04:05.000+09:00","by":"t","actor":"robot","t":"set","prev":[]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r change.Record
			err := r.UnmarshalJSON([]byte(c.in)) // json.Unmarshal は壊れた JSON をここへ渡す前に弾くので、直接呼ぶ
			if err == nil {
				t.Fatal("error expected")
			}
			if !strings.Contains(err.Error(), "record") {
				t.Errorf("error lacks record context: %v", err)
			}
		})
	}
}

func TestActor(t *testing.T) {
	cases := []struct {
		a        change.Actor
		code, by string
	}{
		{change.User("taro"), "user", "taro"},
		{change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "R1"}, "automation:R1", "taro"},
		{change.Actor{Kind: change.ActorGitHub}, "github", change.BotLogin},
		{change.Actor{Kind: change.ActorSync}, "sync", change.BotLogin},
		{change.Actor{Kind: change.ActorNone}, "none", change.BotLogin},
	}
	for _, c := range cases {
		if c.a.Code() != c.code || c.a.By() != c.by {
			t.Errorf("%+v: code=%q by=%q", c.a, c.a.Code(), c.a.By())
		}
		back, err := change.ParseActor(c.code, c.by)
		if err != nil || back != c.a {
			t.Errorf("ParseActor(%q, %q) = %+v, %v", c.code, c.by, back, err)
		}
	}
	if _, err := change.ParseActor("robot", "x"); err == nil {
		t.Error("unknown actor accepted")
	}
}

func TestDrafts(t *testing.T) {
	if d := change.NewCreate(map[string]string{"id": "a"}); d.T != change.Create || string(d.To) != `{"id":"a"}` || d.From != nil || d.F != "" {
		t.Errorf("NewCreate = %+v", d)
	}
	if d := change.NewSet("title", "a", "b"); d.T != change.Set || string(d.From) != `"a"` || string(d.To) != `"b"` {
		t.Errorf("NewSet = %+v", d)
	}
	if d := change.NewAdd("systems", "Y1"); d.T != change.Add || string(d.To) != `"Y1"` || d.From != nil {
		t.Errorf("NewAdd = %+v", d)
	}
	if d := change.NewRemove("systems", "Y1"); d.T != change.Remove || string(d.From) != `"Y1"` || d.To != nil {
		t.Errorf("NewRemove = %+v", d)
	}
}

func TestValidID(t *testing.T) {
	for id, want := range map[change.ID]bool{
		"01TEST00000000000000000001": true, "a": true,
		"": false, "../x": false, "a/b": false, "a b": false, "あ": false, ".": false,
	} {
		if got := change.ValidID(id); got != want {
			t.Errorf("ValidID(%q) = %v", id, got)
		}
	}
}

// メンバーの一覧は Store に 1 つなので、ID が空のときだけ指せる。ほかの種類は ValidID の ID だけ。
func TestRefValid(t *testing.T) {
	for _, c := range []struct {
		ref  change.Ref
		want bool
	}{
		{change.Ref{Kind: change.KindMembers}, true},
		{change.Ref{Kind: change.KindMembers, ID: "x"}, false},
		{change.Ref{Kind: change.KindItem, ID: "01A"}, true},
		{change.Ref{Kind: change.KindWorkspace, ID: ""}, false},
		{change.Ref{Kind: change.KindProject, ID: "../x"}, false},
		{change.Ref{Kind: "robot", ID: "01A"}, false},
	} {
		if got := c.ref.Valid(); got != c.want {
			t.Errorf("%+v.Valid() = %v", c.ref, got)
		}
	}
}
