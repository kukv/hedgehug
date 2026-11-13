package transfer_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const proj = "PRJ"

func st(id, name string, c workspace.Category) workspace.Value {
	return workspace.Value{ID: change.ID(id), Name: name, Category: c}
}

// ruleSettings は、移動元 F(Todo・InProgress・Review・Pending・Done)と、移動先 T(Todo・InProgress・Done)。
// 名前は同じで内部 ID は違う。移動先の初期ステータスは Todo、既定は未着手が Todo・完了が Done(進行中の既定はなし)。
func ruleSettings() (from, to workspace.Settings) {
	from = workspace.Settings{
		Statuses: []workspace.Value{
			st("FTodo", "Todo", workspace.CategoryTodo), st("FProg", "InProgress", workspace.CategoryDoing),
			st("FRev", "Review", workspace.CategoryDoing), st("FPend", "Pending", workspace.CategoryTodo),
			st("FDone", "Done", workspace.CategoryDone),
		},
		InitialStatus: "FTodo",
		CategoryDefaults: map[workspace.Category]change.ID{
			workspace.CategoryTodo: "FTodo", workspace.CategoryDoing: "FProg", workspace.CategoryDone: "FDone",
		},
		Templates: []workspace.Template{
			{ID: "FT1", Name: "Bug 報告", Kind: "Bug"}, {ID: "FT2", Name: "仕様", Kind: "Story"},
		},
	}
	to = workspace.Settings{
		Statuses: []workspace.Value{
			st("TTodo", "Todo", workspace.CategoryTodo), st("TProg", "InProgress", workspace.CategoryDoing),
			st("TDone", "Done", workspace.CategoryDone),
		},
		InitialStatus: "TTodo",
		CategoryDefaults: map[workspace.Category]change.ID{
			workspace.CategoryTodo: "TTodo", workspace.CategoryDone: "TDone",
		},
		Templates: []workspace.Template{{ID: "TT1", Name: "仕様", Kind: "Story"}},
	}
	return from, to
}

func ruleSettingsFrom() workspace.Settings { f, _ := ruleSettings(); return f }

func carry(in transfer.CarryInput) transfer.Carried {
	f, t := ruleSettings()
	in.Project, in.From, in.To = proj, f, t
	if in.NewID == nil {
		in.NewID = func() change.ID { return "NEW" }
	}
	return transfer.CarryRules(in)
}

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %#v\nwant %#v", got, want)
	}
}

func TestCarryRulesHiddenByName(t *testing.T) {
	c := carry(transfer.CarryInput{Rules: projectrule.Rules{Hidden: []change.ID{"FPend", "FProg"}}})
	eq(t, c.Destination, []change.Draft{change.NewAdd(projectrule.HiddenField(proj), "TProg")})
	eq(t, c.Hidden, []change.ID{"TProg"})
	eq(t, c.NotCarried, []transfer.NotCarried{{Kind: "hidden", Name: "Pending"}})

	// 移動先の初期ステータスか既定のものは、Hidden に入らない。
	c = carry(transfer.CarryInput{Rules: projectrule.Rules{Hidden: []change.ID{"FTodo", "FProg"}}})
	eq(t, c.Hidden, []change.ID{"TProg"})
}

func TestCarryRulesWorkflowNeedsBothEnds(t *testing.T) {
	c := carry(transfer.CarryInput{Rules: projectrule.Rules{Workflow: map[change.ID][]change.ID{
		"FProg": {"FDone", "FRev"}, "FRev": {"FPend"},
	}}})
	eq(t, c.Destination, []change.Draft{change.NewAdd(projectrule.WorkflowField(proj, "TProg"), "TDone")})
	eq(t, c.NotCarried, []transfer.NotCarried{
		{Kind: "workflow", Name: "InProgress から Review", Missing: []string{"Review"}},
		{Kind: "workflow", Name: "Review から Pending", Missing: []string{"Review", "Pending"}},
	})
}

func TestCarryRulesTemplatesByNameAndKind(t *testing.T) {
	c := carry(transfer.CarryInput{Rules: projectrule.Rules{Templates: []change.ID{"FT1", "FT2"}}})
	eq(t, c.Destination, []change.Draft{change.NewAdd(projectrule.TemplatesField(proj), "TT1")})
	eq(t, c.NotCarried, []transfer.NotCarried{{Kind: "template", Name: "Bug 報告"}})

	c = carry(transfer.CarryInput{Rules: projectrule.Rules{Templates: []change.ID{"FT1"}}})
	eq(t, c.Destination, []change.Draft(nil))
}

func TestCarryRulesTemplateDefaults(t *testing.T) {
	c := carry(transfer.CarryInput{Rules: projectrule.Rules{TemplateDefaults: map[string]string{
		"Bug": "FT1", "Story": "FT2", "Task": projectrule.None,
	}}})
	eq(t, c.Destination, []change.Draft{
		change.NewSet(projectrule.TemplateDefaultField(proj, "Story"), nil, "TT1"),
		change.NewSet(projectrule.TemplateDefaultField(proj, "Task"), nil, projectrule.None),
	})
	eq(t, c.NotCarried, []transfer.NotCarried{{Kind: "templateDefault", Name: "Bug", Missing: []string{"Bug 報告"}}})
}

