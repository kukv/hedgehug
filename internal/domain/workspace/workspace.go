// Package workspace は、ワークスペース(接頭辞を持ち、プロパティ・ステータス・ワークフローなどを持つ単位)を持つ。
package workspace

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// ID はワークスペースの内部 ID。
type ID = change.ID

// Workspace は、ワークスペースの名前と接頭辞。プロパティ・ワークフローなどの設定は Settings が持つ。
type Workspace struct {
	ID     ID
	Name   string
	Prefix string
	// Deleted は、削除済みの印(B5b)。保存の口は削除済みも返し、除くのは app の側で Live を通す。
	Deleted *change.Deletion
}

// Live は、削除済みを除いたワークスペースを、順を保って返す。
func Live(ws []Workspace) []Workspace {
	out := make([]Workspace, 0, len(ws))
	for _, w := range ws {
		if w.Deleted == nil {
			out = append(out, w)
		}
	}
	return out
}

var prefixPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,9}$`)

// ValidPrefix は、接頭辞に使えるか(英大文字で始まる 10 文字以内の英大文字と数字。01)。
func ValidPrefix(s string) bool { return prefixPattern.MatchString(s) }

// Create は、ワークスペースの作成の変更。名前が空、または接頭辞が使えなければ断る。
func Create(w Workspace) (change.Draft, error) {
	if strings.TrimSpace(w.Name) == "" {
		return change.Draft{}, errors.New("workspace name is empty")
	}
	if !ValidPrefix(w.Prefix) {
		return change.Draft{}, errors.New("workspace prefix is not valid")
	}
	return change.NewCreate(map[string]any{"id": w.ID, "name": w.Name, "prefix": w.Prefix}), nil
}

var opts = change.Options{Multi: IsMulti, Valid: valid}

// IsMulti は、ワークスペースのログで f が値を複数持つ項目(add・remove)か。
func IsMulti(f string) bool {
	return strings.HasSuffix(f, "/kinds") || strings.HasPrefix(f, workflowPrefix) || projectRuleList(f)
}

// projectRuleList は、プロジェクト別のルールの add・remove の項目か。
// projectrule は workspace を使うので import できず、projectrule.IsMulti と同じ判定をここに持つ(テストで一致を確かめる)。
func projectRuleList(f string) bool {
	body, ok := strings.CutPrefix(f, "project_rules/")
	if !ok {
		return false
	}
	parts := strings.Split(body, "/")
	switch {
	case len(parts) == 2:
		return parts[0] != "" && (parts[1] == "hidden_statuses" || parts[1] == "templates" || parts[1] == "disabled_rules")
	case len(parts) == 3:
		return parts[0] != "" && parts[1] == "workflow" && parts[2] != ""
	}
	return false
}

func valid(r change.Record) bool {
	switch {
	case r.T == change.Set && r.F == "prefix":
		var p string
		return json.Unmarshal(r.To, &p) == nil && ValidPrefix(p)
	case r.T == change.Create:
		var v struct{ Prefix string }
		return json.Unmarshal(r.To, &v) == nil && ValidPrefix(v.Prefix)
	case r.T == change.Set && strings.HasSuffix(r.F, "/color"):
		var c Color
		return json.Unmarshal(r.To, &c) == nil && validColor(c)
	case r.T == change.Set && strings.HasSuffix(r.F, "/category"):
		var c Category
		return json.Unmarshal(r.To, &c) == nil && validCategory(c)
	}
	return true
}

// FromLog は、ワークスペースのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Workspace, bool) {
	s := change.Resolve(log, opts)
	if s.Created == nil {
		return Workspace{}, false
	}
	var w Workspace
	_ = json.Unmarshal(s.Values["id"], &w.ID)
	_ = json.Unmarshal(s.Values["name"], &w.Name)
	_ = json.Unmarshal(s.Values["prefix"], &w.Prefix)
	w.Deleted = change.DeletionOf(log)
	return w, true
}
