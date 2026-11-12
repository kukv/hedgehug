package gitstore

import (
	"bytes"
	"encoding/json"
	"path"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
)

// tree は、Store のファイル。キーは Store の根からの相対パス(区切りは /)、値は中身。
type tree map[string][]byte

// mergeTrees は、共通の祖先 base と 2 つの版 a・b を合わせる(保存形式「同期」の手順 3)。
// 入出力をせず、a と b を入れ替えても同じ結果になる。
func mergeTrees(base, a, b tree) (tree, error) {
	gone := removedEntities(base, a, b)
	out := tree{}
	for _, p := range sortedKeys(base, a, b) {
		if gone[entityDir(p)] || isEntityYAML(p) {
			continue
		}
		if v, ok := mergeFile(p, base, a, b); ok {
			out[p] = v
		}
	}
	removeDirs(out, underTransferred(out))
	if err := rebuildYAML(out); err != nil {
		return nil, err
	}
	return out, nil
}

// entityDir は、p が入っているエンティティのフォルダ(items/<ID> など)。エンティティのファイルでなければ空。
func entityDir(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) >= 3 && (parts[0] == "items" || parts[0] == "projects" || parts[0] == "workspaces") {
		return parts[0] + "/" + parts[1]
	}
	return ""
}

// isEntityYAML は、log.jsonl から作り直す読む用の YAML か。
func isEntityYAML(p string) bool {
	dir, name := path.Split(p)
	for _, k := range kindDirs {
		if name != k.yaml {
			continue
		}
		if k.yaml == "members.yaml" {
			return dir == "members/"
		}
		return entityDir(p) != "" && strings.HasPrefix(dir, k.dir+"/")
	}
	return false
}

// removedEntities は、共通の祖先にあって、どちらかの版で消えたエンティティのフォルダ。
func removedEntities(base, a, b tree) map[string]bool {
	gone := map[string]bool{}
	inA, inB := dirSet(a), dirSet(b)
	for p := range base {
		d := entityDir(p)
		if d != "" && (!inA[d] || !inB[d]) {
			gone[d] = true
		}
	}
	return gone
}

// dirSet は、ツリーにファイルがあるエンティティのフォルダの集まり。
func dirSet(t tree) map[string]bool {
	dirs := map[string]bool{}
	for p := range t {
		if d := entityDir(p); d != "" {
			dirs[d] = true
		}
	}
	return dirs
}

// removeDirs は、dirs のフォルダの下のファイルを全部消す。
func removeDirs(t tree, dirs []string) {
	if len(dirs) == 0 {
		return
	}
	set := map[string]bool{}
	for _, d := range dirs {
		set[d] = true
	}
	for p := range t {
		if set[entityDir(p)] {
			delete(t, p)
		}
	}
}

// mergeFile は、1 つのファイルを合わせる。結果にファイルが残らなければ false。
func mergeFile(p string, base, a, b tree) ([]byte, bool) {
	name := path.Base(p)
	switch {
	case name == "log.jsonl" || p == "transfers.jsonl" || p == "purged.jsonl":
		_, inA := a[p]
		_, inB := b[p]
		if !inA && !inB {
			return nil, false
		}
		return filemerge.MergeLog(a[p], b[p]), true
	case name == "description.md" && entityDir(p) != "":
		_, inA := a[p]
		_, inB := b[p]
		if !inA && !inB {
			return nil, false
		}
		return []byte(mergeDescription(p, base, a, b)), true
	}
	return mergeOther(p, base, a, b)
}

// mergeOther は、そのほかのファイルを合わせる(この計画の決定 6)。
func mergeOther(p string, base, a, b tree) ([]byte, bool) {
	vb, inBase := base[p]
	va, inA := a[p]
	vbb, inB := b[p]
	sameA := inA == inBase && bytes.Equal(va, vb)
	sameB := inB == inBase && bytes.Equal(vbb, vb)
	switch {
	case sameA:
		return vbb, inB
	case sameB:
		return va, inA
	case !inA:
		return vbb, inB
	case !inB:
		return va, true
	case bytes.Compare(va, vbb) >= 0:
		return va, true
	}
	return vbb, true
}

// sortedKeys は、ツリーのパスの和を、辞書順で返す。
func sortedKeys(ts ...tree) []string {
	seen := map[string]bool{}
	for _, t := range ts {
		for p := range t {
			seen[p] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// parseLog は、log.jsonl の中身を変更記録にする。形の違う行は読み飛ばす。
func parseLog(b []byte) []change.Record {
	var out []change.Record
	for _, l := range bytes.Split(b, []byte("\n")) {
		var r change.Record
		if len(bytes.TrimSpace(l)) > 0 && json.Unmarshal(l, &r) == nil {
			out = append(out, r)
		}
	}
	return out
}
