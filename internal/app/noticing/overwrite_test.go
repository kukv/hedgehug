package noticing_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
)

const systemAPI change.ID = "01SAMPLEVAL000000000000012" // サンプルの関連システム API

// lose は、hanako(このマシン)が ref に mine を書き、それを見ずに同時に by が theirs を書いて、theirs を判定する。
func lose(f *fixture, ref change.Ref, mine []change.Draft, by change.Actor, theirs []change.Draft) port.ReceivedSave {
	f.t.Helper()
	m := f.save(user("hanako"), appendRef(ref, mine...))
	w := f.save(by, appendRef(ref, theirs...))
	f.overwrite(m, w)
	return w
}

// raw は、v のログに書く形の JSON。
func raw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// winnerOf は、保存 s の ref の最初の行。
func winnerOf(s port.ReceivedSave, ref change.Ref) change.Record {
	for _, e := range s.Entities {
		if e.Ref == ref {
			return e.Rows[0]
		}
	}
	return change.Record{}
}

func TestOverwriteItemStatus(t *testing.T) {
	f := setup(t)
	w := lose(f, epicRef, []change.Draft{setStatus(apptest.SampleStatusTodo, apptest.SampleStatusInProgress)},
		user("taro"), []change.Draft{setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone)})
	want := port.Notice{
		Kind: port.NoticeOverwrite, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleEpic,
		Text: "ステータスが上書きされました", Display: "HH-1", Sender: "taro", Field: "status",
		Mine: raw(t, apptest.SampleStatusInProgress), Kept: raw(t, apptest.SampleStatusDone),
		Source: "overwrite:" + string(apptest.SampleEpic) + ":status:" + string(winnerOf(w, epicRef).ID),
	}
	if n := f.wantOne(); !reflect.DeepEqual(n, want) {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestOverwriteTwoFields(t *testing.T) {
	f := setup(t)
	lose(f, epicRef,
		[]change.Draft{change.NewSet("title", "認証", "認証基盤"), setStatus(apptest.SampleStatusTodo, apptest.SampleStatusInProgress)},
		user("taro"),
		[]change.Draft{change.NewSet("title", "認証", "ログイン"), setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone)})
	if got, want := texts(f.notices()), []string{"ステータスが上書きされました", "タイトルが上書きされました"}; !reflect.DeepEqual(got, want) {
		t.Errorf("texts = %q, want %q", got, want)
	}
}

func TestOverwriteSystems(t *testing.T) {
	t.Run("one value", func(t *testing.T) {
		f := setup(t)
		f.save(user("taro"), appendRef(epicRef, change.NewAdd("systems", apptest.SampleSystemWeb), change.NewAdd("systems", systemAPI)))
		lose(f, epicRef, []change.Draft{change.NewRemove("systems", systemAPI)}, user("taro"), []change.Draft{change.NewAdd("systems", systemAPI)})
		n := f.wantOne()
		if n.Text != "関連システムが上書きされました" || n.Field != "systems" {
			t.Errorf("notice = %+v", n)
		}
		if mine, kept := string(n.Mine), string(raw(t, []change.ID{apptest.SampleSystemWeb, systemAPI})); mine != `["`+string(apptest.SampleSystemWeb)+`"]` || string(n.Kept) != kept {
			t.Errorf("mine = %s, kept = %s, want kept %s", n.Mine, n.Kept, kept)
		}
	})
	t.Run("two values are one row", func(t *testing.T) {
		f := setup(t)
		f.save(user("taro"), appendRef(epicRef, change.NewAdd("systems", apptest.SampleSystemWeb), change.NewAdd("systems", systemAPI)))
		both := []change.Draft{change.NewRemove("systems", apptest.SampleSystemWeb), change.NewRemove("systems", systemAPI)}
		lose(f, epicRef, both, user("taro"), []change.Draft{change.NewAdd("systems", apptest.SampleSystemWeb), change.NewAdd("systems", systemAPI)})
		if n := f.wantOne(); string(n.Mine) != "[]" {
			t.Errorf("mine = %s, want []", n.Mine)
		}
	})
}

