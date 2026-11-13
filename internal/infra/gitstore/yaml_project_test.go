package gitstore

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// project.yaml には、結びつけたリポジトリを書き出す(読む用の写し。正本は log.jsonl)。
func TestProjectYAMLListsRepos(t *testing.T) {
	p := project.Project{ID: yProj, Place: project.Place{Workspace: yWS, Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	meta := change.Meta{Op: "o", At: time.Unix(0, 0), Actor: change.User("taro")}
	log := change.Stamp(nil, []change.Draft{project.Create(p), change.NewAdd("repos", "example-org/web")}, meta, yIDs())
	out, err := renderYAML(change.KindProject, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "repos: [example-org/web]") {
		t.Errorf("project.yaml =\n%s", out)
	}
}
