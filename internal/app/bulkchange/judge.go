package bulkchange

import (
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 理由の文(F2b2-5)。
const (
	reasonReadOnly = "閲覧のみ"
	reasonDeleted  = "削除済み"
	reasonNoValue  = "値がない"
	reasonNoMember = "担当者がこの Store のメンバーにいません"
	reasonUnused   = "このプロジェクトでは使わないステータス"
	reasonDates    = "終了日が開始日より前になる"
)

// itemContext は、1 件の判定の材料。削除済み・存在しないアイテムは it に ID だけを入れて deleted にする。
type itemContext struct {
	it                item.Item
	readOnly, deleted bool
	settings          workspace.Settings
	rules             projectrule.Rules
	plan              planning.Plan
	members           []member.Member
}

// verdict は、1 件の判定。drafts も reason も空なら、値がすでに同じ。sprint は、スプリントへ移すときの
// 行き先(空はバックログ。順位は呼ぶ側が足す)。
type verdict struct {
	drafts []change.Draft
	reason string
	sprint *change.ID
}

// propertyKey は、項目が使う組み込みのプロパティのキー。スプリント・リリースは空(プロパティを持たない)。
func propertyKey(f Field) string {
	switch f {
	case FieldSprint, FieldRelease:
		return ""
	case FieldSystemsAdd, FieldSystemsRemove:
		return "systems"
	}
	return string(f)
}

// property は、組み込みのプロパティ key。
func (c itemContext) property(key string) workspace.Property {
	i := slices.IndexFunc(c.settings.Properties, func(p workspace.Property) bool { return p.Builtin && p.Key == key })
	if i < 0 {
		return workspace.Property{Key: key, Builtin: true}
	}
	return c.settings.Properties[i]
}

// kindReason は、アイテムの種類がその項目を持たない理由(F2b2-5。非表示は見ない)。
func (c itemContext) kindReason(f Field) string {
	kind := string(c.it.Kind)
	switch {
	case f == FieldSprint && !planning.IsWork(c.it.Kind):
		return kind + " はスプリントに入れない"
	case f == FieldRelease && c.it.Kind == item.Step:
		return "Step はリリースに入れない"
	}
	key := propertyKey(f)
	if key == "" {
		return ""
	}
	if p := c.property(key); p.Kinds != nil && !slices.Contains(p.Kinds, kind) {
		return kind + " には" + fieldName(f, true) + "がない"
	}
	return ""
}

// judge は、1 件のアイテムに in を適用した結果(F2b2-5〜F2b2-10)。理由は先に当たったもので残す。
func judge(c itemContext, in parsed) verdict {
	switch {
	case c.readOnly:
		return verdict{reason: reasonReadOnly}
	case c.deleted:
		return verdict{reason: reasonDeleted}
	}
	if r := c.kindReason(in.field); r != "" {
		return verdict{reason: r}
	}
	switch in.field {
	case FieldStatus:
		return judgeStatus(c, in)
	case FieldAssignee:
		return judgeAssignee(c, in)
	case FieldPriority:
		return judgePriority(c, in)
	case FieldSprint:
		return judgeSprint(c, in)
	case FieldRelease:
		return judgeRelease(c, in)
	case FieldSystemsAdd, FieldSystemsRemove:
		return judgeSystems(c, in)
	case FieldPoints, FieldEstimate:
		return judgeNumber(c, in)
	}
	return judgeDate(c, in)
}

// find は、名前 name の値の ID(F2b2-2。最初に一致したもの)。
func find(vs []workspace.Value, name string) (workspace.Value, bool) {
	i := slices.IndexFunc(vs, func(v workspace.Value) bool { return sameName(v.Name, name) })
	if i < 0 {
		return workspace.Value{}, false
	}
	return vs[i], true
}

// set は、組み込みのプロパティ key を to に変える 1 行(from はいまの値)。
func (c itemContext) set(key string, to any) verdict {
	p := c.property(key)
	return verdict{drafts: []change.Draft{change.NewSet(item.Field(p), item.Current(c.it, p), to)}}
}

// judgeStatus は、ステータスを名前か分類で変える(F2b2-5・F2b2-6)。値が同じなら数えない。
func judgeStatus(c itemContext, in parsed) verdict {
	var to workspace.Value
	if in.category != "" {
		if cur, ok := statusOf(c.settings, c.it.Status); ok && cur.Category == in.category {
			return verdict{}
		}
		id := c.settings.CategoryDefaults[in.category]
		v, ok := statusOf(c.settings, id)
		if !ok {
			return verdict{reason: reasonNoValue}
		}
		to = v
	} else {
		v, ok := find(c.settings.Statuses, in.text)
		if !ok {
			return verdict{reason: reasonNoValue}
		}
		to = v
	}
	switch {
	case to.ID == c.it.Status:
		return verdict{}
	case !projectrule.Uses(c.settings, c.rules, to.ID):
		return verdict{reason: reasonUnused}
	}
	if err := projectrule.CheckTransition(c.settings, c.rules, c.it.Status, to.ID); err != nil {
		return verdict{reason: err.Error()}
	}
	return c.set("status", to.ID)
}

// statusOf は、ID のステータス。
func statusOf(s workspace.Settings, id change.ID) (workspace.Value, bool) {
	i := slices.IndexFunc(s.Statuses, func(v workspace.Value) bool { return v.ID == id })
	if i < 0 {
		return workspace.Value{}, false
	}
	return s.Statuses[i], true
}

// judgePriority は、優先度を名前で変える。空は「なし」。
func judgePriority(c itemContext, in parsed) verdict {
	var to change.ID
	if in.text != "" {
		v, ok := find(c.settings.Priorities, in.text)
		if !ok {
			return verdict{reason: reasonNoValue}
		}
		to = v.ID
	}
	if to == c.it.Priority {
		return verdict{}
	}
	return c.set("priority", refValue(to))
}

// judgeAssignee は、担当者をログイン名で変える。書くのはメンバーのログイン名の表記。空は未割り当て。
func judgeAssignee(c itemContext, in parsed) verdict {
	if in.text == "" {
		if c.it.Assignee == "" {
			return verdict{}
		}
		return c.set("assignee", nil)
	}
	i := slices.IndexFunc(c.members, func(m member.Member) bool { return sameName(m.Login, in.text) })
	if i < 0 {
		return verdict{reason: reasonNoMember}
	}
	if sameName(c.it.Assignee, c.members[i].Login) {
		return verdict{}
	}
	return c.set("assignee", c.members[i].Login)
}

// refValue は、参照の値。空は null。
func refValue(id change.ID) any {
	if id == "" {
		return nil
	}
	return id
}
