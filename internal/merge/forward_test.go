package merge

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
)

const (
	storeB = "01JC0M3R5W7M2X9V4T6R5N8P1B"
	storeC = "01JC0M3R5W7M2X9V4T6R5N8P1C"
	taskU  = "01JBZ8K3QW7M2X9V4T6R5N8P0A"
)

func forwardFile(ulid, to string) []byte {
	return []byte("---\nulid: " + ulid + "\nmoved_to: " + to + "\n---\n")
}

func TestMergeForwardBeatsEditedTask(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: old")
	edited := task("2026-10-31T11:00:00Z", "title: new")
	fwd := forwardFile(taskU, storeB)
	r := mergeTask(t, side(base, ""), side(fwd, "a"), side(edited, "b"))
	if !bytes.Equal(r.Data, fwd) {
		t.Fatalf("got %s", r.Data)
	}
	want := string(mustJSON(string(edited)))
	if len(r.Overwritten) != 1 || r.Overwritten[0] != (Overwritten{Path: taskPath, Field: "*", Value: want, Author: "b"}) {
		t.Errorf("overwritten = %+v", r.Overwritten)
	}
	if len(r.Notices) != 0 {
		t.Errorf("notices = %+v", r.Notices)
	}
}

func TestMergeForwardBeatsDeletion(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: old")
	fwd := forwardFile(taskU, storeB)
	r := mergeTask(t, side(base, ""), side(fwd, "a"), side(nil, "b"))
	if !bytes.Equal(r.Data, fwd) {
		t.Fatalf("got %s", r.Data)
	}
	if len(r.Overwritten) != 1 || r.Overwritten[0] != (Overwritten{Path: taskPath, Field: "*", Value: "null", Author: "b"}) {
		t.Errorf("overwritten = %+v", r.Overwritten)
	}
	if len(r.Notices) != 0 {
		t.Errorf("a forward over a deletion tells of no revival: %+v", r.Notices)
	}
}

func TestMergeTwoForwardsPicksNewerMovedTo(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: old")
	toB, toC := forwardFile(taskU, storeB), forwardFile(taskU, storeC)
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	o := Side{Data: toB, Author: "a", Time: at}
	th := Side{Data: toC, Author: "b", Time: at.Add(time.Minute)}
	r := mergeTask(t, side(base, ""), o, th)
	if !bytes.Equal(r.Data, toC) {
		t.Fatalf("got %s", r.Data)
	}
	if len(r.Overwritten) != 1 || r.Overwritten[0] != (Overwritten{Path: taskPath, Field: "moved_to", Value: `"` + storeB + `"`, Author: "a"}) {
		t.Errorf("overwritten = %+v", r.Overwritten)
	}
}

