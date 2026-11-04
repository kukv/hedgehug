package stores

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

// StoreItem is a row of store list: the store as StoreView has it, and
// whether hedgehug picks it without --store.
type StoreItem struct {
	StoreView
	Default bool `json:"default" doc:"The store hedgehug picks without --store"`
}

// StoreItems lists the stores that are set up, the one hedgehug picks without
// --store marked.
func StoreItems(ss *usecase.Session) ([]StoreItem, error) {
	list, err := StoreList(ss)
	if err != nil {
		return nil, err
	}
	def := ""
	if c, _, err := ss.Config(); err == nil {
		if e, err := c.Select(""); err == nil {
			def = e.Name
		}
	}
	out := make([]StoreItem, len(list))
	for i, v := range list {
		out[i] = StoreItem{StoreView: v, Default: v.Name == def}
	}
	return out, nil
}

// ViewOf is the row of the store name of StoreList.
func ViewOf(ss *usecase.Session, name string) (StoreView, error) {
	list, err := StoreList(ss)
	if err != nil {
		return StoreView{}, err
	}
	for _, v := range list {
		if v.Name == name {
			return v, nil
		}
	}
	return StoreView{}, fmt.Errorf("store %s is not listed after it was added", name)
}

// StoreInitView is the store StoreInit made, as store init --json prints it
// and as the server answers.
type StoreInitView struct {
	Name      string                `json:"name"`
	Path      string                `json:"path"`
	Workspace usecase.WorkspaceView `json:"workspace"`
	Remotes   []string              `json:"remotes"`
	Pushed    []string              `json:"pushed"`
}

// StoreInitViewOf is r as a view. ss is a session on the store r made: its
// workspace is read from there, as StoreInit leaves the time of the prefix
// out of its own.
func StoreInitViewOf(ss *usecase.Session, r StoreInitResult) StoreInitView {
	v := StoreInitView{
		Name: r.Name, Path: r.Path,
		Workspace: usecase.WorkspaceView{ULID: r.Workspace.ULID.String(), Name: r.Workspace.Name, IDPrefix: r.Workspace.IDPrefix, PrefixSet: r.Workspace.PrefixSet},
		Remotes:   append([]string{}, r.Remotes...), Pushed: append([]string{}, r.Pushed...),
	}
	if _, ix, err := ss.Open(); err == nil {
		if wss := ix.Workspaces(); len(wss) == 1 {
			v.Workspace = usecase.WorkspaceViewOf(ix, wss[0])
		}
	}
	return v
}

// DefaultView is the default store as store default --json prints it and as
// the server answers.
type DefaultView struct {
	Name *string `json:"name" doc:"The default store; null when there is none"`
}

// DefaultViewOf is name as a view; "" is no default.
func DefaultViewOf(name string) DefaultView {
	if name == "" {
		return DefaultView{}
	}
	return DefaultView{Name: &name}
}

// MountFailed is a candidate that could not be added, as the CLI's --json and
// the server tell it.
type MountFailed struct {
	Name    string `json:"name"`
	Code    string `json:"code" doc:"What went wrong, as the type of a problem"`
	Message string `json:"message" doc:"The message in English, as the CLI prints it"`
}

// MountView is a MountResult as the CLI's --json and the server tell it.
type MountView struct {
	Mounted []string      `json:"mounted" doc:"The names of the stores that were added"`
	Failed  []MountFailed `json:"failed" doc:"The candidates that could not be added; the others were"`
}

// View is r as a view; the lists are never null.
func (r MountResult) View() MountView {
	v := MountView{Mounted: append([]string{}, r.Mounted...), Failed: []MountFailed{}}
	for _, f := range r.Failed {
		v.Failed = append(v.Failed, MountFailed{Name: f.Name, Code: usecase.ProblemCode(f.Err), Message: usecase.Describe(f.Err)})
	}
	return v
}
