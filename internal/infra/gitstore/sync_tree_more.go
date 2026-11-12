package gitstore

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
)

// descType は、説明を保存した変更記録の種類(保存形式「変更記録」の desc)。型はトラック A が足す。
const descType change.Type = "desc"

// mergeDescription は、説明を行ごとに合わせる。保存の古い版を先にする(この計画の決定 8)。
func mergeDescription(p string, base, a, b tree) string {
	logPath := entityDir(p) + "/log.jsonl"
	textA, textB := string(a[p]), string(b[p])
	atA := lastDesc(a[logPath], base[logPath])
	atB := lastDesc(b[logPath], base[logPath])
	aOlder := atA.Before(atB) || (atA.Equal(atB) && textA <= textB)
	if aOlder {
		return filemerge.MergeText(string(base[p]), textA, textB)
	}
	return filemerge.MergeText(string(base[p]), textB, textA)
}

// lastDesc は、side で足された(base にない)desc の行のうち、最後の時刻。
func lastDesc(side, base []byte) time.Time {
	inBase := map[change.ID]bool{}
	for _, r := range parseLog(base) {
		inBase[r.ID] = true
	}
	var last time.Time
	for _, r := range parseLog(side) {
		if r.T == descType && !inBase[r.ID] && r.At.After(last) {
			last = r.At
		}
	}
	return last
}

// underTransferred は、別の Store へ移ったもの(transfers.jsonl にあり、フォルダがない)の下にある
// エンティティのフォルダを返す(保存形式「同期」の手順 3 の最後)。消す前に全部を決める。
func underTransferred(t tree) []string {
	targets := transferTargets(t["transfers.jsonl"])
	if len(targets) == 0 {
		return nil
	}
	var out []string
	for _, p := range sortedKeys(t) {
		d := entityDir(p)
		if d == "" || !strings.HasSuffix(p, "/log.jsonl") {
			continue
		}
		for _, anc := range ancestors(t, d) {
			if targets[anc] && !hasDir(t, dirOfAncestor(t, anc)) {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

func transferTargets(b []byte) map[string]bool {
	targets := map[string]bool{}
	for _, l := range bytes.Split(b, []byte("\n")) {
		var v struct {
			Target string `json:"target"`
		}
		if json.Unmarshal(l, &v) == nil && v.Target != "" {
			targets[v.Target] = true
		}
	}
	return targets
}

// ancestors は、エンティティの祖先の内部 ID(親のアイテムを辿り、プロジェクトとワークスペースまで)。
// 祖先のフォルダがなければ、そこで辿るのをやめる。
func ancestors(t tree, d string) []string {
	var out []string
	seen := map[string]bool{}
	for d != "" && !seen[d] {
		seen[d] = true
		log := parseLog(t[d+"/log.jsonl"])
		switch {
		case strings.HasPrefix(d, "items/"):
			it, ok := item.FromLog(log)
			if !ok {
				return out
			}
			out = append(out, string(it.Place.Workspace), string(it.Place.Project))
			if it.Place.Parent == nil {
				return out
			}
			out = append(out, string(*it.Place.Parent))
			d = "items/" + string(*it.Place.Parent)
		case strings.HasPrefix(d, "projects/"):
			if p, ok := project.FromLog(log); ok {
				out = append(out, string(p.Place.Workspace))
			}
			return out
		default:
			return out
		}
	}
	return out
}

// dirOfAncestor は、祖先の ID のフォルダ。アイテム・プロジェクト・ワークスペースのどれかにあればそのフォルダ、
// どれにもなければ、アイテムのフォルダの名前を返す(どれも無いので、hasDir は false になる)。
func dirOfAncestor(t tree, id string) string {
	for _, k := range []string{"items/", "projects/", "workspaces/"} {
		if hasDir(t, k+id) {
			return k + id
		}
	}
	return "items/" + id
}

// rebuildYAML は、合わせた log.jsonl から、読む用の YAML を作り直す。
func rebuildYAML(t tree) error {
	for _, p := range sortedKeys(t) {
		if !strings.HasSuffix(p, "/log.jsonl") {
			continue
		}
		dir := strings.TrimSuffix(p, "/log.jsonl")
		kind, ok := kindOfDir(dir)
		if !ok {
			continue
		}
		y, err := renderYAML(kind, parseLog(t[p]))
		if err != nil {
			return err
		}
		if y != nil {
			t[dir+"/"+kindDirs[kind].yaml] = y
		}
	}
	return nil
}

// kindOfDir は、フォルダ(members、items/<ID> など)の種類。
func kindOfDir(dir string) (change.Kind, bool) {
	top, _, _ := strings.Cut(dir, "/")
	for kind, k := range kindDirs {
		if k.dir == top && (kind == change.KindMembers) == (dir == "members") {
			return kind, true
		}
	}
	return "", false
}
