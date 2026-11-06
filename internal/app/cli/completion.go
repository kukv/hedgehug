package cli

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kukv/hedgehug/internal/app/usecase/automations"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/planning"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/templates"
	"github.com/kukv/hedgehug/internal/app/usecase/views"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
)

// noCompletion names the commands that need no hook: the script, and the
// hidden commands the script runs for every Tab.
func noCompletion(cmd *cobra.Command) bool {
	name := cmd.Name()
	return name == "completion" || name == cobra.ShellCompRequestCmd || name == cobra.ShellCompNoDescRequestCmd
}

func (a *app) completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion <bash|zsh|fish>",
		Short: "Print the completion script of a shell",
		Long: "Print the script that completes commands, flags and values in a shell.\n\n" +
			"  bash: hh completion bash > ~/.local/share/bash-completion/completions/hh\n" +
			"  zsh:  hh completion zsh > \"${fpath[1]}/_hh\"\n" +
			"  fish: hh completion fish > ~/.config/fish/completions/hh.fish\n\n" +
			"The script is for the name it is run by: run it as hedgehug to complete hedgehug.",
		Args:      asUsage(cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs)),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			if a.progName == "hh" {
				root.Use = "hh"
			}
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(a.stdout, true)
			case "zsh":
				return root.GenZshCompletion(a.stdout)
			default:
				return root.GenFishCompletion(a.stdout, true)
			}
		},
	}
}

// A completer gives the candidates of a word. A failed read gives none: a Tab
// says nothing of what is wrong.
type completer func(a *app, word string) []cobra.Completion

// argSpec is what the arguments of a command complete to: the first ones by
// their place, the ones after them by rest.
type argSpec struct {
	each []completer
	rest completer
}

func one(c completer) argSpec { return argSpec{each: []completer{c}} }

// argCompleters are the commands whose arguments complete, by the path under
// the root.
var argCompleters = map[string]argSpec{
	"show": one(completeIDs), "edit": one(completeIDs), "rm": one(completeIDs), "restore": one(completeIDs),
	"done": one(completeIDs), "mv": one(completeIDs), "log": one(completeIDs), "use": one(completeIDs),
	"start": one(completeIDs), "pr": one(completeIDs), "issue": one(completeIDs),
	"link": one(completeIDs), "unlink": one(completeIDs),
	"watch": one(completeIDs), "unwatch": one(completeIDs),
	"comment add": one(completeIDs),
	"dep add":     one(completeIDs), "dep rm": one(completeIDs),
	"worklog": one(completeIDs), "worklog ls": one(completeIDs), "worklog rm": one(completeIDs),
	"schema scope set": one(completeIDs), "schema scope show": one(completeIDs), "schema scope rm": one(completeIDs),
	"automation rm": one(completeRules), "automation enable": one(completeRules), "automation disable": one(completeRules),
	"status": {each: []completer{completeIDs, completeStatuses}},
	"set":    {each: []completer{completeIDs}, rest: completeAssign},
	"ls":     {rest: completeQuery},

	"store remove": one(completeStores), "store set": one(completeStores), "store default": one(completeStores),
	"workspace rename": one(completeWorkspaces), "workspace set-prefix": one(completeWorkspaces), "workspace rm": one(completeWorkspaces),
	"sprint start": one(completeSprints), "sprint close": one(completeSprints), "sprint set": one(completeSprints), "sprint rm": one(completeSprints),
	"report burndown": one(completeSprints),
	"release set":     one(completeReleases), "release notes": one(completeReleases), "release rm": one(completeReleases),
	"view show": one(completeViews), "view rm": one(completeViews), "view rename": one(completeViews),
	"template show": one(completeTemplates), "template edit": one(completeTemplates), "template rm": one(completeTemplates),
}

// flagCompleters are the flags whose values complete, by name, in every
// command but store init: its --template and --workspace are not the ones of
// a store that exists.
var flagCompleters = map[string]completer{
	"parent": completeIDs, "after": completeIDs, "before": completeIDs, "blocked-by": completeIDs, "project": completeIDs,
	"store": completeStores, "workspace": completeWorkspaces,
	"sprint": completeSprints, "release": completeReleases, "template": completeTemplates,
}

// wireCompletion gives the commands under root their completers.
func (a *app) wireCompletion(root *cobra.Command) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		path := strings.TrimPrefix(strings.TrimPrefix(c.CommandPath(), root.Name()), " ")
		if spec, ok := argCompleters[path]; ok {
			c.ValidArgsFunction = a.completeArgs(spec)
		}
		if path != "store init" {
			for _, fs := range []*pflag.FlagSet{c.Flags(), c.PersistentFlags()} {
				fs.VisitAll(func(f *pflag.Flag) {
					if fn, ok := flagCompleters[f.Name]; ok {
						_ = c.RegisterFlagCompletionFunc(f.Name, a.completeFlag(fn)) // a flag in both sets is there already
					}
				})
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}

func (a *app) completeFlag(fn completer) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, word string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return fn(a, word), cobra.ShellCompDirectiveNoFileComp
	}
}

