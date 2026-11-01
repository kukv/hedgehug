package inbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestAddDropsDuplicates(t *testing.T) {
	_, b := twoClones(t)
	box := Open(b.s)
	task := model.NewULID()
	ws := model.NewULID()
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	items := []Item{
		{Kind: Mention, Task: task, Commit: "c1", Author: "github.com/a", Time: t0},
		{Kind: Overwritten, Task: task, Path: "p.md", Commit: "c1", To: "github.com/b", Time: t0, Detail: json.RawMessage(`{"field":"title","value":"x"}`)},
		{Kind: PrefixDuplicate, Commit: "c1", Author: "github.com/b", Time: t0, Detail: json.RawMessage(`{"kind":"prefix_duplicate","workspace":"` + ws.String() + `","value":"WEB"}`)},
	}
	added, err := box.Add(items)
	if err != nil || len(added) != 3 || added[0].ULID.IsZero() {
		t.Fatalf("added = %+v, %v", added, err)
	}
	again := []Item{
		{Kind: Mention, Task: task, Commit: "c1", Author: "github.com/a", Time: t0.Add(time.Hour)},                                                               // the same commit
		{Kind: Overwritten, Task: task, Path: "p.md", Commit: "c2", To: "GitHub.com/b", Time: t0, Detail: json.RawMessage(`{ "field": "title", "value": "x" }`)}, // another merge of the same pair
		{Kind: PrefixDuplicate, Commit: "c2", Author: "github.com/b", Time: t0, Detail: json.RawMessage(`{"kind":"prefix_duplicate","workspace":"` + ws.String() + `","value":"WEB"}`)},
		{Kind: Mention, Task: task, Commit: "c2", Author: "github.com/a", Time: t0}, // a new commit
		{Kind: Mention, Task: task, Commit: "c2", Author: "github.com/a", Time: t0}, // twice in one call
	}
	added, err = box.Add(again)
	if err != nil || len(added) != 1 || added[0].Commit != "c2" || added[0].Kind != Mention {
		t.Errorf("added = %+v, %v", added, err)
	}
	all, err := box.List(true)
	if err != nil || len(all) != 4 {
		t.Errorf("List = %+v, %v", all, err)
	}
}

func TestListAndMarkRead(t *testing.T) {
	_, b := twoClones(t)
	box := Open(b.s)
	if items, err := box.List(true); err != nil || len(items) != 0 {
		t.Fatalf("empty inbox = %v, %v", items, err)
	}
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	added, err := box.Add([]Item{
		{Kind: Assigned, Task: model.NewULID(), Commit: "c1", Time: t0},
		{Kind: Assigned, Task: model.NewULID(), Commit: "c2", Time: t0.Add(2 * time.Hour)},
		{Kind: Assigned, Task: model.NewULID(), Commit: "c3", Time: t0.Add(time.Hour)},
	})
	if err != nil {
		t.Fatal(err)
	}
	commits := func(items []Item) string {
		s := ""
		for _, it := range items {
			s += it.Commit + " "
		}
		return s
	}
	if items, err := box.List(false); err != nil || commits(items) != "c2 c3 c1 " {
		t.Errorf("List(false) = %s, %v", commits(items), err)
	}
	if err := box.MarkRead([]model.ULID{added[2].ULID}); err != nil {
		t.Fatal(err)
	}
	if items, _ := box.List(false); commits(items) != "c2 c1 " {
		t.Errorf("after MarkRead: List(false) = %s", commits(items))
	}
	if items, _ := box.List(true); commits(items) != "c2 c3 c1 " {
		t.Errorf("after MarkRead: List(true) = %s", commits(items))
	}
	if err := box.MarkRead([]model.ULID{model.NewULID()}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("MarkRead of an unknown item: %v", err)
	}
	it, ok, err := box.Item(added[1].ULID)
	if err != nil || !ok || it.Commit != "c2" {
		t.Errorf("Item = %+v, %v, %v", it, ok, err)
	}
	if _, ok, err := box.Item(model.NewULID()); err != nil || ok {
		t.Errorf("Item of an unknown ulid: %v, %v", ok, err)
	}
	if err := box.MarkAllRead(); err != nil {
		t.Fatal(err)
	}
	if items, _ := box.List(false); len(items) != 0 {
		t.Errorf("after MarkAllRead: %s", commits(items))
	}
	// Read again from the files.
	if items, _ := Open(b.s).List(true); commits(items) != "c2 c3 c1 " {
		t.Errorf("reopened: %s", commits(items))
	}
}