// placeOf は、タスク HH-2 の場所(親を parent にしたもの)。
func placeOf(parent *item.ID) item.Place {
	return item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: parent, Number: 2}
}

func TestOverwriteMoveOnlyPlace(t *testing.T) {
	epic := apptest.SampleEpic
	t.Run("move", func(t *testing.T) {
		f := setup(t)
		lose(f, taskRef, []change.Draft{change.NewSet("place", placeOf(&epic), placeOf(nil)), change.NewSet("rank", "V", "a")},
			user("taro"), []change.Draft{change.NewSet("place", placeOf(&epic), placeOf(&epic)), change.NewSet("rank", "V", "b")})
		if n := f.wantOne(); n.Text != "場所が上書きされました" || n.Field != "place" {
			t.Errorf("notice = %+v", n)
		}
	})
	t.Run("rank only", func(t *testing.T) {
		f := setup(t)
		lose(f, taskRef, []change.Draft{change.NewSet("rank", "V", "a")}, user("taro"), []change.Draft{change.NewSet("rank", "V", "b")})
		if n := f.wantOne(); n.Text != "順位が上書きされました" {
			t.Errorf("notice = %+v", n)
		}
	})
}

func TestOverwriteSkips(t *testing.T) {
	title := func(v string) []change.Draft { return []change.Draft{change.NewSet("title", "認証", v)} }
	links := func(v string) []change.Draft {
		return []change.Draft{change.NewSet("links/01LINK0000000000000000001/state", "open", v)}
	}
	cases := map[string]struct {
		by       change.Actor
		mineBy   string
		mine     []change.Draft
		theirs   []change.Draft
		readOnly bool
		noOwn    bool
	}{
		"read only":  {by: user("taro"), mine: title("a"), theirs: title("b"), readOnly: true},
		"not own":    {by: user("taro"), mine: title("a"), theirs: title("b"), noOwn: true},
		"other mine": {by: user("taro"), mineBy: "jiro", mine: title("a"), theirs: title("b")},
		"github":     {by: change.Actor{Kind: change.ActorGitHub}, mine: title("a"), theirs: title("b")},
		"sync":       {by: change.Actor{Kind: change.ActorSync}, mine: title("a"), theirs: title("b")},
		"watchers": {
			by: user("taro"), mine: []change.Draft{change.NewRemove("watchers", "taro")},
			theirs: []change.Draft{change.NewAdd("watchers", "taro")},
		},
		"link state": {by: user("taro"), mine: links("merged"), theirs: links("closed")},
		"same value": {by: user("taro"), mine: title("a"), theirs: title("a")},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			mineBy := "hanako"
			if c.mineBy != "" {
				mineBy = c.mineBy
			}
			m := f.save(user(mineBy), appendRef(epicRef, c.mine...))
			w := f.save(c.by, appendRef(epicRef, c.theirs...))
			in := port.Received{Saves: []port.ReceivedSave{w}, ReadOnly: c.readOnly, Own: ownOp(m)}
			if c.noOwn {
				in.Own = nil
			}
			f.judgeOver(m, in)
			f.wantNone()
		})
	}
}

func TestOverwriteMineAutomation(t *testing.T) {
	f := setup(t)
	rule := change.Draft{T: change.Set, F: "title", From: change.Value("認証"), To: change.Value("a"), Rule: "01RULE0000000000000000001"}
	lose(f, epicRef, []change.Draft{rule}, user("taro"), []change.Draft{change.NewSet("title", "認証", "b")})
	if n := f.wantOne(); n.Sender != "taro" {
		t.Errorf("notice = %+v", n)
	}
}

func TestOverwriteAutomationSender(t *testing.T) {
	f := setup(t)
	rule := change.Draft{T: change.Set, F: "title", From: change.Value("認証"), To: change.Value("b"), Rule: "01RULE0000000000000000001"}
	lose(f, epicRef, []change.Draft{change.NewSet("title", "認証", "a")}, user("taro"), []change.Draft{rule})
	if n := f.wantOne(); n.Sender != "" || n.Text != "タイトルが上書きされました" {
		t.Errorf("notice = %+v", n)
	}
}

