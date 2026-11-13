package projectrule

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Uses は、プロジェクトがステータス status を使うか(B5a-5)。外していないか、初期ステータスか、
// 分類ごとの既定のどれかなら使う(外した印は残すが、使うものとして扱う)。
func Uses(s workspace.Settings, r Rules, status change.ID) bool {
	if !slices.Contains(r.Hidden, status) || s.InitialStatus == status {
		return true
	}
	for _, id := range s.CategoryDefaults {
		if id == status {
			return true
		}
	}
	return false
}

// CheckUse は、使わないステータスなら断る。
func CheckUse(s workspace.Settings, r Rules, status change.ID) error {
	if Uses(s, r, status) {
		return nil
	}
	return refusal.New("このプロジェクトでは「" + workspace.StatusName(s, status) + "」を使いません")
}

// UsedStatuses は、使うステータスを定義の順に返す。
func UsedStatuses(s workspace.Settings, r Rules) []workspace.Value {
	var out []workspace.Value
	for _, v := range s.Statuses {
		if Uses(s, r, v.ID) {
			out = append(out, v)
		}
	}
	return out
}

// projectTargets は、from の行のプロジェクトのチェックのうち、From と To の両方が使うステータスで、ワークスペースの
// 制約も許すもの。ワークスペースの制約が許さないセルのチェック(ワークスペースの制約を後から変えて残ったもの)は
// 数えない(03「プロジェクトの遷移の制約」)。
func projectTargets(s workspace.Settings, r Rules, from change.ID) []change.ID {
	if !Uses(s, r, from) {
		return nil
	}
	ws, wsRestricted := s.Allowed(from)
	var to []change.ID
	for _, v := range s.Statuses {
		if v.ID != from && Uses(s, r, v.ID) && slices.Contains(r.Workflow[from], v.ID) &&
			(!wsRestricted || slices.Contains(ws, v.ID)) {
			to = append(to, v.ID)
		}
	}
	return to
}

// Allowed は、from から変更できる To をステータスの並びで返す(B5a-6)。ワークスペースの許す To と
// プロジェクトの許す To の両方に入るもので、使わないステータスと from は除く。両方の行に制限がなければ
// false(使う先すべて)。
func Allowed(s workspace.Settings, r Rules, from change.ID) (to []change.ID, restricted bool) {
	ws, wsRestricted := s.Allowed(from)
	pr := projectTargets(s, r, from)
	prRestricted := len(pr) > 0
	for _, v := range s.Statuses {
		if v.ID == from || !Uses(s, r, v.ID) {
			continue
		}
		if (!wsRestricted || slices.Contains(ws, v.ID)) && (!prRestricted || slices.Contains(pr, v.ID)) {
			to = append(to, v.ID)
		}
	}
	return to, wsRestricted || prRestricted
}

// CheckTransition は、from から to への変更を、ワークスペースとプロジェクトの両方の遷移の制約で検査する
// (B5a-6)。from が空・同じ・いまの一覧にないとき、両方の行に制限がないときは通す。
func CheckTransition(s workspace.Settings, r Rules, from, to change.ID) error {
	if from == "" || from == to || !slices.ContainsFunc(s.Statuses, func(v workspace.Value) bool { return v.ID == from }) {
		return nil
	}
	allowed, restricted := Allowed(s, r, from)
	if !restricted || slices.Contains(allowed, to) {
		return nil
	}
	tail := "変更できる先がありません"
	if len(allowed) > 0 {
		names := make([]string, 0, len(allowed))
		for _, id := range allowed {
			names = append(names, workspace.StatusName(s, id))
		}
		tail = strings.Join(names, "・") + " のみ"
	}
	return refusal.New(fmt.Sprintf("ワークフローの制約: %s から %s には変更できません(%s)",
		workspace.StatusName(s, from), workspace.StatusName(s, to), tail))
}

// UsableTemplates は、種類 kind のテンプレートのうち、プロジェクトで使えるもの(B5a-9)。プロジェクトの
// templates のうち、いまあるテンプレートが 1 件もなければ、種類の合うすべて。
func UsableTemplates(s workspace.Settings, r Rules, kind string) []workspace.Template {
	narrowed := slices.ContainsFunc(r.Templates, func(id change.ID) bool { _, ok := s.Template(id); return ok })
	var out []workspace.Template
	for _, t := range s.Templates {
		if t.Kind == kind && (!narrowed || slices.Contains(r.Templates, t.ID)) {
			out = append(out, t)
		}
	}
	return out
}

// DefaultTemplate は、種類 kind の既定のテンプレート(B5a-9)。プロジェクトの既定が None なら「なし」、
// テンプレート ID ならそれが使えればそれ、使えなければ「なし」(ワークスペースの既定には戻らない)、
// 未設定ならワークスペースの既定が使えるならそれ、なければ「なし」。
func DefaultTemplate(s workspace.Settings, r Rules, kind string) (change.ID, bool) {
	usable := UsableTemplates(s, r, kind)
	has := func(id change.ID) bool {
		return slices.ContainsFunc(usable, func(t workspace.Template) bool { return t.ID == id })
	}
	switch own := r.TemplateDefaults[kind]; {
	case own == None:
		return "", false
	case own != "":
		if has(change.ID(own)) {
			return change.ID(own), true
		}
		return "", false
	}
	if id := s.TemplateDefaults[kind]; id != "" && has(id) {
		return id, true
	}
	return "", false
}
