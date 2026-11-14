package noticing

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// droppedPlans は、i 番目が合わせなら、合わせで取り除かれたスプリント・リリースのうち、共通の祖先にない行(New)に
// あなたの行があるものごとの通知(06「完全な削除と重なったとき」。C5a-8・C5a-9)。取り除く前のログ(Log)だけに
// ある行は数えない。
func droppedPlans(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	s := b.saves[i]
	if !s.Merge {
		return nil, nil
	}
	var out []port.Notice
	for _, d := range s.DroppedPlan {
		if !wroteBy(d.New, me) {
			continue
		}
		n, ok, err := droppedPlanNotice(r, b, d)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// droppedPlanNotice は、取り除かれた d の通知。名前は purged.jsonl の display、なければ d.Log の名前。どちらもなければ
// false。
func droppedPlanNotice(r port.Reader, b batch, d port.DroppedPlan) (port.Notice, bool, error) {
	cs, err := b.causes(r)
	if err != nil {
		return port.Notice{}, false, err
	}
	name := logName(d)
	if j := slices.IndexFunc(cs.purged, func(p port.Purged) bool {
		return p.Target == d.Target && (p.Entity == "sprint" || p.Entity == "release")
	}); j >= 0 {
		if d := cs.purged[j].Display; d != "" {
			name = d
		}
	}
	if oneLine(name) == "" {
		return port.Notice{}, false, nil
	}
	// Log は d.Target の行だけなので、sprints/ の行があればスプリント、なければリリース。
	section := "releases"
	if slices.ContainsFunc(d.Log, func(row change.Record) bool { return strings.HasPrefix(row.F, sprintsPrefix) }) {
		section = "sprints"
	}
	return port.Notice{
		Workspace: d.Workspace, Section: section,
		Text:   purgedText("「" + oneLine(name) + "」"),
		Source: "repair:purged:" + string(d.Target),
	}, true, nil
}

// logName は、d.Log の name の行(スプリント・リリースのどちらか)のいまの値。Log は Target の行だけで作成の行を
// 持たないので、planning.FromLog ではなく項目ごとに読む。なければ空。
func logName(d port.DroppedPlan) string {
	all := func(change.Record) bool { return true }
	for _, prefix := range []string{sprintsPrefix, releasesPrefix} {
		row, ok := change.Current(d.Log, change.Set, prefix+string(d.Target)+"/name", all)
		var name string
		if ok && json.Unmarshal(row.To, &name) == nil && name != "" {
			return name
		}
	}
	return ""
}
