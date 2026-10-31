package merge

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestTaskOneSide(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a", "status: todo")
	ours := task("2026-10-31T11:00:00Z", "title: b", "status: todo")
	theirs := task("2026-10-31T12:00:00Z", "title: a", "status: doing")
	r := mergeTask(t, side(base, ""), side(ours, "github.com/a"), side(theirs, "github.com/b"))
	x := decoded(t, r.Data)
	if x.Title != "b" || x.Status != "doing" || len(r.Overwritten) != 0 {
		t.Errorf("title %s status %s overwritten %v", x.Title, x.Status, r.Overwritten)
	}
	if !x.Updated.Equal(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("updated = %s, want the larger", x.Updated)
	}
}

func TestTaskLWW(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	ours := task("2026-10-31T11:00:00Z", "title: ours")
	theirs := task("2026-10-31T12:00:00Z", "title: theirs")
	r := mergeTask(t, side(base, ""), side(ours, "github.com/a"), side(theirs, "github.com/b"))
	if x := decoded(t, r.Data); x.Title != "theirs" {
		t.Errorf("title = %s", x.Title)
	}
	want := []Overwritten{{Path: taskPath, Field: "title", Value: `"ours"`, Author: "github.com/a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestTaskLWWTieBreaks(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	same := "2026-10-31T11:00:00Z"
	r := mergeTask(t, side(base, ""), side(task(same, "title: x"), "github.com/a"), side(task(same, "title: y"), "github.com/b"))
	if x := decoded(t, r.Data); x.Title != "y" {
		t.Errorf("author tie-break: %s", x.Title) // github.com/b > github.com/a
	}
	r = mergeTask(t, side(base, ""), side(task(same, "title: x"), "u"), side(task(same, "title: y"), "u"))
	if x := decoded(t, r.Data); x.Title != "y" {
		t.Errorf("value tie-break: %s", x.Title) // "y" > "x"
	}
}

func TestTaskSameChangeNotOverwritten(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	r := mergeTask(t, side(base, ""), side(task("2026-10-31T11:00:00Z", "title: z"), "a"), side(task("2026-10-31T12:00:00Z", "title: z"), "b"))
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestTaskSets(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "labels: [a, b]", "former_ids: [OLD-1]")
	ours := task("2026-10-31T11:00:00Z", "labels: [b, c]", "former_ids: [OLD-1, X-1]")
	theirs := task("2026-10-31T12:00:00Z", "labels: [a, b, d]", "former_ids: [OLD-1, Y-1]")
	x := decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data)
	if !slices.Equal(x.Labels, []string{"b", "c", "d"}) {
		t.Errorf("labels = %v", x.Labels) // a removed by ours, c and d added (added ones sorted)
	}
	if !slices.Equal(x.FormerIDs, []string{"OLD-1", "X-1", "Y-1"}) {
		t.Errorf("former_ids = %v", x.FormerIDs)
	}
}

func TestTaskIDBothChangedKeepsLoserInFormer(t *testing.T) {
	base := task("2026-10-31T10:00:00Z")
	ours := bytes.Replace(task("2026-10-31T11:00:00Z", "former_ids: [WEB-1]"), []byte("id: WEB-1"), []byte("id: API-1"), 1)
	theirs := bytes.Replace(task("2026-10-31T12:00:00Z", "former_ids: [WEB-1]"), []byte("id: WEB-1"), []byte("id: BE-1"), 1)
	x := decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data)
	if x.ID != "BE-1" || !slices.Contains(x.FormerIDs, "API-1") || !slices.Contains(x.FormerIDs, "WEB-1") {
		t.Errorf("id = %s, former = %v", x.ID, x.FormerIDs)
	}
}

func TestTaskProps(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "props:", "  priority: low", "  points: 1")
	ours := task("2026-10-31T11:00:00Z", "props:", "  priority: high", "  points: 1")
	theirs := task("2026-10-31T12:00:00Z", "props:", "  priority: low", "  points: 3")
	x := decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data)
	if v := x.Props.Get("priority"); v == nil || v.Value != "high" {
		t.Errorf("priority = %v", v)
	}
	if v := x.Props.Get("points"); v == nil || v.Value != "3" {
		t.Errorf("points = %v", v)
	}
}