func TestCorruptLineIsSkipped(t *testing.T) {
	_, b := twoClones(t)
	box := Open(b.s)
	added, err := box.Add([]Item{{Kind: Assigned, Task: model.NewULID(), Commit: "c1"}})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := b.s.ReadLocal(itemsFile)
	if err := b.s.WriteLocal(itemsFile, append([]byte("{broken\n"), data...)); err != nil {
		t.Fatal(err)
	}
	box = Open(b.s)
	if _, err := box.Add([]Item{{Kind: Assigned, Task: model.NewULID(), Commit: "c2"}}); err != nil {
		t.Fatal(err)
	}
	if err := box.MarkRead([]model.ULID{added[0].ULID}); err != nil {
		t.Fatal(err)
	}
	items, err := box.List(false)
	if err != nil || len(items) != 1 || items[0].Commit != "c2" {
		t.Errorf("List = %+v, %v", items, err)
	}
	if w := box.Warnings(); len(w) != 1 || !strings.Contains(w[0], "inbox.jsonl:1") {
		t.Errorf("warnings = %q", w)
	}
	if data, _ := b.s.ReadLocal(itemsFile); !strings.HasPrefix(string(data), "{broken\n") {
		t.Errorf("the line is not kept: %s", data)
	}
}

func TestTwoWritersAtOnce(t *testing.T) {
	_, b := twoClones(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Go(func() {
			box := Open(b.s) // a box each, as two processes have
			if _, err := box.Add([]Item{{Kind: Assigned, Task: model.NewULID(), Commit: fmt.Sprint(i)}}); err != nil {
				errs <- err
			}
			if err := box.MarkAllRead(); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if items, err := Open(b.s).List(true); err != nil || len(items) != 20 {
		t.Errorf("%d items, %v", len(items), err)
	}
	if items, _ := Open(b.s).List(false); len(items) != 0 {
		t.Errorf("unread: %+v", items)
	}
}

func TestReadSetKeepsOnlyItems(t *testing.T) {
	_, b := twoClones(t)
	box := Open(b.s)
	added, err := box.Add([]Item{{Kind: Assigned, Task: model.NewULID(), Commit: "c1"}})
	if err != nil {
		t.Fatal(err)
	}
	gone := model.NewULID()
	if err := b.s.WriteLocal(readFile, []byte(`["`+gone.String()+`"]`)); err != nil {
		t.Fatal(err)
	}
	if err := box.MarkAllRead(); err != nil {
		t.Fatal(err)
	}
	data, _ := b.s.ReadLocal(readFile)
	var read []model.ULID
	if err := json.Unmarshal(data, &read); err != nil || !slices.Equal(read, []model.ULID{added[0].ULID}) {
		t.Errorf("inbox-read.json = %s, %v", data, err)
	}
}

func TestStampChangesWithTheFiles(t *testing.T) {
	_, b := twoClones(t)
	box := Open(b.s)
	before := box.Stamp()
	if _, err := box.Add([]Item{{Kind: Mention, Task: model.NewULID(), Commit: "c", Time: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	after := box.Stamp()
	if before == after {
		t.Fatalf("stamp did not change: %q", after)
	}
	if box.Stamp() != after {
		t.Error("stamp changed with nothing written")
	}
}
