package ops

import "testing"

func TestShowAt(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	r, err := ts.Add(AddInput{Title: "orig", Body: "old body\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Set("ME-1", mustAssigns(t, "title=changed", "status=in_progress", "labels+=x")); err != nil {
		t.Fatal(err)
	}
	later, err := ts.Add(AddInput{Title: "added after"})
	if err != nil {
		t.Fatal(err)
	}

	v, err := ts.ShowAt("ME-1", r.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != "ME-1" || v.ULID != r.ULID.String() || v.Title != "orig" || v.Status != "todo" || v.Body != "old body\n" || len(v.Labels) != 0 {
		t.Errorf("ShowAt = %+v (body %q)", v.TaskView, v.Body)
	}
	if v.Comments == nil || v.History == nil || v.Reverse == nil || v.Watchers == nil || len(v.Comments)+len(v.History) != 0 {
		t.Errorf("comments, history and the rest are empty lists: %+v", v)
	}

	// The task did not exist yet at that commit.
	_, err = ts.ShowAt(later.ULID.String(), r.Commit)
	wantErr(t, err, KindNotFound, "restore_at.not_found")
	// Neither a time nor a commit.
	_, err = ts.ShowAt("ME-1", "yesterday-ish")
	wantErr(t, err, KindInvalid, "restore_at.bad_time")
}
