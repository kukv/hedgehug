package gitstore

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	commentlog "github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// noteID は、26 文字の内部 ID(先頭に 01N を付け、s を右に寄せて 0 で埋める)。
func noteID(s string) change.ID { return change.ID(fmt.Sprintf("01N%023s", s)) }

// notesYAML は、item.yaml のうち、関連・コメント・作業時間の部分を読み戻す形。
type notesYAML struct {
	Relations []struct{ Type, To string } `yaml:"relations"`
	Comments  []struct {
		ID, By, At, Body string
		Edited           bool
	} `yaml:"comments"`
	Worklogs []struct {
		ID, By, Date string
		Hours        float64
		Memo         string
	} `yaml:"worklogs"`
}

// notesLog は、関連 1 つ・コメント 2 つ(1 つは編集、1 つは削除)・作業時間 3 つ(1 つは削除)を足したアイテムのログ。
func notesLog() []change.Record {
	jst := time.FixedZone("JST", 9*3600)
	n := 0
	newID := func() change.ID { n++; return noteID(fmt.Sprintf("R%d", n)) }
	var log []change.Record
	add := func(by string, minute int, ds ...change.Draft) {
		meta := change.Meta{Op: noteID(fmt.Sprintf("O%d", minute)), At: time.Date(2026, 11, 12, 9, minute, 0, 0, jst), Actor: change.User(by)}
		log = append(log, change.Stamp(log, ds, meta, newID)...)
	}
	it := item.Item{ID: noteID("A1"), Place: item.Place{Workspace: yWS, Project: yProj, Number: 1}, Kind: item.Task, Rank: "V", Title: "t"}
	first := commentlog.Value("@taro デザイン見てください\n二行目")
	add("hanako", 0, item.Create(it))
	add("hanako", 1, change.NewAdd("relations", item.Relation{Type: "blocks", To: noteID("B1")}))
	add("hanako", 2, change.NewComment(noteID("C1"), nil, first))
	add("hanako", 3, change.NewComment(noteID("C2"), nil, commentlog.Value("消すコメント")))
	add("hanako", 4, change.NewComment(noteID("C2"), commentlog.Value("消すコメント"), nil))
	add("hanako", 5, change.NewComment(noteID("C1"), first, commentlog.Value("直しました\n二行目も")))
	add("taro", 6, change.NewWorklog(noteID("W1"), nil, worklog.Value(150, "下書き", "2026-11-12")))
	add("taro", 7, change.NewWorklog(noteID("W2"), nil, worklog.Value(200, "", "2026-11-13")))
	add("taro", 8, change.NewWorklog(noteID("W3"), nil, worklog.Value(25, "消す", "2026-11-13")))
	add("taro", 9, change.NewWorklog(noteID("W3"), worklog.Value(25, "消す", "2026-11-13"), nil))
	return log
}

