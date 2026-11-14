package bulkchange

import (
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// judgeSprint は、スプリントを名前で変える。空はバックログ。使えないスプリントへの参照はバックログと
// 同じ値として扱う(F2b2-9)。完了したスプリントは、いま入っているときだけ行き先にできる。
func judgeSprint(c itemContext, in parsed) verdict {
	var to change.ID
	if in.text != "" {
		live := c.plan.LiveSprints()
		i := slices.IndexFunc(live, func(s planning.Sprint) bool {
			_, err := planning.AssignableSprint(c.plan, s.ID, c.it.Kind, c.it.Sprint)
			return sameName(s.Name, in.text) && err == nil
		})
		if i < 0 {
			return verdict{reason: reasonNoValue}
		}
		to = live[i].ID
	}
	_, usable := c.plan.UsableSprint(c.it.Sprint)
	if to == c.it.Sprint || to == "" && !usable {
		return verdict{}
	}
	d := change.NewSet(item.SprintField, refValue(c.it.Sprint), refValue(to))
	return verdict{drafts: []change.Draft{d}, sprint: &to}
}

// judgeRelease は、リリースを名前で変える。空はなし。使えないリリースへの参照は「なし」と同じ値として
// 扱う。アーカイブのリリースは、いま入っているときだけ行き先にできる。
func judgeRelease(c itemContext, in parsed) verdict {
	var to change.ID
	if in.text != "" {
		i := slices.IndexFunc(c.plan.LiveReleases(), func(r planning.Release) bool {
			return sameName(r.Name, in.text) && (r.State != planning.Archived || r.ID == c.it.Release)
		})
		if i < 0 {
			return verdict{reason: reasonNoValue}
		}
		to = c.plan.LiveReleases()[i].ID
	}
	_, usable := c.plan.UsableRelease(c.it.Release)
	if to == c.it.Release || to == "" && !usable {
		return verdict{}
	}
	return verdict{drafts: []change.Draft{change.NewSet(item.ReleaseField, refValue(c.it.Release), refValue(to))}}
}

// judgeSystems は、関連システムを足す・外す(F2b2-9)。足すのは持たない値だけ、外すのは持つ値だけを
// 値ごとに 1 行で書く。足す値に 1 つでもない名前があれば「値がない」。外す名前のうちない値は、持たない
// ものとして飛ばす。
func judgeSystems(c itemContext, in parsed) verdict {
	add := in.field == FieldSystemsAdd
	var ids []change.ID
	for _, name := range in.names {
		v, ok := find(c.settings.Systems, name)
		switch {
		case !ok && add:
			return verdict{reason: reasonNoValue}
		case ok && !slices.Contains(ids, v.ID):
			ids = append(ids, v.ID)
		}
	}
	var out verdict
	for _, id := range ids {
		has := slices.Contains(c.it.Systems, id)
		switch {
		case add && !has:
			out.drafts = append(out.drafts, change.NewAdd("systems", id))
		case !add && has:
			out.drafts = append(out.drafts, change.NewRemove("systems", id))
		}
	}
	return out
}

// judgeNumber は、ポイントか見積もり時間を変える。空は値の削除。
func judgeNumber(c itemContext, in parsed) verdict {
	key, to, cur := "points", in.points, c.it.Points
	if in.field == FieldEstimate {
		key, to, cur = "estimate", in.estimate, c.it.Estimate
	}
	if (to == nil && cur == nil) || (to != nil && cur != nil && *to == *cur) {
		return verdict{}
	}
	return c.set(key, to)
}

// judgeDate は、開始日か終了日を変える。空は値の削除。もう片方と比べて、終了日が開始日より前になるなら断る。
func judgeDate(c itemContext, in parsed) verdict {
	key, cur := "end", c.it.End
	start, end := c.it.Start, in.date
	if in.field == FieldStart {
		key, cur = "start", c.it.Start
		start, end = in.date, c.it.End
	}
	if in.date == cur {
		return verdict{}
	}
	if item.CheckDates(start, end) != nil {
		return verdict{reason: reasonDates}
	}
	if in.date == "" {
		return c.set(key, nil)
	}
	return c.set(key, in.date)
}
