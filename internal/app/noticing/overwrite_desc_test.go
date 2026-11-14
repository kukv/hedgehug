package noticing_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const descConflictText = "説明の同じ箇所が同時に編集されたため、両方を残しました"

// describe は、ref の説明を text にして desc の行を 1 つ足す保存の中身。
func describe(ref change.Ref, text string) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		if ref.Kind == change.KindProject {
			return errors.Join(tx.WriteProjects().Describe(ref.ID, text), tx.WriteProjects().Append(ref.ID, change.NewDesc()))
		}
		return errors.Join(tx.WriteItems().Describe(ref.ID, text), tx.WriteItems().Append(ref.ID, change.NewDesc()))
	}
}

// rowsOf は、保存 s の ref の行。
func rowsOf(s port.ReceivedSave, ref change.Ref) []change.Record {
	for _, e := range s.Entities {
		if e.Ref == ref {
			return e.Rows
		}
	}
	return nil
}

// descMerge は、ref の説明が重なった合わせ(版の印 vM)。side0・side1 は、その側の desc の行。
func descMerge(ref change.Ref, side0, side1 []change.Record) port.ReceivedSave {
	return port.ReceivedSave{
		Key: port.SaveKey{Version: "vM"}, Merge: true,
		Conflicts: []port.DescConflict{{Ref: ref, Sides: [2][]change.Record{side0, side1}}},
	}
}

func TestDescConflictNotice(t *testing.T) {
	f := setup(t)
	m := f.save(user("hanako"), describe(epicRef, "a\nx\n"))
	w := f.save(user("taro"), describe(epicRef, "a\ny\n"))
	merge := descMerge(epicRef, rowsOf(m, epicRef), rowsOf(w, epicRef))
	if err := f.judgeReceived(port.Received{Saves: []port.ReceivedSave{merge}, Own: ownOp(m)}); err != nil {
		t.Fatal(err)
	}
	want := port.Notice{
		Kind: port.NoticeOverwrite, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleEpic,
		Text: descConflictText, Display: "HH-1", Sender: "taro", Field: "desc",
		Source: "overwrite:desc:" + string(apptest.SampleEpic) + ":vM",
	}
	if n := f.wantOne(); !reflect.DeepEqual(n, want) {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
	// 同じ合わせを 2 回判定しても 1 行(C4b1-5 のやり直し)。
	if err := f.judgeReceived(port.Received{Saves: []port.ReceivedSave{merge}, Own: ownOp(m)}); err != nil {
		t.Fatal(err)
	}
	f.wantOne()
}

// このマシンの行が remote の側(合わせの 2 つ目の親)にあっても作り、差出人は、もう一方の側の at が最後の desc の行の人。
func TestDescConflictSenderIsLatestOfOtherSide(t *testing.T) {
	f := setup(t)
	taro := f.save(user("taro"), describe(projectRef, "t\n"))
	jiro := f.save(user("jiro"), describe(projectRef, "j\n"))
	m := f.save(user("hanako"), describe(projectRef, "h\n"))
	other := append(rowsOf(jiro, projectRef), rowsOf(taro, projectRef)...) // 並びは at の順でない
	merge := descMerge(projectRef, other, rowsOf(m, projectRef))
	if err := f.judgeReceived(port.Received{Saves: []port.ReceivedSave{merge}, Own: ownOp(m)}); err != nil {
		t.Fatal(err)
	}
	n := f.wantOne()
	if n.Sender != "jiro" || n.Project != apptest.SampleProject || n.Item != "" || n.Display != "HH-P1" || n.Field != "desc" {
		t.Errorf("notice = %+v", n)
	}
}

func TestDescConflictAutomationSender(t *testing.T) {
	f := setup(t)
	m := f.save(user("hanako"), describe(epicRef, "a\n"))
	w := f.save(user("taro"), func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(epicRef.ID, "b\n"), tx.WriteItems().Append(epicRef.ID, change.NewDesc().ByRule("01RULE0000000000000000001")))
	})
	merge := descMerge(epicRef, rowsOf(m, epicRef), rowsOf(w, epicRef))
	if err := f.judgeReceived(port.Received{Saves: []port.ReceivedSave{merge}, Own: ownOp(m)}); err != nil {
		t.Fatal(err)
	}
	if n := f.wantOne(); n.Sender != "" {
		t.Errorf("notice = %+v", n)
	}
}

func TestDescConflictSkips(t *testing.T) {
	missing := change.Ref{Kind: change.KindItem, ID: "01NOPE00000000000000000000"}
	cases := map[string]struct {
		readOnly bool
		noOwn    bool
		mineBy   string
		ref      *change.Ref
	}{
		"read only":      {readOnly: true},
		"not own":        {noOwn: true},
		"other person":   {mineBy: "jiro"},
		"missing entity": {ref: &missing},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			mineBy := "hanako"
			if c.mineBy != "" {
				mineBy = c.mineBy
			}
			m := f.save(user(mineBy), describe(epicRef, "a\n"))
			w := f.save(user("taro"), describe(epicRef, "b\n"))
			merge := descMerge(epicRef, rowsOf(m, epicRef), rowsOf(w, epicRef))
			if c.ref != nil {
				merge.Conflicts[0].Ref = *c.ref
			}
			in := port.Received{Saves: []port.ReceivedSave{merge}, ReadOnly: c.readOnly, Own: ownOp(m)}
			if c.noOwn {
				in.Own = nil
			}
			if err := f.judgeReceived(in); err != nil {
				t.Fatal(err)
			}
			f.wantNone()
		})
	}
}