func TestOverwriteProject(t *testing.T) {
	t.Run("owner", func(t *testing.T) {
		f := setup(t)
		lose(f, projectRef, []change.Draft{change.NewSet("owner", "taro", "hanako")}, user("taro"), []change.Draft{change.NewSet("owner", "taro", "jiro")})
		n := f.wantOne()
		if n.Text != "オーナーが上書きされました" || n.Project != apptest.SampleProject || n.Display != "HH-P1" || n.Item != "" || n.Workspace != apptest.SampleWorkspace {
			t.Errorf("notice = %+v", n)
		}
	})
	t.Run("comment", func(t *testing.T) {
		f := setup(t)
		f.save(user("hanako"), appendRef(projectRef, change.NewComment(commentID, nil, comment.Value("最初"))))
		lose(f, projectRef, []change.Draft{change.NewComment(commentID, comment.Value("最初"), comment.Value("自分"))},
			user("taro"), []change.Draft{change.NewComment(commentID, comment.Value("最初"), comment.Value("太郎"))})
		n := f.wantOne()
		if n.Text != "コメントが上書きされました" || n.Project != apptest.SampleProject || n.Field != "comment:"+string(commentID) {
			t.Errorf("notice = %+v", n)
		}
	})
}

func TestOverwriteComment(t *testing.T) {
	f := setup(t)
	f.save(user("hanako"), writeComment("", "最初"))
	lose(f, taskRef, []change.Draft{change.NewComment(commentID, comment.Value("最初"), comment.Value("自分"))},
		user("taro"), []change.Draft{change.NewComment(commentID, comment.Value("最初"), comment.Value("太郎"))})
	want := port.Notice{
		Kind: port.NoticeOverwrite, Text: "コメントが上書きされました", Item: apptest.SampleTask, Field: "comment:" + string(commentID),
		Mine: raw(t, comment.Value("自分")), Kept: raw(t, comment.Value("太郎")),
	}
	n := f.wantOne()
	if n.Kind != want.Kind || n.Text != want.Text || n.Item != want.Item || n.Field != want.Field ||
		string(n.Mine) != string(want.Mine) || string(n.Kept) != string(want.Kept) {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestOverwriteDeletedCommentSkipped(t *testing.T) {
	f := setup(t)
	f.save(user("hanako"), writeComment("", "最初"))
	lose(f, taskRef, []change.Draft{change.NewComment(commentID, comment.Value("最初"), nil)},
		user("taro"), []change.Draft{change.NewComment(commentID, comment.Value("最初"), comment.Value("太郎"))})
	f.wantNone()
}

func TestOverwriteMemberName(t *testing.T) {
	f := setup(t)
	membersRef := change.Ref{Kind: change.KindMembers}
	m := f.save(user("hanako"), func(tx port.Tx) error { return tx.WriteMembers().Append(member.Rename("hanako", "hanako", "花子")) })
	w := f.save(user("taro"), func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Rename("hanako", "hanako", "はなこ"))
	})
	f.overwrite(m, w)
	want := port.Notice{
		Kind: port.NoticeOverwrite, Store: store, Text: "hanako の表示名が上書きされました", Sender: "taro", Field: "name/hanako",
		Mine: raw(t, "花子"), Kept: raw(t, "はなこ"),
		Source: "overwrite::name/hanako:" + string(winnerOf(w, membersRef).ID),
	}
	if n := f.wantOne(); !reflect.DeepEqual(n, want) {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestOverwriteOnce(t *testing.T) {
	f := setup(t)
	m := f.save(user("hanako"), appendRef(epicRef, change.NewSet("title", "認証", "a")))
	w := f.save(user("taro"), appendRef(epicRef, change.NewSet("title", "認証", "b")))
	f.overwrite(m, w)
	f.overwrite(m, w)
	f.wantOne()
}
