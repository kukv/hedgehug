package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
)

type WorkspaceParam struct {
	StoreParam
	ID string `path:"id" doc:"The workspace: its name, prefix or ULID"`
}

type workspaceWrite struct {
	Workspace workspaces.WorkspaceView `json:"workspace"`
	Commit    *string                  `json:"commit"`
	IDs       []idChange               `json:"ids,omitempty" doc:"With a new prefix: the IDs the tasks had and have now"`
}

type idChange struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type workspaceOutput struct{ Body workspaceWrite }

type workspaceAddInput struct {
	StoreParam
	Body struct {
		Name   string `json:"name"`
		Prefix string `json:"prefix"`
	}
}

type workspacePatchInput struct {
	WorkspaceParam
	Body struct {
		Name   *string `json:"name,omitempty" doc:"A new name; give it or prefix, not both"`
		Prefix *string `json:"prefix,omitempty" doc:"A new prefix, which renumbers the tasks"`
	}
}

type findingsOutput struct {
	Body struct {
		Findings []usecase.Finding `json:"findings"`
	}
}

type migrateInput struct {
	StoreParam
	Body struct {
		Target string `json:"target" enum:"status,kind,prop,value"`
		Prop   string `json:"prop,omitempty" doc:"With value: the property whose value it is"`
		Old    string `json:"old" minLength:"1"`
		New    string `json:"new" minLength:"1"`
		DryRun bool   `json:"dry_run,omitempty"`
	}
}

type migrateOutput struct {
	Body struct {
		Changed []*usecase.TaskView `json:"changed"`
		Skipped []string            `json:"skipped"`
		Commit  *string             `json:"commit"`
	}
}

type storeItem struct {
	stores.StoreView
	Default bool `json:"default" doc:"The store hedgehug picks without --store"`
}

type storesOutput struct{ Body []storeItem }

type storeInitInput struct {
	Body struct {
		Name      string   `json:"name"`
		Prefix    string   `json:"prefix"`
		Workspace string   `json:"workspace,omitempty" doc:"The name of the first workspace; the name of the store without it"`
		Remotes   []string `json:"remotes,omitempty"`
		Account   string   `json:"account,omitempty"`
	}
}

type storeInitOutput struct {
	Body struct {
		Name      string                   `json:"name"`
		Path      string                   `json:"path"`
		Workspace workspaces.WorkspaceView `json:"workspace"`
		Remotes   []string                 `json:"remotes"`
		Pushed    []string                 `json:"pushed"`
	}
}

type doctorOutput struct{ Body []maintenance.DoctorReport }

// workspaceOut is the output of a write to a workspace. The view counts the
// tasks from the index the operation read (refreshed after a renumbering), so
// that a write that is committed is never answered with an error.
func workspaceOut(r workspaces.WorkspaceResult) *workspaceOutput {
	out := &workspaceOutput{Body: workspaceWrite{Workspace: workspaces.WorkspaceViewOf(r.Index, r.Workspace), Commit: commitPtr(r.Commit)}}
	for _, c := range r.IDs {
		out.Body.IDs = append(out.Body.IDs, idChange{c.Old, c.New})
	}
	return out
}