func TestMergeForwardUnderOursWins(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: old")
	edited := task("2026-10-31T11:00:00Z", "title: new")
	r := mergeWith(t, OursWins, side(base, ""), side(edited, "a"), side(forwardFile(taskU, storeB), "b"))
	if !bytes.Equal(r.Data, edited) || len(r.Overwritten) != 0 {
		t.Errorf("got %s %+v", r.Data, r.Overwritten)
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// forwardRepair runs Repair over f and returns the files after it, what it
// repaired and what it dropped.
func forwardRepair(t *testing.T, f memFiles) (memFiles, []Repaired, []Overwritten) {
	t.Helper()
	out, rs, dropped, err := Repair(RepairInput{
		Files:    f,
		Author:   func(p string) string { return "by " + p },
		OnOrigin: func(model.ULID, string) bool { return false },
		Changed:  func(string) bool { return true },
		Now:      time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	after := memFiles{}
	for p, b := range f {
		after[p] = b
	}
	for p, b := range out {
		if b == nil {
			delete(after, p)
		} else {
			after[p] = b
		}
	}
	return after, rs, dropped
}

const (
	fwdTask   = "01JD0000000000000000000001"
	childTask = "01JD0000000000000000000002"
	otherTask = "01JD0000000000000000000003"
)

func TestRepairChildOfForwardGoesToRoot(t *testing.T) {
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(fwdTask):                    forwardFile(fwdTask, storeB),
		pathOf(childTask):                  taskFile(childTask, "WEB-2", wsA, fwdTask, "2026-10-31T10:00:00Z", "rank: a0"),
		pathOf(otherTask):                  taskFile(otherTask, "WEB-3", wsA, "", "2026-10-31T10:00:00Z", "rank: a1"),
	}
	after, rs, dropped := forwardRepair(t, f)
	if got := decoded(t, after[pathOf(childTask)]); got.Parent != nil || got.Rank <= "a1" {
		t.Errorf("parent %v rank %s", got.Parent, got.Rank)
	}
	want := Repaired{ULID: mustULID(childTask), Kind: "parent_moved", Value: `{"from":"` + fwdTask + `","to":null}`, Author: "by " + pathOf(childTask)}
	if len(rs) != 1 || rs[0] != want || len(dropped) != 0 {
		t.Errorf("repaired = %+v dropped = %+v", rs, dropped)
	}
	if !bytes.Equal(after[pathOf(fwdTask)], f[pathOf(fwdTask)]) {
		t.Error("the forward changed")
	}
}

func TestRepairBlockedByAndTaskRefToForward(t *testing.T) {
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(fwdTask):                    forwardFile(fwdTask, storeB),
		pathOf(childTask): taskFile(childTask, "WEB-2", wsA, "", "2026-10-31T10:00:00Z",
			"blocked_by: ["+fwdTask+", "+otherTask+"]", "props:\n  duplicates: ["+fwdTask+"]\n  relates: "+fwdTask),
		pathOf(otherTask): taskFile(otherTask, "WEB-3", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, rs, _ := forwardRepair(t, f)
	got := decoded(t, after[pathOf(childTask)])
	if !slices.Equal(got.BlockedBy, []model.ULID{mustULID(otherTask)}) || got.Props.Get("duplicates") != nil || got.Props.Get("relates") != nil {
		t.Errorf("blocked_by %v props %v", got.BlockedBy, got.Props)
	}
	var kinds, values []string
	for _, x := range rs {
		kinds = append(kinds, x.Kind)
		values = append(values, x.Value)
		if x.ULID.String() != childTask || x.Author != "by "+pathOf(childTask) {
			t.Errorf("repaired = %+v", x)
		}
	}
	wantValues := []string{
		`{"field":"blocked_by","target":"` + fwdTask + `"}`,
		`{"field":"props.duplicates","target":"` + fwdTask + `"}`,
		`{"field":"props.relates","target":"` + fwdTask + `"}`,
	}
	if !slices.Equal(kinds, []string{"ref_moved", "ref_moved", "ref_moved"}) || !slices.Equal(values, wantValues) {
		t.Errorf("kinds %v values %v", kinds, values)
	}
}

func TestRepairDropsFilesUnderForward(t *testing.T) {
	c1, w1 := "01JE0000000000000000000001", "01JE0000000000000000000002"
	keep := "01JE0000000000000000000003"
	fwdUnder := func(base func(task, child model.ULID) string, child string) string {
		return base(mustULID(fwdTask), mustULID(child))
	}
	cPath, wPath := fwdUnder(model.CommentPath, c1), fwdUnder(model.WorklogPath, w1)
	kept := model.CommentPath(mustULID(otherTask), mustULID(keep))
	f := memFiles{
		pathOf(fwdTask):                    forwardFile(fwdTask, storeB),
		pathOf(otherTask):                  taskFile(otherTask, "WEB-3", wsA, "", "2026-10-31T10:00:00Z"),
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		cPath:                              []byte("a comment\n"),
		wPath:                              []byte("a worklog\n"),
		kept:                               []byte("stays\n"),
	}
	after, rs, dropped := forwardRepair(t, f)
	if _, ok := after[cPath]; ok {
		t.Error("the comment stays")
	}
	if _, ok := after[wPath]; ok {
		t.Error("the worklog stays")
	}
	if _, ok := after[kept]; !ok {
		t.Error("a comment under a live task was removed")
	}
	if len(rs) != 0 {
		t.Errorf("repaired = %+v", rs)
	}
	want := []Overwritten{
		{Path: cPath, Field: "*", Value: string(mustJSON("a comment\n")), Author: "by " + cPath},
		{Path: wPath, Field: "*", Value: string(mustJSON("a worklog\n")), Author: "by " + wPath},
	}
	if !slices.Equal(dropped, want) {
		t.Errorf("dropped = %+v", dropped)
	}
}

func TestRepairForwardListedTwiceIsToldOnce(t *testing.T) {
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(fwdTask):                    forwardFile(fwdTask, storeB),
		pathOf(childTask): taskFile(childTask, "WEB-2", wsA, "", "2026-10-31T10:00:00Z",
			"blocked_by: ["+fwdTask+", "+fwdTask+"]", "props:\n  duplicates: ["+fwdTask+", "+fwdTask+"]"),
	}
	after, rs, _ := forwardRepair(t, f)
	if got := decoded(t, after[pathOf(childTask)]); len(got.BlockedBy) != 0 || got.Props.Get("duplicates") != nil {
		t.Errorf("blocked_by %v props %v", got.BlockedBy, got.Props)
	}
	if len(rs) != 2 {
		t.Errorf("repaired = %+v; want one record per field and target", rs)
	}
}
