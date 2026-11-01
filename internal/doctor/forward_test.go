package doctor

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestRunForwardFiles(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	put(t, s, newTask("A"))
	good, bad, mismatch := model.NewULID(), model.NewULID(), model.NewULID()
	dest := "01JC0M3R5W7M2X9V4T6R5N8P1B"
	writeFile(t, dir, model.TaskPath(good), "---\nulid: "+good.String()+"\nmoved_to: "+dest+"\n---\n")
	writeFile(t, dir, model.TaskPath(bad), "---\nulid: "+bad.String()+"\nmoved_to: "+dest+"\ntitle: x\n---\n")
	writeFile(t, dir, model.TaskPath(mismatch), "---\nulid: "+good.String()+"\nmoved_to: "+dest+"\n---\n")
	fs := run(t, dir)
	for _, f := range fs {
		if f.Check == "file" && f.Where == model.TaskPath(good) {
			t.Errorf("a good forward was reported: %+v", f)
		}
	}
	if f := find(t, fs, "file", model.TaskPath(bad)); !strings.Contains(f.Msg, "title") {
		t.Errorf("broken forward = %+v", f)
	}
	if f := find(t, fs, "file", model.TaskPath(mismatch)); !strings.Contains(f.Msg, good.String()) {
		t.Errorf("mismatch = %+v", f)
	}
}
