package transfer

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 移らないものの種類(A5b-8)。
const (
	NotCarriedHidden          = "hidden"
	NotCarriedHiddenFixed     = "hiddenFixed"
	NotCarriedWorkflow        = "workflow"
	NotCarriedTemplate        = "template"
	NotCarriedTemplateDefault = "templateDefault"
	NotCarriedDisabled        = "disabled"
)

// CarryInput は、プロジェクトのルールを移動先に引き継ぐ入力。
type CarryInput struct {
	Project      project.ID
	From, To     workspace.Settings
	Rules        projectrule.Rules       // 移動元のプロジェクト別のルール
	Own          []automation.Rule       // このプロジェクトの専用ルール
	StatusChoice map[change.ID]change.ID // 移動先にない移動元のステータス → 置き換え先
	People       PeopleMap               // nil なら同じ Store
	NewID        func() change.ID        // 専用ルールの新しい内部 ID
}

// NotCarried は、移動先に引き継げないルール。文は画面が作る。
type NotCarried struct {
	Kind    string
	Name    string
	Missing []string
	Count   int
}

// Carried は、引き継ぎの結果。
type Carried struct {
	Source      []change.Draft    // 移動元のワークスペースのログに書く、ルールを消す行
	Destination []change.Draft    // 移動先のワークスペースのログに書く行
	Created     []automation.Rule // 移動先で作る専用ルール
	Hidden      []change.ID       // 移動先で使わないにするステータス(書いたものだけ)
	NotCarried  []NotCarried
}

