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

// Workspace は、土台が扱う項目だけを持つ。プロパティ・ワークフローなどはトラック B が足す。
type Workspace struct {
	ID     ID
	Name   string
	Prefix string
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

var opts = change.Options{
	Multi: func(f string) bool { return strings.HasSuffix(f, "/kinds") },
	Valid: valid,
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
	return w, true
}