func TestTaskBodyDiff3(t *testing.T) {
	base := append(task("2026-10-31T10:00:00Z"), "1\n2\n"...)
	ours := append(task("2026-10-31T11:00:00Z"), "1\nA\n"...)
	theirs := append(task("2026-10-31T12:00:00Z"), "1\nB\n"...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	if x := decoded(t, r.Data); x.Body != "1\nA\n-----\nB\n" {
		t.Errorf("body = %q", x.Body) // older (ours) first
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overlap recorded as overwritten: %v", r.Overwritten)
	}
}

func TestTaskDeleteVsEdit(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	ours := task("2026-10-31T11:00:00Z", "title: a", "deleted_at: 2026-10-31T11:00:00Z")
	theirs := task("2026-10-31T12:00:00Z", "title: edited")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	if x.DeletedAt != nil || x.Title != "edited" {
		t.Errorf("deleted_at %v title %s", x.DeletedAt, x.Title)
	}
	if len(r.Notices) != 1 || r.Notices[0] != (Notice{Kind: Revived, Path: taskPath, Author: "a"}) {
		t.Errorf("notices = %v", r.Notices)
	}
}

func TestTaskPurgeVsEdit(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	r := mergeTask(t, side(base, ""), side(nil, "a"), side(task("2026-10-31T12:00:00Z", "title: b"), "b"))
	if r.Data == nil || len(r.Notices) != 1 {
		t.Errorf("data %q notices %v", r.Data, r.Notices)
	}
	r = mergeTask(t, side(base, ""), side(nil, "a"), side(base, "b"))
	if r.Data != nil {
		t.Errorf("purge of an unchanged file kept it")
	}
}

func TestTaskKeepsCommentsAndUnknownKeys(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a # keep", "zz: 1")
	ours := task("2026-10-31T11:00:00Z", "title: a # keep", "zz: 1", "status: doing")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(base, "b"))
	if !bytes.Contains(r.Data, []byte("# keep")) || !bytes.Contains(r.Data, []byte("zz: 1")) {
		t.Errorf("lost comment or key:\n%s", r.Data)
	}
}

func TestBrokenSideFallsBackToOtherFile(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	broken := []byte("---\nulid: [\n---\n")
	r, err := Merge(taskPath, schema.Default(), base, Side{Data: broken, Author: "a", Time: time.Unix(10, 0)}, Side{Data: task("2026-10-31T12:00:00Z", "title: b"), Author: "b", Time: time.Unix(20, 0)}, LWW)
	if err != nil || r.Data == nil || bytes.Equal(r.Data, broken) || len(r.Overwritten) != 1 || r.Overwritten[0].Field != "*" {
		t.Errorf("r = %+v, %v", r, err)
	}
}

// Both sides change the file, and the comments and unknown keys of the side
// the result is built on survive.
func TestTaskKeepsCommentsWhenBothChange(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "# head", "title: a # keep", "status: todo", "zz: 1 # mine")
	ours := task("2026-10-31T11:00:00Z", "# head", "title: b # keep", "status: todo", "zz: 1 # mine")
	theirs := task("2026-10-31T12:00:00Z", "# head", "title: a # keep", "status: doing", "zz: 1 # mine")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	for _, s := range []string{"# head", "title: b # keep", "zz: 1 # mine", "status: doing"} {
		if !bytes.Contains(r.Data, []byte(s)) {
			t.Errorf("missing %q:\n%s", s, r.Data)
		}
	}
}

func TestTaskScalarsOfEveryKind(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "parent: 01JBZ8K3QW7M2X9V4T6R5N8P0B", "rank: a0", "created: 2026-10-30T10:00:00+09:00")
	ours := task("2026-10-31T11:00:00Z", "parent: 01JBZ8K3QW7M2X9V4T6R5N8P0C", "rank: a1", "created: 2026-10-30T10:00:00+09:00")
	theirs := task("2026-10-31T12:00:00Z", "parent: 01JBZ8K3QW7M2X9V4T6R5N8P0D", "rank: a0", "created: 2026-10-30T10:00:00+09:00")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	if x.Parent == nil || x.Parent.String() != "01JBZ8K3QW7M2X9V4T6R5N8P0D" || x.Rank != "a1" {
		t.Errorf("parent %v rank %s", x.Parent, x.Rank)
	}
	want := []Overwritten{{Path: taskPath, Field: "parent", Value: `"01JBZ8K3QW7M2X9V4T6R5N8P0C"`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten) // created did not change: not overwritten
	}
	if !bytes.Contains(r.Data, []byte("created: 2026-10-30T10:00:00+09:00")) {
		t.Errorf("created rewritten:\n%s", r.Data)
	}
}

