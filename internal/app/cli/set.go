package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/cli/prompt"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

func (a *app) setCmd() *cobra.Command {
	var bulkQuery string
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "set <id> <key=value>...",
		Short: "Change fields and properties of a task",
		Long: `Change fields and properties of a task, or of every task a query finds:

  hedgehug set --query 'label:x' status=done

That is one commit in each store, and nothing is written when a task would
break the schema. It lists the tasks and asks; --yes does not ask, and
--dry-run stops at the list.

  key=value   set (a list becomes just value)
  key=        clear
  key+=value  add to a list
  key-=value  remove from a list

Fields: title, kind, status, assignees, watchers, labels, repos, blocked_by,
causes, relates, duplicates, clones, sprint, releases (a name in the workspace
of the task). The other direction of a relation (blocks, caused_by, ...) is
read-only.
Any other key is a property of the schema. Use hedgehug mv for the parent
and the order.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if bulkQuery != "" {
				return minArgs(1)(cmd, args)
			}
			return minArgs(2)(cmd, args)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if bulkQuery != "" {
				assigns, err := usecase.ParseAssigns(args)
				if err != nil {
					return err
				}
				return a.setQuery(bulkQuery, assigns, dryRun, yes)
			}
			assigns, err := usecase.ParseAssigns(args[1:])
			if err != nil {
				return err
			}
			return a.set(args[0], assigns)
		},
	}
	f := cmd.Flags()
	f.StringVar(&bulkQuery, "query", "", "change every task this query finds")
	f.BoolVar(&dryRun, "dry-run", false, "with --query: list the tasks and what they would break, and write nothing")
	f.BoolVar(&yes, "yes", false, "with --query: do not ask")
	return cmd
}

func (a *app) setQuery(queryText string, assigns []usecase.Assign, dryRun, yes bool) error {
	if !dryRun && !yes && !a.ask().Interactive() {
		return usageErrorf("--query changes many tasks: give --yes, or --dry-run to see them")
	}
	targets, err := browse.Targets(a.ss, queryText)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if a.jsonOut {
			return a.printJSON(tasks.BulkResult{Tasks: []tasks.BulkTask{}})
		}
		_, err := fmt.Fprintln(a.stdout, "No tasks match")
		return err
	}
	in := tasks.BulkInput{Targets: targets, Assigns: assigns, DryRun: true}
	if !yes {
		r, err := tasks.Bulk(a.ss, in)
		if err != nil {
			return err
		}
		if dryRun || len(r.Violations) > 0 {
			if a.jsonOut && dryRun {
				return a.printJSON(r)
			}
			if err := a.printBulk(r); err != nil || dryRun {
				return err
			}
		} else {
			if err := a.printBulk(r); err != nil {
				return err
			}
			ok, err := a.ask().Confirm(prompt.YesNo{Question: fmt.Sprintf("Update %s? [y/N] ", plural.Count(len(r.Tasks), "task")), Canceled: "canceled"})
			if err != nil || !ok {
				return err
			}
		}
	}
	in.DryRun = false
	r, err := tasks.Bulk(a.ss, in)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(r)
	}
	_, err = fmt.Fprintf(a.stdout, "Updated %s\n", plural.Count(len(r.Tasks), "task"))
	return err
}

// printBulk lists the tasks of a bulk update and what they would break.
func (a *app) printBulk(r tasks.BulkResult) error {
	rows := make([][]string, len(r.Tasks))
	for i, t := range r.Tasks {
		rows[i] = []string{t.ID, t.Store, t.Title}
	}
	if err := writeTable(a.stdout, []string{"ID", "STORE", "TITLE"}, rows, nil); err != nil {
		return err
	}
	for _, v := range r.Violations {
		if _, err := fmt.Fprintf(a.stdout, "  %s: %s\n", v.Task.ID, v.Message); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) set(arg string, assigns []usecase.Assign) error {
	r, err := tasks.Set(a.ss, arg, assigns)
	if err != nil {
		return err
	}
	return a.printTask(r, verb(r.Committed, "Updated"))
}
