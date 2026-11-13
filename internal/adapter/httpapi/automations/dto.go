package automations

import (
	"time"

	appautomations "github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// AutomationRuleInput は、ルールの作成と編集の本文。
type AutomationRuleInput struct {
	Name          string `json:"name"`
	Trigger       string `json:"trigger" enum:"pr_open,review_requested,pr_merged,issue_closed,status,steps_done"`
	TriggerStatus string `json:"triggerStatus" doc:"きっかけが status のときのステータスの ID。ほかは空"`
	AllMerged     bool   `json:"allMerged" doc:"きっかけが pr_merged のときの条件"`
	Action        string `json:"action" enum:"status,assignee"`
	Value         string `json:"value" doc:"ステータスの ID、ログイン名、または @none(未割り当て)"`
}

// AutomationRule は、ルール。
type AutomationRule struct {
	ID string `json:"id"`
	AutomationRuleInput
	Enabled bool `json:"enabled"`
}

// AutomationRun は、最近の実行の 1 行。
type AutomationRun struct {
	At      time.Time `json:"at"`
	Item    string    `json:"item"`
	Display string    `json:"display"`
	Text    string    `json:"text"`
}

// WorkspaceAutomation は、自動化のサブタブの中身。
type WorkspaceAutomation struct {
	Rules  []AutomationRule `json:"rules"`
	Recent []AutomationRun  `json:"recent"`
}

// SetAutomationRuleEnabled は、スイッチの本文。
type SetAutomationRuleEnabled struct {
	Enabled bool `json:"enabled"`
}

// ApplyAutomationIgnoringWorkflow は、「制約を無視して…」の本文。
type ApplyAutomationIgnoringWorkflow struct {
	Status string `json:"status" doc:"変えるステータスの ID(通知の Status)"`
}

func fromInput(in AutomationRuleInput) automation.Rule {
	return automation.Rule{
		Name: in.Name, Trigger: automation.Trigger(in.Trigger), TriggerStatus: change.ID(in.TriggerStatus),
		AllMerged: in.AllMerged, Action: automation.Action(in.Action), Value: in.Value,
	}
}

func toRule(r automation.Rule) AutomationRule {
	return AutomationRule{ID: string(r.ID), Enabled: r.Enabled, AutomationRuleInput: AutomationRuleInput{
		Name: r.Name, Trigger: string(r.Trigger), TriggerStatus: string(r.TriggerStatus),
		AllMerged: r.AllMerged, Action: string(r.Action), Value: r.Value,
	}}
}

func toView(v appautomations.View) WorkspaceAutomation {
	out := WorkspaceAutomation{Rules: []AutomationRule{}, Recent: []AutomationRun{}}
	for _, r := range v.Rules {
		out.Rules = append(out.Rules, toRule(r))
	}
	for _, r := range v.Recent {
		out.Recent = append(out.Recent, AutomationRun{At: r.At, Item: string(r.Item), Display: r.Display, Text: r.Text})
	}
	return out
}
