package main

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
)

func (a *app) storeDiscoverCmd() *cobra.Command {
	var account, from string
	var all bool
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Find the stores you can read and offer to add them",
		Long:  "Discover lists the GitHub repositories with the hedgehug-store topic that the accounts can read, or with --from the bare repositories under a directory that hold a store, leaving out the stores set up already. On a terminal it asks which to add; elsewhere it only lists them, and --all adds every one.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.storeDiscover(ops.DiscoverInput{Account: account, From: from}, all)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "look with this account only (with --from: the account of the stores found)")
	cmd.Flags().StringVar(&from, "from", "", "look for bare repositories under this directory instead of on GitHub")
	cmd.Flags().BoolVar(&all, "all", false, "add every store found")
	return cmd
}

func (a *app) storeDiscover(in ops.DiscoverInput, all bool) error {
	cands, err := a.ss.StoreDiscover(in)
	// --from with several accounts and none named: on a terminal, ask which.
	if e := ops.AsError(err); e != nil && e.Code == "choice.required" && e.Args["what"] == "--account" && a.isTerminal(a.stdin) {
		names, _ := e.Args["candidates"].([]string)
		var picked string
		if picked, err = a.askChoice("Which account do the stores belong to?", names); err != nil {
			return err
		}
		in.Account = picked
		cands, err = a.ss.StoreDiscover(in)
	}
	if err != nil {
		return err
	}
	terminal := a.isTerminal(a.stdin)
	// With --json, stdout is the JSON alone: what a person reads goes to stderr.
	list := a.stdout
	if a.jsonOut {
		list = a.stderr
	}
	var picked []ops.Candidate
	listed := false
	switch {
	case all:
		picked = cands
	case terminal && len(cands) > 0:
		if err := a.printCandidates(list, cands); err != nil {
			return err
		}
		listed = true
		if picked, err = a.pickCandidates(cands); err != nil {
			return err
		}
	}
	var res ops.MountResult
	var mountErr error
	if len(picked) > 0 {
		res, mountErr = a.ss.StoreMount(picked)
	}
	if a.jsonOut {
		if err := a.printDiscoverJSON(cands, res); err != nil {
			return err
		}
		return mountErr
	}
	switch {
	case len(cands) == 0:
		if _, err := fmt.Fprintln(a.stdout, "No store found"); err != nil {
			return err
		}
	case !all && !terminal && !listed:
		if err := a.printCandidates(list, cands); err != nil {
			return err
		}
	}
	for _, name := range res.Mounted {
		if _, err := fmt.Fprintf(a.stdout, "Added store %s\n", name); err != nil {
			return err
		}
	}
	for _, f := range res.Failed {
		if _, err := fmt.Fprintf(a.stderr, "Could not add store %s: %s\n", f.Name, describe(f.Err)); err != nil {
			return err
		}
	}
	return mountErr
}

func (a *app) printDiscoverJSON(cands []ops.Candidate, res ops.MountResult) error {
	type failure struct {
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	out := struct {
		Candidates []ops.Candidate `json:"candidates"`
		Mounted    []string        `json:"mounted"`
		Failed     []failure       `json:"failed,omitempty"`
	}{Candidates: cands, Mounted: res.Mounted}
	if out.Candidates == nil {
		out.Candidates = []ops.Candidate{}
	}
	if out.Mounted == nil {
		out.Mounted = []string{}
	}
	for _, f := range res.Failed {
		out.Failed = append(out.Failed, failure{f.Name, describe(f.Err)})
	}
	return a.printJSON(out)
}

func (a *app) printCandidates(w io.Writer, cands []ops.Candidate) error {
	rows := make([][]string, len(cands))
	for i, x := range cands {
		acct := x.Account
		if acct == "" {
			acct = "-"
		}
		rows[i] = []string{strconv.Itoa(i + 1), x.Name, x.URL, acct, x.Permission}
	}
	if err := writeTable(w, []string{"#", "NAME", "URL", "ACCOUNT", "PERMISSION"}, rows, nil); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "To add one under another name, use hedgehug store add <name> --remote <url>")
	return err
}

// pickCandidates asks which candidates to add: numbers with commas, all, or
// nothing to stop.
func (a *app) pickCandidates(cands []ops.Candidate) ([]ops.Candidate, error) {
	if _, err := fmt.Fprint(a.stderr, "Add which? (e.g. 1,3 / all / empty to stop) "); err != nil {
		return nil, err
	}
	line, _ := a.readLine()
	line = strings.TrimSpace(line)
	switch strings.ToLower(line) {
	case "":
		return nil, nil
	case "all":
		return cands, nil
	}
	var out []ops.Candidate
	var chosen []int // a number given twice is added once
	for _, f := range strings.Split(line, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 || n > len(cands) {
			return nil, usageErrorf("%q is not a number from the list", strings.TrimSpace(f))
		}
		if !slices.Contains(chosen, n) {
			chosen = append(chosen, n)
			out = append(out, cands[n-1])
		}
	}
	return out, nil
}