func TestCarryRulesDisabledIsNotCarried(t *testing.T) {
	c := carry(transfer.CarryInput{Rules: projectrule.Rules{Disabled: []change.ID{"R1", "R2"}}})
	eq(t, c.Destination, []change.Draft(nil))
	eq(t, c.NotCarried, []transfer.NotCarried{{Kind: "disabled", Count: 2}})
	eq(t, carry(transfer.CarryInput{}).NotCarried, []transfer.NotCarried(nil))
}

func ownRule() automation.Rule {
	return automation.Rule{
		ID: "FR", Name: "レビュー待ち", Trigger: automation.TriggerStatus, TriggerStatus: "FRev",
		Action: automation.ActionStatus, Value: "FDone", Enabled: true, Scope: proj,
	}
}

func TestCarryRulesOwnRulesGetNewIDsAndStatuses(t *testing.T) {
	c := carry(transfer.CarryInput{
		Own:          []automation.Rule{ownRule()},
		StatusChoice: map[change.ID]change.ID{"FRev": "TProg"},
	})
	want := ownRule()
	want.ID, want.TriggerStatus, want.Value = "NEW", "TProg", "TDone"
	eq(t, c.Created, []automation.Rule{want})
	eq(t, c.Destination, automation.Drafts(automation.Rule{}, want))

	// 選択がなく、名前も合わなければ、同じ分類(進行中)の移動先の既定。なければ初期ステータス。
	c = carry(transfer.CarryInput{Own: []automation.Rule{ownRule()}})
	eq(t, c.Created[0].TriggerStatus, change.ID("TTodo"))
	_, to := ruleSettings()
	to.CategoryDefaults[workspace.CategoryDoing] = "TProg"
	c = transfer.CarryRules(transfer.CarryInput{Project: proj, From: ruleSettingsFrom(), To: to, Own: []automation.Rule{ownRule()}, NewID: func() change.ID { return "N" }})
	eq(t, c.Created[0].TriggerStatus, change.ID("TProg"))
}

func TestCarryRulesOwnRuleAssigneeAcrossStores(t *testing.T) {
	r := automation.Rule{ID: "FR", Name: "担当", Trigger: automation.TriggerPROpen, Action: automation.ActionAssignee, Value: "Taro", Enabled: true, Scope: proj}
	c := carry(transfer.CarryInput{Own: []automation.Rule{r}, People: transfer.PeopleMap{"taro": "TARO"}})
	eq(t, c.Created[0].Value, "TARO")

	r.Value = automation.Unassigned
	c = carry(transfer.CarryInput{Own: []automation.Rule{r}, People: transfer.PeopleMap{"taro": "TARO"}})
	eq(t, c.Created[0].Value, automation.Unassigned)

	r.Value = "Taro"
	c = carry(transfer.CarryInput{Own: []automation.Rule{r}})
	eq(t, c.Created[0].Value, "Taro")
}

func TestCarryRulesSourceRemovesEverything(t *testing.T) {
	rules := projectrule.Rules{
		Hidden:           []change.ID{"FPend"},
		Workflow:         map[change.ID][]change.ID{"FProg": {"FDone"}},
		Templates:        []change.ID{"FT1"},
		TemplateDefaults: map[string]string{"Bug": "FT1"},
		Disabled:         []change.ID{"R1"},
	}
	c := carry(transfer.CarryInput{Rules: rules, Own: []automation.Rule{ownRule()}})
	var want []change.Draft
	for _, s := range []projectrule.Section{projectrule.SectionStatuses, projectrule.SectionWorkflow, projectrule.SectionTemplates, projectrule.SectionAutomation} {
		want = append(want, projectrule.ResetDrafts(rules, proj, s)...)
	}
	want = append(want, automation.DeleteDraft("FR"))
	eq(t, c.Source, want)
}

func TestCarryRulesHiddenThatCannotBeHidden(t *testing.T) {
	c := carry(transfer.CarryInput{Rules: projectrule.Rules{Hidden: []change.ID{"FTodo", "FDone"}}})
	eq(t, c.Destination, []change.Draft(nil))
	eq(t, c.Hidden, []change.ID(nil))
	eq(t, c.NotCarried, []transfer.NotCarried{{Kind: "hiddenFixed", Name: "Todo"}, {Kind: "hiddenFixed", Name: "Done"}})
}

func TestCarryRulesKeepsRuleWhenStatusesCollapse(t *testing.T) {
	r := ownRule()
	r.TriggerStatus, r.Value = "FRev", "FPend"
	c := carry(transfer.CarryInput{
		Own:          []automation.Rule{r},
		StatusChoice: map[change.ID]change.ID{"FRev": "TProg", "FPend": "TProg"},
	})
	eq(t, len(c.Created), 1)
	eq(t, c.Created[0].TriggerStatus, change.ID("TProg"))
	eq(t, c.Created[0].Value, "TProg")
	eq(t, c.Created[0].Enabled, true)
}

func TestRuleStatuses(t *testing.T) {
	a := ownRule() // Review → Done
	b := automation.Rule{ID: "B", Trigger: automation.TriggerStatus, TriggerStatus: "FDone", Action: automation.ActionAssignee, Value: "Taro"}
	eq(t, transfer.RuleStatuses([]automation.Rule{a, b}), []change.ID{"FRev", "FDone"})
	eq(t, transfer.RuleStatuses(nil), []change.ID(nil))
}