func (s *Server) registerAdmin() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-workspaces", Method: http.MethodGet, Path: "/api/workspaces",
		Summary: "List the workspaces, as hh workspace ls", Tags: []string{"workspaces"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []workspaces.WorkspaceView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		_, ws, err := workspaces.WorkspaceList(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		if ws == nil {
			ws = []workspaces.WorkspaceView{}
		}
		return &struct{ Body []workspaces.WorkspaceView }{ws}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "add-workspace", Method: http.MethodPost, Path: "/api/workspaces", DefaultStatus: http.StatusCreated,
		Summary: "Add a workspace, as hh workspace add", Tags: []string{"workspaces"},
	}, func(_ context.Context, in *workspaceAddInput) (*workspaceOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := workspaces.WorkspaceAdd(ss, in.Body.Name, in.Body.Prefix)
		if err != nil {
			return nil, problemOf(err)
		}
		return workspaceOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-workspace", Method: http.MethodPatch, Path: "/api/workspaces/{id}",
		Summary: "Rename a workspace or change its prefix", Tags: []string{"workspaces"},
	}, func(_ context.Context, in *workspacePatchInput) (*workspaceOutput, error) {
		b := in.Body
		ss := s.session(in.Store)
		defer ss.Close()
		var r workspaces.WorkspaceResult
		var err error
		switch {
		case b.Name != nil && b.Prefix != nil:
			return nil, newProblem(http.StatusBadRequest, "workspace.one_change", "give name or prefix, not both")
		case b.Name != nil:
			r, err = workspaces.WorkspaceRename(ss, in.ID, *b.Name)
		case b.Prefix != nil:
			r, err = workspaces.WorkspacePrefix(ss, in.ID, *b.Prefix)
		default:
			return nil, badRequest(errors.New("give name or prefix"))
		}
		if err != nil {
			return nil, problemOf(err)
		}
		return workspaceOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-workspace", Method: http.MethodDelete, Path: "/api/workspaces/{id}",
		Summary: "Remove an empty workspace, as hh workspace rm", Tags: []string{"workspaces"},
	}, func(_ context.Context, in *WorkspaceParam) (*workspaceOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := workspaces.WorkspaceRemove(ss, in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return workspaceOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-schema", Method: http.MethodGet, Path: "/api/schema",
		Summary: "Show schema.yaml", Tags: []string{"schema"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body schemas.SchemaView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		_, ix, err := ss.Open()
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body schemas.SchemaView }{schemas.SchemaViewOf(ix.Schema())}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "check-schema", Method: http.MethodGet, Path: "/api/schema/check",
		Summary: "Check the schema and the tasks, as hh schema check", Tags: []string{"schema"},
	}, func(_ context.Context, in *StoreParam) (*findingsOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		fs, err := schemas.SchemaCheck(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		out := &findingsOutput{}
		out.Body.Findings = append([]usecase.Finding{}, fs...)
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "migrate-schema", Method: http.MethodPost, Path: "/api/schema/migrate",
		Summary: "Rename or merge a status, kind, property or value, as hh migrate", Tags: []string{"schema"},
	}, func(_ context.Context, in *migrateInput) (*migrateOutput, error) {
		b := in.Body
		m := schema.Migration{Target: schema.Target(b.Target), Prop: b.Prop, Old: b.Old, New: b.New}
		if (m.Target == schema.TargetValue) != (b.Prop != "") {
			return nil, badRequest(errors.New("prop is given with target value, and only then"))
		}
		ss := s.session(in.Store)
		defer ss.Close()
		ix, plan, err := schemas.Migrate(ss, m, b.DryRun)
		if err != nil {
			return nil, problemOf(err)
		}
		out := &migrateOutput{}
		out.Body.Changed, out.Body.Skipped = []*usecase.TaskView{}, []string{}
		out.Body.Commit = commitPtr(plan.Commit)
		for _, t := range plan.Changed {
			e, ok := ix.Task(t.ULID)
			if b.DryRun || !ok {
				e = index.EntryOf(t) // the task as migrated, as the CLI prints it
			}
			out.Body.Changed = append(out.Body.Changed, usecase.ListViewOf(ix, e))
		}
		for _, u := range plan.Skipped {
			out.Body.Skipped = append(out.Body.Skipped, u.String())
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-stores", Method: http.MethodGet, Path: "/api/stores",
		Summary: "List the stores, as hh store list", Tags: []string{"stores"},
	}, func(context.Context, *struct{}) (*storesOutput, error) {
		ss := s.session("")
		defer ss.Close()
		list, err := stores.StoreList(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		def := ""
		if _, e, err := ss.Entry(); err == nil { // the store the CLI picks without --store
			def = e.Name
		}
		out := &storesOutput{Body: []storeItem{}}
		for _, v := range list {
			out.Body = append(out.Body, storeItem{StoreView: v, Default: v.Name == def})
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "init-store", Method: http.MethodPost, Path: "/api/stores", DefaultStatus: http.StatusCreated,
		Summary: "Make a store, as hh store init", Tags: []string{"stores"},
	}, func(_ context.Context, in *storeInitInput) (*storeInitOutput, error) {
		b := in.Body
		ss := s.session("")
		defer ss.Close()
		r, err := stores.StoreInit(ss, stores.StoreInitInput{Name: b.Name, Prefix: b.Prefix, Workspace: b.Workspace, Remotes: b.Remotes, Account: b.Account})
		if err != nil {
			return nil, problemOf(err)
		}
		out := &storeInitOutput{}
		out.Body.Name, out.Body.Path = r.Name, r.Path
		out.Body.Remotes, out.Body.Pushed = append([]string{}, r.Remotes...), append([]string{}, r.Pushed...)
		out.Body.Workspace = workspaces.WorkspaceView{ULID: r.Workspace.ULID.String(), Name: r.Workspace.Name, IDPrefix: r.Workspace.IDPrefix, PrefixSet: r.Workspace.PrefixSet}
		// StoreInit leaves the time of the prefix out of its workspace; the store has it.
		made := s.session(r.Name)
		defer made.Close()
		if _, ws, err := workspaces.WorkspaceList(made); err == nil && len(ws) == 1 {
			out.Body.Workspace = ws[0]
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "doctor", Method: http.MethodGet, Path: "/api/doctor",
		Summary: "Check the store, as hh doctor", Tags: []string{"maintenance"},
	}, func(_ context.Context, in *StoreParam) (*doctorOutput, error) {
		return s.doctor(in.Store, false)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "doctor-fix", Method: http.MethodPost, Path: "/api/doctor/fix",
		Summary: "Check the store and repair what can be, as hh doctor --fix", Tags: []string{"maintenance"},
	}, func(_ context.Context, in *StoreParam) (*doctorOutput, error) {
		return s.doctor(in.Store, true)
	})
}

func (s *Server) doctor(store string, fix bool) (*doctorOutput, error) {
	ss := s.session(store)
	defer ss.Close()
	r, err := maintenance.Doctor(ss, fix)
	if err != nil {
		return nil, problemOf(err)
	}
	for i := range r {
		if r[i].Findings == nil {
			r[i].Findings = []usecase.Finding{}
		}
		if r[i].Repaired == nil {
			r[i].Repaired = []maintenance.Fixed{}
		}
	}
	return &doctorOutput{Body: r}, nil
}