func (a *app) completeArgs(spec argSpec) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, word string) ([]cobra.Completion, cobra.ShellCompDirective) {
		fn := spec.rest
		if q := cmd.Flags().Lookup("query"); q == nil || !q.Changed {
			if len(args) < len(spec.each) {
				fn = spec.each[len(args)]
			}
		}
		if fn == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		dir := cobra.ShellCompDirectiveNoFileComp
		if spec.rest != nil && len(args) >= len(spec.each) {
			dir |= cobra.ShellCompDirectiveNoSpace // key= and key:value go on
		}
		return fn(a, word), dir
	}
}

// withPrefix keeps the names that start with word, after prefix.
func withPrefix(prefix string, names []string, word string) []cobra.Completion {
	var out []cobra.Completion
	for _, n := range names {
		if strings.HasPrefix(prefix+n, word) {
			out = append(out, prefix+n)
		}
	}
	return out
}

func completeIDs(a *app, word string) []cobra.Completion {
	res, err := browse.List(a.ss, "")
	if err != nil {
		return nil
	}
	var out []cobra.Completion
	for _, t := range res.Tasks {
		if strings.HasPrefix(t.ID, word) {
			out = append(out, cobra.CompletionWithDesc(t.ID, t.Title))
		}
	}
	return out
}

func completeStatuses(a *app, word string) []cobra.Completion {
	v, err := browse.ReadVocabulary(a.ss)
	if err != nil {
		return nil
	}
	return withPrefix("", v.Statuses, word)
}

func completeStores(a *app, word string) []cobra.Completion {
	list, err := stores.StoreItems(a.ss)
	if err != nil {
		return nil
	}
	names := make([]string, len(list))
	for i, s := range list {
		names[i] = s.Name
	}
	return withPrefix("", names, word)
}

func completeWorkspaces(a *app, word string) []cobra.Completion {
	list, err := workspaces.WorkspaceList(a.ss)
	if err != nil {
		return nil
	}
	names := make([]string, len(list))
	for i, w := range list {
		names[i] = w.Name
	}
	return withPrefix("", names, word)
}

func completeSprints(a *app, word string) []cobra.Completion {
	return withPrefix("", sprintNames(a), word)
}

func completeReleases(a *app, word string) []cobra.Completion {
	return withPrefix("", releaseNames(a), word)
}

// sprintNames lists the sprints of every workspace: a completion has no
// workspace to ask for.
func sprintNames(a *app) []string {
	return namesByWorkspace(a, func(ws string) ([]string, error) {
		list, err := planning.SprintList(a.ss, ws, "")
		names := make([]string, len(list))
		for i, s := range list {
			names[i] = s.Name
		}
		return names, err
	})
}

func releaseNames(a *app) []string {
	return namesByWorkspace(a, func(ws string) ([]string, error) {
		list, err := planning.ReleaseList(a.ss, ws, "")
		names := make([]string, len(list))
		for i, r := range list {
			names[i] = r.Name
		}
		return names, err
	})
}

func namesByWorkspace(a *app, list func(ws string) ([]string, error)) []string {
	wss, err := workspaces.WorkspaceList(a.ss)
	if err != nil {
		return nil
	}
	var out []string
	for _, w := range wss {
		names, err := list(w.Name)
		if err != nil {
			continue
		}
		out = append(out, names...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func completeRules(a *app, word string) []cobra.Completion {
	list, err := automations.List(a.ss, "")
	if err != nil {
		return nil
	}
	names := make([]string, len(list))
	for i, r := range list {
		names[i] = r.ID
	}
	return withPrefix("", names, word)
}

func completeViews(a *app, word string) []cobra.Completion {
	list, err := views.ViewList(a.ss)
	if err != nil {
		return nil
	}
	names := make([]string, len(list))
	for i, v := range list {
		names[i] = v.Name
	}
	return withPrefix("", names, word)
}

func completeTemplates(a *app, word string) []cobra.Completion {
	list, err := templates.List(a.ss)
	if err != nil {
		return nil
	}
	names := make([]string, len(list))
	for i, t := range list {
		names[i] = t.Name
	}
	return withPrefix("", names, word)
}

// setFields are the keys of hedgehug set that are not properties.
var setFields = []string{"title", "kind", "status", "assignees", "watchers", "labels", "repos", "blocked_by", "sprint", "releases"}

// completeAssign completes a key=, then the value of the keys with values to
// offer.
func completeAssign(a *app, word string) []cobra.Completion {
	v, err := browse.ReadVocabulary(a.ss)
	if err != nil {
		return nil
	}
	i := strings.IndexByte(word, '=')
	if i < 0 {
		keys := make([]string, 0, len(setFields)+len(v.Props))
		for _, k := range append(setFields[:len(setFields):len(setFields)], v.Props...) {
			keys = append(keys, k+"=")
		}
		return withPrefix("", keys, word)
	}
	var values []string
	switch strings.TrimRight(word[:i], "+-") {
	case "status":
		values = v.Statuses
	case "assignees", "watchers":
		values = v.Members
	case "sprint":
		values = sprintNames(a)
	case "releases":
		values = releaseNames(a)
	}
	return withPrefix(word[:i+1], values, word)
}

// completeQuery completes the value of assignee:, the term that needs a name
// nobody remembers.
func completeQuery(a *app, word string) []cobra.Completion {
	if !strings.HasPrefix(word, "assignee:") {
		return nil
	}
	v, err := browse.ReadVocabulary(a.ss)
	if err != nil {
		return nil
	}
	return withPrefix("assignee:", v.Members, word)
}
