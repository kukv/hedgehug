package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// CreateInput は、プロジェクトの作成ダイアログの入力。Status が空なら Todo。
type CreateInput struct {
	Store     port.StoreID
	Workspace workspace.ID
	Title     string
	Owner     string
	Status    project.Status
	Start     string
	End       string
}

// Create は、プロジェクトを作る。番号はワークスペースの連番の次、並びはワークスペースの中の末尾。
func (s Service) Create(ctx context.Context, in CreateInput) (project.Project, error) {
	me, err := s.Identity.Me(ctx, in.Store)
	if err != nil {
		return project.Project{}, err
	}
	if in.Status != "" && !project.ValidStatus(in.Status) {
		return project.Project{}, fmt.Errorf("unknown project status %q", in.Status)
	}
	title := strings.TrimSpace(in.Title)
	// 保存するタイトルは前後の空白だけを除く(02)。アクティビティの 1 行は 1 行でなければならないので、
	// 改行を含む連続する空白を 1 つの空白にまとめて書く。
	oneLine := strings.Join(strings.Fields(title), " ")
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("プロジェクト「%s」を作成", oneLine)}
	var created project.Project
	err = s.Stores.Write(ctx, in.Store, op, func(tx port.Tx) error {
		if _, err := port.LiveWorkspace(tx, in.Workspace); err != nil {
			return err
		}
		siblings, err := port.LiveProjects(tx, in.Workspace)
		if err != nil {
			return err
		}
		if err := project.CheckCreate(in.Title, in.Start, in.End, siblings); err != nil {
			return err
		}
		wsLog, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: in.Workspace})
		if err != nil {
			return err
		}
		created = newProject(s.IDs.New(), in, title, numbering.Next(wsLog, numbering.Projects), siblings)
		return errors.Join(
			tx.WriteWorkspaces().Append(in.Workspace, numbering.Draft(numbering.Projects, created.Place.Number)),
			tx.WriteProjects().Append(created.ID, project.Create(created)),
		)
	})
	return created, err
}

func newProject(id project.ID, in CreateInput, title string, number int, siblings []project.Project) project.Project {
	last := ""
	if len(siblings) > 0 {
		last = siblings[len(siblings)-1].Rank
	}
	status := in.Status
	if status == "" {
		status = project.Todo
	}
	return project.Project{
		ID: id, Place: project.Place{Workspace: in.Workspace, Number: number}, Rank: rank.After(last),
		Title: title, Owner: in.Owner, Status: status, Start: in.Start, End: in.End,
	}
}
