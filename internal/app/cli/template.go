package cli

import (
	"bytes"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/templates"
	"github.com/kukv/hedgehug/internal/lib/osfs"
)

func (a *app) templateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage the task templates of a store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "ls",
			Short: "List the templates",
			Args:  cobra.NoArgs,
			RunE:  func(_ *cobra.Command, _ []string) error { return a.templateLs() },
		},
		&cobra.Command{
			Use:   "show <name>",
			Short: "Print the file of a template",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.templateShow(args[0]) },
		},
		&cobra.Command{
			Use:   "edit <name>",
			Short: "Edit a template in $EDITOR, or write a new one",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.templateEdit(args[0]) },
		},
		&cobra.Command{
			Use:   "rm <name>",
			Short: "Delete a template that the schema does not name",
			Args:  exactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				w, err := templates.Remove(a.ss, args[0])
				if err != nil {
					return err
				}
				return a.printTemplate(w, "Removed template "+w.Template.Name)
			},
		},
	)
	return cmd
}

func (a *app) templateLs() error {
	vs, err := templates.List(a.ss)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(vs)
	}
	rows := make([][]string, len(vs))
	for i, v := range vs {
		rows[i] = []string{v.Name, v.Kind, v.Title, fmt.Sprint(v.Children)}
	}
	return writeTable(a.stdout, []string{"NAME", "KIND", "TITLE", "CHILDREN"}, rows, nil)
}

func (a *app) templateShow(name string) error {
	f, err := templates.Read(a.ss, name)
	if err != nil {
		return err
	}
	if !f.Exists {
		return usecase.NewErr(usecase.KindNotFound, "template.not_found", fmt.Errorf("template %s is not in this store", name), "name", name)
	}
	if a.jsonOut {
		return a.printJSON(f)
	}
	_, err = fmt.Fprint(a.stdout, f.Content)
	return err
}

func (a *app) templateEdit(name string) error {
	f, err := templates.Read(a.ss, name)
	if err != nil {
		return err
	}
	orig := []byte(f.Content)
	path, edited, err := a.editFile("template-"+name, orig)
	if err != nil {
		return err
	}
	for {
		if bytes.Equal(edited, orig) {
			if err := osfs.Remove(path); err != nil {
				return err
			}
			_, err := fmt.Fprintln(a.stdout, verb(false, "")+": template "+name)
			return err
		}
		w, err := templates.Put(a.ss, name, string(edited))
		if err == nil {
			if err := osfs.Remove(path); err != nil {
				return err
			}
			return a.printTemplate(w, verb(w.Commit != nil, "Saved")+" template "+name)
		}
		if usecase.KindOf(err) != usecase.KindInvalid {
			return fmt.Errorf("%s; the edited file is kept at %s", usecase.Describe(err), path)
		}
		writeErr(a.stderr, err)
		if !a.askRetry() {
			return fmt.Errorf("the edit is canceled; the edited file is kept at %s", path)
		}
		if edited, err = a.reopen(path, edited); err != nil {
			return err
		}
	}
}

// printTemplate prints the template w wrote as JSON, or line as text.
func (a *app) printTemplate(w templates.TemplateWrite, line string) error {
	if a.jsonOut {
		return a.printJSON(w)
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
