// Package archtest は、設計書 2 節の依存の向きと 4 節の大きさの上限を、テストで確かめる。
package archtest

import "strings"

const module = "github.com/kukv/hedgehug"

// unit は、依存の表の 1 行に当たるパッケージのまとまり。
// name は app・infra・adapter の下の名前(internal/app/items/move なら items)。
type unit struct {
	kind string // domain, port, apptest, app, infra, adapter, cmd, other, external
	name string
}

func unitOf(path string) unit {
	rel, ok := strings.CutPrefix(path, module+"/")
	if !ok {
		if path == module {
			return unit{kind: "other"}
		}
		return unit{kind: "external"}
	}
	p := strings.Split(rel, "/")
	switch {
	case p[0] == "cmd":
		return unit{kind: "cmd"}
	case len(p) >= 2 && p[0] == "internal" && p[1] == "domain":
		return unit{kind: "domain"}
	case len(p) >= 3 && p[0] == "internal" && p[1] == "app" && (p[2] == "port" || p[2] == "apptest"):
		return unit{kind: p[2]}
	case len(p) >= 3 && p[0] == "internal" && (p[1] == "app" || p[1] == "infra" || p[1] == "adapter"):
		return unit{kind: p[1], name: p[2]}
	}
	return unit{kind: "other"}
}

// isStd は、標準ライブラリのパッケージかを返す。標準ライブラリのパスは、最初の要素に「.」を含まない。
func isStd(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// Allowed は、from のパッケージが to を import してよいかを返す(設計書 2 節の表)。
// test は、テストのファイルからの import のとき true。テストも同じ表に従い、例外は apptest だけ。
func Allowed(from, to string, test bool) bool {
	f, t := unitOf(from), unitOf(to)
	if test && t.kind == "apptest" {
		return true
	}
	if t.kind == "external" {
		if isStd(to) {
			return true
		}
		return f.kind == "infra" || f.kind == "adapter" || f.kind == "cmd"
	}
	same := t.kind == f.kind && t.name == f.name
	switch f.kind {
	case "domain":
		return t.kind == "domain"
	case "port", "apptest":
		return t.kind == "domain" || t.kind == "port"
	case "app", "infra":
		return t.kind == "domain" || t.kind == "port" || same
	case "adapter":
		return t.kind == "domain" || t.kind == "port" || t.kind == "app" || same
	case "cmd":
		return true
	}
	return false
}
