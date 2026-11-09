package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/automations"
)

func (a *app) automationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "automation",
		Short: "List and edit the rules of automations.yaml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.AddCommand(a.automationListCmd(), a.automationAddCmd(), a.automationRmCmd(),
		a.automationEnableCmd(true), a.automationEnableCmd(false), a.automationFieldsCmd())
	return cmd
}

func (a *app) automationListCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the rules, with whether each is on",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			rs, err := automations.List(a.ss, project)
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(rs)
			}
			if len(rs) == 0 {
				_, err := fmt.Fprintln(a.stdout, "No rule")
				return err
			}
			rows := make([][]string, len(rs))
			for i, r := range rs {
				rows[i] = []string{r.ID, r.On, dash(r.When), ruleSet(r), ruleScope(r), ruleState(r)}
			}
			return writeTable(a.stdout, []string{"ID", "ON", "WHEN", "SET", "SCOPE", "STATE"}, rows, nil)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "only the rules that apply in this project")
	return cmd
}

func (a *app) automationAddCmd() *cobra.Command {
	var in automations.AddInput
	cmd := &cobra.Command{
		Use:   "add --id <id> --on <event> (--status <s>|--category <c>) [--when <w>] [--project <id>]",
		Short: "Add a rule",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if in.ID == "" || in.On == "" {
				return usageErrorf("--id and --on are required")
			}
			if (in.Status == "") == (in.Category == "") {
				return usageErrorf("give --status or --category, one of them")
			}
			if err := automations.Add(a.ss, in); err != nil {
				return err
			}
			return a.printAutomationDone("Added rule " + in.ID)
		},
	}
	f := cmd.Flags()
	f.StringVar(&in.ID, "id", "", "the id of the rule")
	f.StringVar(&in.On, "on", "", "the event: pr.opened, pr.review_requested, pr.merged or issue.closed")
	f.StringVar(&in.Status, "status", "", "the status to set")
	f.StringVar(&in.Category, "category", "", "the category whose default status to set")
	f.StringVar(&in.When, "when", "", "a condition: all_linked_prs_merged")
	f.StringVar(&in.Project, "project", "", "only for the tasks in this project")
	return cmd
}

func (a *app) automationRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a rule",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := automations.Remove(a.ss, args[0]); err != nil {
				return err
			}
			return a.printAutomationDone("Removed rule " + args[0])
		},
	}
}

func (a *app) automationEnableCmd(on bool) *cobra.Command {
	use, short, done := "disable", "Turn a rule off, or off in one project", "Disabled rule "
	if on {
		use, short, done = "enable", "Turn a rule on, or back on in one project", "Enabled rule "
	}
	var project string
	cmd := &cobra.Command{
		Use:   use + " <id> [--project <id>]",
		Short: short,
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := automations.SetEnabled(a.ss, args[0], project, on); err != nil {
				return err
			}
			msg := done + args[0]
			if project != "" {
				msg += " in " + project
			}
			return a.printAutomationDone(msg)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "change it for this project and the tasks in it, not for the whole store")
	return cmd
}

func (a *app) automationFieldsCmd() *cobra.Command {
	var sync, push, mirror string
	var props []string
	cmd := &cobra.Command{
		Use:   "fields [--default-sync <mode>] [--push <a,b>] [--mirror <a,b>] [--prop <key=value>]",
		Short: "Show issue_fields, or change it",
		Long:  "Without flags, print issue_fields. A flag changes that part and keeps the rest. An empty --push or --mirror takes every field off; --prop key= takes a key off.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := cmd.Flags()
			var in automations.FieldsInput
			changed := false
			if f.Changed("default-sync") {
				in.DefaultSync, changed = &sync, true
			}
			for _, l := range []struct {
				name, val string
				to        **[]string
			}{{"push", push, &in.Push}, {"mirror", mirror, &in.Mirror}} {
				if !f.Changed(l.name) {
					continue
				}
				v, err := splitNames("--"+l.name, l.val)
				if err != nil {
					return err
				}
				*l.to, changed = &v, true
			}
			if f.Changed("prop") {
				in.Props = map[string]string{}
				for _, p := range props {
					k, v, ok := strings.Cut(p, "=")
					if !ok || strings.TrimSpace(k) == "" {
						return usageErrorf("--prop: %q is not of the form key=value", p)
					}
					in.Props[strings.TrimSpace(k)] = strings.TrimSpace(v)
				}
				changed = true
			}
			if changed {
				if err := automations.SetFields(a.ss, in); err != nil {
					return err
				}
			}
			v, err := automations.Fields(a.ss)
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(v)
			}
			var b strings.Builder
			if changed {
				b.WriteString("Changed issue_fields\n")
			}
			fmt.Fprintf(&b, "default_sync: %s\npush: %s\nmirror: %s\n", v.DefaultSync, listText(v.Push), listText(v.Mirror))
			keys := make([]string, 0, len(v.Props))
			for k := range v.Props {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) == 0 {
				b.WriteString("props: -\n")
			}
			for _, k := range keys {
				fmt.Fprintf(&b, "props: %s=%s\n", k, v.Props[k])
			}
			_, err = fmt.Fprint(a.stdout, b.String())
			return err
		},
	}
	f := cmd.Flags()
	f.StringVar(&sync, "default-sync", "", "how a linked issue syncs when --sync is not given: mirror, push or link")
	f.StringVar(&push, "push", "", "the fields a push link sends, such as title,body,status; comments sends the comments of the task")
	f.StringVar(&mirror, "mirror", "", "the fields a mirror link takes both ways, such as title,status; comments takes the comments of the issue")
	f.StringArrayVar(&props, "prop", nil, "a property of the issue to sync, such as milestone=target (repeatable)")
	return cmd
}

// printAutomationDone prints what an edit did; with --json, the rules as they
// are now.
func (a *app) printAutomationDone(msg string) error {
	if a.jsonOut {
		rs, err := automations.List(a.ss, "")
		if err != nil {
			return err
		}
		return a.printJSON(rs)
	}
	_, err := fmt.Fprintln(a.stdout, msg)
	return err
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func listText(l []string) string {
	if len(l) == 0 {
		return "-"
	}
	return strings.Join(l, ", ")
}

func ruleSet(r automations.RuleView) string {
	if r.Status != "" {
		return "status=" + r.Status
	}
	return "category=" + r.Category
}

func refName(r usecase.Ref) string {
	if r.Missing || r.DisplayID == "" {
		return r.ULID
	}
	return r.DisplayID
}

func ruleScope(r automations.RuleView) string {
	if r.Project == nil {
		return "store"
	}
	return refName(*r.Project)
}

func ruleState(r automations.RuleView) string {
	var in []string
	for _, p := range r.DisabledIn {
		in = append(in, refName(p))
	}
	switch {
	case !r.Enabled:
		return "off"
	case len(in) > 0:
		return "on (off in " + strings.Join(in, ", ") + ")"
	}
	return "on"
}
