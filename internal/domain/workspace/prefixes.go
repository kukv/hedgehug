package workspace

import (
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// OldPrefixes は、旧い接頭辞(辞書順)。ログの create と set prefix の行に出た、使える形の接頭辞の
// すべてから、いまの接頭辞を除いたもの(保存形式「移動・番号・旧 ID」)。同期で負けた接頭辞も含む。
func OldPrefixes(log []change.Record) []string {
	w, ok := FromLog(log)
	if !ok {
		return nil
	}
	seen := map[string]bool{w.Prefix: true}
	var out []string
	for _, r := range log {
		p := prefixOf(r)
		if ValidPrefix(p) && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func prefixOf(r change.Record) string {
	switch {
	case r.T == change.Create:
		var v struct{ Prefix string }
		_ = json.Unmarshal(r.To, &v)
		return v.Prefix
	case r.T == change.Set && r.F == "prefix":
		var p string
		_ = json.Unmarshal(r.To, &p)
		return p
	}
	return ""
}
