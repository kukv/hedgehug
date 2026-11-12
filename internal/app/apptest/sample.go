package apptest

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// サンプルデータの内部 ID。トラックのテストと e2e が参照してよい。
const (
	SampleWorkspace workspace.ID = "01SAMPLEWS0000000000000001"
	SampleProject   project.ID   = "01SAMPLEPR0000000000000001"
	SampleEpic      item.ID      = "01SAMPLEIT0000000000000001"
	SampleTask      item.ID      = "01SAMPLEIT0000000000000002"
)

// Sample は、メンバー 2 人(taro・hanako)、ワークスペース「Web」(HH)、プロジェクト HH-P1、
// Epic HH-1 とその下の Task HH-2 を、1 回の保存で書く。
func Sample(ctx context.Context, s port.Stores, id port.StoreID) error {
	ws := workspace.Workspace{ID: SampleWorkspace, Name: "Web", Prefix: "HH"}
	pr := project.Project{ID: SampleProject, Place: project.Place{Workspace: ws.ID, Number: 1}, Title: "ログイン"}
	epic := item.Item{ID: SampleEpic, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Number: 1}, Kind: item.Epic, Rank: "a", Title: "認証"}
	parent := epic.ID
	task := item.Item{ID: SampleTask, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Parent: &parent, Number: 2}, Kind: item.Task, Rank: "a", Title: "ログイン画面を作る"}
	op := port.Operation{Actor: change.User("taro"), Summary: "サンプルデータを作成"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		d, err := workspace.Create(ws)
		if err != nil {
			return err
		}
		return errors.Join(
			tx.WriteMembers().Append(member.Init(), member.Join("taro"), member.Join("hanako")),
			tx.WriteWorkspaces().Append(ws.ID, d),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
			tx.WriteItems().Append(epic.ID, item.Create(epic)),
			tx.WriteItems().Append(task.ID, item.Create(task)),
		)
	})
}