func TestItemYAMLHasRelationsCommentsWorklogs(t *testing.T) {
	out, err := renderYAML(change.KindItem, notesLog(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got notesYAML
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("読み戻せない: %v\n%s", err, out)
	}
	if len(got.Relations) != 1 || got.Relations[0].Type != "blocks" || got.Relations[0].To != string(noteID("B1")) {
		t.Errorf("relations = %+v\n%s", got.Relations, out)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("comments = %+v(削除したコメントが残っている、または出ていない)\n%s", got.Comments, out)
	}
	c := got.Comments[0]
	if c.ID != string(noteID("C1")) || c.By != "hanako" || c.At != "2026-11-12T09:02:00.000+09:00" || c.Body != "直しました\n二行目も" || !c.Edited {
		t.Errorf("comment = %+v(投稿者と時刻は最初の行、本文は編集のあと、複数行も保つ)\n%s", c, out)
	}
	if len(got.Worklogs) != 2 {
		t.Fatalf("worklogs = %+v(削除した記録が残っている、または出ていない)\n%s", got.Worklogs, out)
	}
	w1, w2 := got.Worklogs[0], got.Worklogs[1]
	if w1.ID != string(noteID("W1")) || w1.By != "taro" || w1.Date != "2026-11-12" || w1.Hours != 1.5 || w1.Memo != "下書き" {
		t.Errorf("worklog 1 = %+v\n%s", w1, out)
	}
	if w2.ID != string(noteID("W2")) || w2.Date != "2026-11-13" || w2.Hours != 2 || w2.Memo != "" {
		t.Errorf("worklog 2 = %+v\n%s", w2, out)
	}
	mustContain(t, "item.yaml", string(out),
		"relations:\n  - {type: blocks, to: "+string(noteID("B1"))+"}\n",
		"edited: true}\n",
	)
}

func TestItemYAMLNotesAreEmptyLists(t *testing.T) {
	it := item.Item{ID: noteID("A1"), Place: item.Place{Workspace: yWS, Project: yProj, Number: 1}, Kind: item.Task, Rank: "V", Title: "t"}
	meta := change.Meta{Op: noteID("O1"), At: time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	log := change.Stamp(nil, []change.Draft{item.Create(it)}, meta, yIDs())
	out, err := renderYAML(change.KindItem, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "item.yaml", string(out), "relations: []\n", "comments: []\n", "worklogs: []\n")
}

// 書く道で足した行が item.yaml に出て、同期の作り直し(rebuildYAML)も同じ中身を作る。
func TestWritingNotesShowsInItemYAML(t *testing.T) {
	s, id, dir, _ := ySeed(t)
	op := port.Operation{Actor: change.User("hanako"), Summary: "コメントと作業時間を足す"}
	err := s.Write(t.Context(), id, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(yItem,
			change.NewAdd("relations", item.Relation{Type: "blocks", To: noteID("B1")}),
			change.NewComment(noteID("C1"), nil, commentlog.Value("確認")),
			change.NewWorklog(noteID("W1"), nil, worklog.Value(150, "下書き", "2026-11-12")),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	name := "items/" + string(yItem) + "/item.yaml"
	written := readFileString(t, dir, name)
	var got notesYAML
	if err := yaml.Unmarshal([]byte(written), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Relations) != 1 || len(got.Comments) != 1 || got.Comments[0].By != "hanako" || len(got.Worklogs) != 1 || got.Worklogs[0].Hours != 1.5 {
		t.Errorf("item.yaml に足した行が出ていない:\n%s", written)
	}
	if !worktreeStatus(t, dir).IsClean() {
		t.Error("作業ツリーがきれいでない(item.yaml が同じコミットに入っていない)")
	}
	tr := tree{}
	for _, rel := range []string{"workspaces/" + string(yWS), "projects/" + string(yProj), "items/" + string(yItem)} {
		tr[rel+"/log.jsonl"] = []byte(readFileString(t, dir, rel+"/log.jsonl"))
	}
	if err := rebuildYAML(tr); err != nil {
		t.Fatal(err)
	}
	if string(tr[name]) != written {
		t.Errorf("同期の作り直しが書く道と違う:\n--- 同期\n%s\n--- 書く道\n%s", tr[name], written)
	}
}

func TestItemYAMLWritesLinks(t *testing.T) {
	pr41 := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	pr40 := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 40}
	iss := itemlink.Ref{Kind: itemlink.Issue, Repo: "org/api", Number: 12}
	var log []change.Record
	n := 0
	newID := func() change.ID { n++; return noteID(fmt.Sprintf("L%d", n)) }
	add := func(minute int, ds ...change.Draft) {
		meta := change.Meta{Op: noteID(fmt.Sprintf("P%d", minute)), At: time.Date(2026, 11, 13, 9, minute, 0, 0, time.UTC), Actor: change.User("taro")}
		log = append(log, change.Stamp(log, ds, meta, newID)...)
	}
	add(0, item.Create(item.Item{ID: noteID("A1"), Place: item.Place{Workspace: yWS, Project: yProj, Number: 1}, Kind: item.Task, Rank: "V", Title: "t"}))
	add(1,
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "Add login form"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
		change.NewSet(itemlink.ValueField(pr41, "review"), nil, "pending"),
		change.NewAdd("links", iss),
		change.NewSet(itemlink.ValueField(iss, "title"), nil, "落ちる"),
		change.NewSet(itemlink.ValueField(iss, "state"), nil, "closed"),
	)
	add(2, change.NewAdd("links_off", pr40))
	out, err := renderYAML(change.KindItem, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "links:\n" +
		"  - {kind: pr, repo: org/web, number: 41, title: Add login form, state: open, review: pending}\n" +
		"  - {kind: issue, repo: org/api, number: 12, title: 落ちる, state: closed}\n" +
		"links_off:\n" +
		"  - {kind: pr, repo: org/web, number: 40}\n"
	if !strings.Contains(string(out), want) {
		t.Errorf("item.yaml =\n%s\nwant to contain\n%s", out, want)
	}
	if i, j := strings.Index(string(out), "estimate:"), strings.Index(string(out), "links:"); i < 0 || j < i {
		t.Errorf("links must come after estimate:\n%s", out)
	}
}