func TestTaskLinks(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "links: [{type: pr, ref: o/r#1}, {type: pr, ref: o/r#2}]")
	ours := task("2026-10-31T11:00:00Z", "links: [{type: pr, ref: o/r#1, sync: status}, {type: pr, ref: o/r#2}, {type: issue, ref: o/r#3}]")
	theirs := task("2026-10-31T12:00:00Z", "links: [{type: issue, ref: o/r#1}]")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	want := []model.Link{{Type: "issue", Ref: "o/r#1"}, {Type: "issue", Ref: "o/r#3"}}
	if !slices.Equal(x.Links, want) {
		t.Errorf("links = %v", x.Links) // #2 removed by theirs, #3 added by ours, #1 changed on both
	}
	ow := []Overwritten{{Path: taskPath, Field: "links", Value: `{"ref":"o/r#1","sync":"status","type":"pr"}`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, ow) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestTaskMultiPropIsASet(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "props:", "  relates: [01JBZ8K3QW7M2X9V4T6R5N8P0B]", "  priority: low")
	ours := task("2026-10-31T11:00:00Z", "props:", "  relates: [01JBZ8K3QW7M2X9V4T6R5N8P0B, 01JBZ8K3QW7M2X9V4T6R5N8P0C]", "  priority: high")
	theirs := task("2026-10-31T12:00:00Z", "props:", "  relates: [01JBZ8K3QW7M2X9V4T6R5N8P0D]", "  priority: mid")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	v := x.Props.Get("relates")
	if v == nil || len(v.Content) != 2 || v.Content[0].Value != "01JBZ8K3QW7M2X9V4T6R5N8P0C" || v.Content[1].Value != "01JBZ8K3QW7M2X9V4T6R5N8P0D" {
		t.Errorf("relates = %v\n%s", v, r.Data)
	}
	ow := []Overwritten{{Path: taskPath, Field: "props.priority", Value: `"high"`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, ow) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestTaskKeysOfMappings(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "props:", "  priority: low", "gh:", "  number: 1", "  state: open", "zz: 1", "yy: 1")
	ours := task("2026-10-31T11:00:00Z", "props:", "  points: 2", "gh:", "  number: 1", "  state: closed", "zz: 2", "yy: 1")
	theirs := task("2026-10-31T12:00:00Z", "props:", "  priority: high", "gh:", "  number: 2", "  state: open", "yy: 3", "xx: 1")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	// ours removed priority and theirs changed it: the change stays.
	if v := x.Props.Get("priority"); v == nil || v.Value != "high" {
		t.Errorf("priority = %v", v)
	}
	if v := x.Props.Get("points"); v == nil || v.Value != "2" {
		t.Errorf("points = %v", v)
	}
	var gh model.Fields
	if x.GH == nil || gh.UnmarshalYAML(x.GH) != nil || gh.Get("number").Value != "2" || gh.Get("state").Value != "closed" {
		t.Errorf("gh:\n%s", r.Data)
	}
	// theirs removed zz and ours changed it; yy changed on theirs; xx added.
	for _, k := range []struct{ key, value string }{{"zz", "2"}, {"yy", "3"}, {"xx", "1"}} {
		if v := x.Extra.Get(k.key); v == nil || v.Value != k.value {
			t.Errorf("%s = %v", k.key, v)
		}
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestTaskDeleteAlone(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a", "labels: [x]")
	ours := task("2026-10-31T11:00:00Z", "title: a", "labels: [x]", "deleted_at: 2026-10-31T11:00:00Z")
	theirs := task("2026-10-31T12:00:00Z", "title: a", "labels: [x]") // only updated moved
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	if x := decoded(t, r.Data); x.DeletedAt == nil || len(r.Notices) != 0 {
		t.Errorf("deleted_at %v notices %v", x.DeletedAt, r.Notices)
	}
}

func TestTaskBothDelete(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	ours := task("2026-10-31T11:00:00Z", "title: b", "deleted_at: 2026-10-31T11:00:00Z")
	theirs := task("2026-10-31T12:00:00Z", "title: a", "deleted_at: 2026-10-31T12:00:00Z")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	if x.DeletedAt == nil || !x.DeletedAt.Equal(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)) || len(r.Notices) != 0 {
		t.Errorf("deleted_at %v notices %v", x.DeletedAt, r.Notices)
	}
}

func TestTaskBothCreated(t *testing.T) {
	ours := task("2026-10-31T11:00:00Z", "title: a", "labels: [x]")
	theirs := task("2026-10-31T12:00:00Z", "title: b", "labels: [y]")
	r := mergeTask(t, side(nil, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	if x.Title != "b" || !slices.Equal(x.Labels, []string{"x", "y"}) {
		t.Errorf("title %s labels %v", x.Title, x.Labels)
	}
	// A base that cannot be read is taken as no base.
	r2 := mergeTask(t, side([]byte("---\nulid: [\n---\n"), ""), side(ours, "a"), side(theirs, "b"))
	if !bytes.Equal(r2.Data, r.Data) {
		t.Errorf("broken base:\n%s", r2.Data)
	}
}

func TestTaskOursWins(t *testing.T) {
	base := append(task("2026-10-31T10:00:00Z", "title: a", "status: todo", "labels: [x]"), "1\n2\n3\n"...)
	ours := append(task("2026-10-31T11:00:00Z", "title: ours", "status: todo", "labels: [x, y]"), "1\n2\nA\n"...)
	theirs := append(task("2026-10-31T12:00:00Z", "title: theirs", "status: done", "labels: []"), "T\n2\nB\n"...)
	r := mergeWith(t, OursWins, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	// theirs' change apart from ours is applied; the overlap takes ours.
	if x.Title != "ours" || x.Status != "done" || !slices.Equal(x.Labels, []string{"y"}) || x.Body != "T\n2\nA\n" {
		t.Errorf("title %s status %s labels %v body %q", x.Title, x.Status, x.Labels, x.Body)
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}