// RuleStatuses は、専用ルールが使うステータス(重複なし)。
func RuleStatuses(own []automation.Rule) []change.ID {
	var out []change.ID
	for _, r := range own {
		for _, id := range automation.Statuses(r) {
			if !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	return out
}

// CarryRules は、プロジェクト別のルールと専用ルールを、移動先のワークスペースの名前に合わせて引き継ぐ(03「移動・削除・復元」)。
func CarryRules(in CarryInput) Carried {
	c := &carrier{in: in}
	c.hidden()
	c.workflow()
	c.templates()
	c.templateDefaults()
	c.disabled()
	c.own()
	c.source()
	return c.out
}

type carrier struct {
	in  CarryInput
	out Carried
}

func (c *carrier) draft(d change.Draft) { c.out.Destination = append(c.out.Destination, d) }

func (c *carrier) notCarried(n NotCarried) { c.out.NotCarried = append(c.out.NotCarried, n) }

// toStatus は、移動先のステータスのうち、移動元のステータス id と名前が一致するもの。
func (c *carrier) toStatus(id change.ID) (change.ID, bool) {
	v, ok := valueByID(c.in.From.Statuses, id)
	if !ok {
		return "", false
	}
	return usableByName(c.in.To, projectrule.Rules{}, v.Name)
}

func (c *carrier) hidden() {
	for _, v := range c.in.From.Statuses { // 移動元のステータスの並び
		if !slices.Contains(c.in.Rules.Hidden, v.ID) {
			continue
		}
		to, ok := c.toStatus(v.ID)
		switch {
		case !ok:
			c.notCarried(NotCarried{Kind: NotCarriedHidden, Name: v.Name})
		case projectrule.IsFixedStatus(c.in.To, to):
			c.notCarried(NotCarried{Kind: NotCarriedHiddenFixed, Name: v.Name})
		case !slices.Contains(c.out.Hidden, to):
			c.out.Hidden = append(c.out.Hidden, to)
			c.draft(change.NewAdd(projectrule.HiddenField(c.in.Project), to))
		}
	}
}

func (c *carrier) workflow() {
	for _, from := range c.in.From.Statuses {
		for _, to := range c.in.Rules.Workflow[from.ID] {
			toValue, ok := valueByID(c.in.From.Statuses, to)
			if !ok {
				continue
			}
			c.transition(from, toValue)
		}
	}
}

// transition は、遷移 from → to を、両方の名前が移動先にあれば書き、なければ移らないものにする。
func (c *carrier) transition(from, to workspace.Value) {
	f, fok := c.toStatus(from.ID)
	t, tok := c.toStatus(to.ID)
	if fok && tok {
		c.draft(change.NewAdd(projectrule.WorkflowField(c.in.Project, f), t))
		return
	}
	var missing []string
	if !fok {
		missing = append(missing, from.Name)
	}
	if !tok {
		missing = append(missing, to.Name)
	}
	c.notCarried(NotCarried{Kind: NotCarriedWorkflow, Name: from.Name + " から " + to.Name, Missing: missing})
}

// toTemplate は、移動元のテンプレート id と名前・種類が一致する移動先のテンプレートの内部 ID。
// 移動元にないテンプレートは、ok=false で from の ID が空。
func (c *carrier) toTemplate(id change.ID) (to change.ID, from workspace.Template, ok bool) {
	from, found := c.in.From.Template(id)
	if !found {
		return "", workspace.Template{}, false
	}
	i := slices.IndexFunc(c.in.To.Templates, func(x workspace.Template) bool { return x.Name == from.Name && x.Kind == from.Kind })
	if i < 0 {
		return "", from, false
	}
	return c.in.To.Templates[i].ID, from, true
}

func (c *carrier) templates() {
	for _, t := range c.in.From.Templates { // 移動元のテンプレートの並び
		if !slices.Contains(c.in.Rules.Templates, t.ID) {
			continue
		}
		to, from, ok := c.toTemplate(t.ID)
		switch {
		case ok:
			c.draft(change.NewAdd(projectrule.TemplatesField(c.in.Project), to))
		case from.ID != "":
			c.notCarried(NotCarried{Kind: NotCarriedTemplate, Name: from.Name})
		}
	}
}

func (c *carrier) templateDefaults() {
	for _, kind := range workspace.Kinds { // 種類の並び
		v, ok := c.in.Rules.TemplateDefaults[kind]
		if !ok {
			continue
		}
		field := projectrule.TemplateDefaultField(c.in.Project, kind)
		if v == projectrule.None {
			c.draft(change.NewSet(field, nil, projectrule.None))
			continue
		}
		to, from, ok := c.toTemplate(change.ID(v))
		switch {
		case ok:
			c.draft(change.NewSet(field, nil, string(to)))
		case from.ID != "":
			c.notCarried(NotCarried{Kind: NotCarriedTemplateDefault, Name: kind, Missing: []string{from.Name}})
		}
	}
}

func (c *carrier) disabled() {
	if n := len(c.in.Rules.Disabled); n > 0 {
		c.notCarried(NotCarried{Kind: NotCarriedDisabled, Count: n})
	}
}

// statusTarget は、専用ルールが使う移動元のステータスの移動先。名前が移動先にあればそれ、なければ選んだ置き換え先、
// なければ同じ分類の移動先の既定。
func (c *carrier) statusTarget(id change.ID) change.ID {
	if to, ok := c.toStatus(id); ok {
		return to
	}
	if to, ok := c.in.StatusChoice[id]; ok {
		return to
	}
	if v, ok := valueByID(c.in.From.Statuses, id); ok {
		if d := c.in.To.CategoryDefaults[v.Category]; d != "" {
			return d
		}
	}
	return c.in.To.InitialStatus
}

// own は、専用ルールをすべて新しい内部 ID で移す。ルールは削除も無効化もしない(きっかけとすることが同じになっても)。
func (c *carrier) own() {
	for _, r := range c.in.Own {
		n := r
		n.ID = c.in.NewID()
		if n.Trigger == automation.TriggerStatus {
			n.TriggerStatus = c.statusTarget(r.TriggerStatus)
		}
		switch {
		case n.Action == automation.ActionStatus:
			n.Value = string(c.statusTarget(change.ID(r.Value)))
		case n.Action == automation.ActionAssignee && c.in.People != nil && n.Value != automation.Unassigned:
			if to, ok := c.in.People[strings.ToLower(n.Value)]; ok {
				n.Value = to
			}
		}
		c.out.Created = append(c.out.Created, n)
		c.out.Destination = append(c.out.Destination, automation.Drafts(automation.Rule{}, n)...)
	}
}

func (c *carrier) source() {
	for _, sec := range []projectrule.Section{
		projectrule.SectionStatuses, projectrule.SectionWorkflow, projectrule.SectionTemplates, projectrule.SectionAutomation,
	} {
		c.out.Source = append(c.out.Source, projectrule.ResetDrafts(c.in.Rules, c.in.Project, sec)...)
	}
	for _, r := range c.in.Own {
		c.out.Source = append(c.out.Source, automation.DeleteDraft(r.ID))
	}
}
