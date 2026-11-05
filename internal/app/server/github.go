package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
)

type linkInput struct {
	TaskParam
	Body struct {
		PR    string `json:"pr,omitempty" doc:"The pull request, as [<host>/]owner/repo#n; not with issue"`
		Issue string `json:"issue,omitempty" doc:"The issue, as [<host>/]owner/repo#n; not with pr"`
		Sync  string `json:"sync,omitempty" enum:"mirror,push,link" doc:"How an issue syncs; on an issue linked already it changes the mode. Without it, issue_fields.default_sync"`
	}
}

type unlinkInput struct {
	TaskParam
	PR    string `query:"pr" doc:"The pull request, as [<host>/]owner/repo#n; not with issue"`
	Issue string `query:"issue" doc:"The issue, as [<host>/]owner/repo#n; not with pr"`
}

type createIssueInput struct {
	TaskParam
	Body struct {
		Repo string `json:"repo,omitempty" doc:"The repository, as [<host>/]owner/repo; without it, the one the task names. A task with none or several needs it"`
	}
}

type githubImportInput struct {
	StoreParam
	Body struct {
		Repo      string `json:"repo" minLength:"1" doc:"The repository, as [<host>/]owner/repo"`
		Label     string `json:"label" minLength:"1" doc:"The label of the issues to take in"`
		Parent    string `json:"parent,omitempty" doc:"The task to put them under"`
		Workspace string `json:"workspace,omitempty" doc:"The workspace; needed without parent when the store has several"`
		DryRun    bool   `json:"dry_run,omitempty" doc:"Only count them"`
	}
}

type repoAccountsBody struct {
	Rules []accounts.RepoAccountView `json:"rules" nullable:"false" doc:"The rules in the order they apply: this list replaces the one there is"`
}

// linkFrom is what the body or the query of a link names. One of the two is
// the whole of what hh link and hh unlink take.
func linkFrom(pr, issue, sync string) (integration.LinkInput, error) {
	if (pr == "") == (issue == "") {
		return integration.LinkInput{}, errors.New("give pr or issue")
	}
	return integration.LinkInput{PR: pr, Issue: issue, Sync: sync}, nil
}

func (s *Server) registerGitHub() {
	huma.Register(s.api, huma.Operation{
		OperationID: "link-task", Method: http.MethodPost, Path: "/api/tasks/{id}/links",
		Summary: "Link a pull request or an issue, or change how an issue syncs, as hh link", Tags: []string{"github"},
	}, func(_ context.Context, in *linkInput) (*taskWriteOutput, error) {
		li, err := linkFrom(in.Body.PR, in.Body.Issue, in.Body.Sync)
		if err != nil {
			return nil, badRequest(err)
		}
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := integration.Link(ss, in.ID, li)
		if err != nil {
			return nil, problemOf(err)
		}
		return writeOut(r)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "unlink-task", Method: http.MethodDelete, Path: "/api/tasks/{id}/links",
		Summary: "Stop using a link, as hh unlink", Tags: []string{"github"},
	}, func(_ context.Context, in *unlinkInput) (*taskWriteOutput, error) {
		li, err := linkFrom(in.PR, in.Issue, "")
		if err != nil {
			return nil, badRequest(err)
		}
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := integration.Unlink(ss, in.ID, li)
		if err != nil {
			return nil, problemOf(err)
		}
		return writeOut(r)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "create-issue", Method: http.MethodPost, Path: "/api/tasks/{id}/issue", DefaultStatus: http.StatusCreated,
		Summary: "Open an issue for a task and link it in push mode, as hh issue", Tags: []string{"github"},
	}, func(_ context.Context, in *createIssueInput) (*struct{ Body integration.IssueWrite }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := integration.CreateIssue(ss, in.ID, integration.IssueInput{Repo: in.Body.Repo})
		if err != nil {
			return nil, problemOf(err)
		}
		w, err := r.Write()
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body integration.IssueWrite }{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-github", Method: http.MethodGet, Path: "/api/github",
		Summary: "How the reads of each repository go: of the store given, or of every store", Tags: []string{"github"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []usecase.GitHubRepoView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		out, err := usecase.GitHubStatus(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []usecase.GitHubRepoView }{out}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "import-issues", Method: http.MethodPost, Path: "/api/github/import",
		Summary: "Make a task in mirror mode for each open issue with a label, as hh gh import", Tags: []string{"github"},
	}, func(_ context.Context, in *githubImportInput) (*struct{ Body integration.ImportResult }, error) {
		b := in.Body
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := integration.ImportIssues(ss, integration.ImportInput{
			Repo: b.Repo, Label: b.Label, Parent: b.Parent, Workspace: b.Workspace, DryRun: b.DryRun,
		})
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body integration.ImportResult }{r}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-repo-accounts", Method: http.MethodGet, Path: "/api/repo-accounts",
		Summary: "List repo_accounts of config.yaml, in the order they apply", Tags: []string{"accounts"},
	}, func(context.Context, *struct{}) (*struct{ Body repoAccountsBody }, error) {
		ss := s.session("")
		defer ss.Close()
		rules, err := accounts.RepoAccounts(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body repoAccountsBody }{repoAccountsBody{rules}}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "put-repo-accounts", Method: http.MethodPut, Path: "/api/repo-accounts",
		Summary: "Replace repo_accounts of config.yaml", Tags: []string{"accounts"},
	}, func(_ context.Context, in *struct{ Body repoAccountsBody }) (*struct{ Body repoAccountsBody }, error) {
		ss := s.session("")
		defer ss.Close()
		rules, err := accounts.SetRepoAccounts(ss, in.Body.Rules)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body repoAccountsBody }{repoAccountsBody{rules}}, nil
	})
}
