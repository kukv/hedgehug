package inbox

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// itemReason は、アイテム it の項目 f を mine に戻せない理由(C4c2-6。上から最初の 1 つ)。cur はいまのワークスペース。
// 戻せるなら空。
func itemReason(it item.Item, f itemField, cur wsView, mine json.RawMessage, ms []member.Member) string {
	switch f.kind {
	case fieldSprint:
		return sprintReason(it, planning.FromLog(cur.log), cur.log, mine)
	case fieldRelease:
		return releaseReason(it, planning.FromLog(cur.log), cur.log, mine)
	case fieldProperty:
		return propertyReason(it, f.prop, cur, mine, ms)
	}
	return ""
}

func deletedReason(name string) string { return "「" + name + "」は削除されています" }

// sprintReason は、スプリントの理由。使えない → 完了していていまのスプリントでない → 作業項目でない、の順。
func sprintReason(it item.Item, p planning.Plan, log []change.Record, mine json.RawMessage) string {
	if id := change.ID(textOf(mine)); id != "" {
		sp, ok := p.UsableSprint(id)
		if !ok {
			return deletedReason(showPlan(mine, log, "sprints/"))
		}
		if sp.State == planning.Closed && it.Sprint != id {
			return "スプリント「" + sp.Name + "」は完了しています"
		}
	}
	if !planning.IsWork(it.Kind) {
		return string(it.Kind) + " はスプリントに入れられません"
	}
	return ""
}

// releaseReason は、リリースの理由。使えない → アーカイブでいまのリリースでない → Step、の順。
func releaseReason(it item.Item, p planning.Plan, log []change.Record, mine json.RawMessage) string {
	if id := change.ID(textOf(mine)); id != "" {
		rel, ok := p.UsableRelease(id)
		if !ok {
			return deletedReason(showPlan(mine, log, "releases/"))
		}
		if rel.State == planning.Archived && it.Release != id {
			return "リリース「" + rel.Name + "」はアーカイブされています"
		}
	}
	if it.Kind == item.Step {
		return "Step はリリースに入れられません"
	}
	return ""
}

// propertyReason は、プロパティの理由。値がいまの設定にない・ユーザーがメンバーにいない・開始日と終了日が合わない。
func propertyReason(it item.Item, p workspace.Property, cur wsView, mine json.RawMessage, ms []member.Member) string {
	switch p.Type {
	case workspace.TypeUser:
		if login := textOf(mine); login != "" && !isMember(ms, login) {
			return p.Name + " " + login + " はこの Store のメンバーにいません"
		}
	case workspace.TypeDate:
		if err := checkItemDates(it, p, textOf(mine)); err != nil {
			return err.Error()
		}
	default:
		return missingValue(p, cur, mine)
	}
	return ""
}

// missingValue は、値の ID を持つ種類で、mine のうち最初にいまの設定にない値の理由。ほかの種類と、すべてあるときは空。
func missingValue(p workspace.Property, cur wsView, mine json.RawMessage) string {
	st := cur.st
	vals, prefix, set := valueList(p, &st)
	if set == nil {
		return ""
	}
	ids, _ := idsOf(mine)
	for _, id := range ids {
		if slices.ContainsFunc(vals, func(v workspace.Value) bool { return v.ID == id }) {
			continue
		}
		name := workspace.NameOfDeleted(cur.log, prefix+string(id))
		if name == "" {
			name = deletedValueText
		}
		return deletedReason(name)
	}
	return ""
}

// checkItemDates は、組み込みの開始日か終了日を v にするとき、いまのもう片方と比べる(items の checkDatePair と同じ)。
func checkItemDates(it item.Item, p workspace.Property, v string) error {
	switch {
	case !p.Builtin:
		return nil
	case p.Key == "start":
		return item.CheckDates(v, it.End)
	case p.Key == "end":
		return item.CheckDates(it.Start, v)
	}
	return nil
}

// projectReason は、プロジェクト p の項目 f を mine に戻せない理由。タイトルは通常の保存と同じ検査(同じワークスペースの
// 生きているプロジェクトと同じ名前・空)、オーナーはメンバーにいない、日付は開始日と終了日が合わない。
func projectReason(r port.Reader, p project.Project, f, mine string, ms []member.Member) (string, error) {
	var err error
	switch f {
	case "title":
		siblings, rerr := port.LiveProjects(r, p.Place.Workspace)
		if rerr != nil {
			return "", rerr
		}
		err = project.CheckRename(strings.TrimSpace(mine), p.ID, siblings)
	case "owner":
		if mine != "" && !isMember(ms, mine) {
			return "オーナー " + mine + " はこの Store のメンバーにいません", nil
		}
	case "start":
		err = project.CheckDates(mine, p.End)
	case "end":
		err = project.CheckDates(p.Start, mine)
	}
	if err != nil {
		return err.Error(), nil
	}
	return "", nil
}
