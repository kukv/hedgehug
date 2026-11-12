package apptest

import (
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// サンプルデータの内部 ID。トラックのテストと e2e が参照してよい。
const (
	SampleWorkspace workspace.ID = "01SAMPLEWS0000000000000001"
	SampleProject   project.ID   = "01SAMPLEPR0000000000000001"
	SampleEpic      item.ID      = "01SAMPLEIT0000000000000001"
	SampleTask      item.ID      = "01SAMPLEIT0000000000000002"
)

// サンプルのワークスペースの値の内部 ID(DefaultSettings が振る順: ステータス 5、優先度 5、関連システム 6)。
const (
	SampleStatusTodo       change.ID = "01SAMPLEVAL000000000000001"
	SampleStatusInProgress change.ID = "01SAMPLEVAL000000000000002"
	SampleStatusDone       change.ID = "01SAMPLEVAL000000000000003"
	SampleStatusPending    change.ID = "01SAMPLEVAL000000000000004"
	SampleStatusCancel     change.ID = "01SAMPLEVAL000000000000005"
	SamplePriorityMedium   change.ID = "01SAMPLEVAL000000000000008"
)

func sampleValueIDs() func() change.ID {
	n := 0
	return func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01SAMPLEVAL%015d", n))
	}
}

// Sample は、メンバー 2 人(taro・hanako)、ワークスペース「Web」(HH)、プロジェクト HH-P1、
// Epic HH-1 とその下の Task HH-2 を、1 回の保存で書く。
func Sample(ctx context.Context, s port.Stores, id port.StoreID) error {
	ws := workspace.Workspace{ID: SampleWorkspace, Name: "Web", Prefix: "HH"}
	pr := project.Project{ID: SampleProject, Place: project.Place{Workspace: ws.ID, Number: 1}, Rank: "V", Title: "ログイン", Owner: "taro", Status: project.Todo}
	epic := item.Item{ID: SampleEpic, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Number: 1}, Kind: item.Epic, Rank: rank.First, Title: "認証", Watchers: []string{"taro"}}
	parent := epic.ID
	task := item.Item{ID: SampleTask, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Parent: &parent, Number: 2}, Kind: item.Task, Rank: rank.First, Title: "ログイン画面を作る", Watchers: []string{"taro"}}
	op := port.Operation{Actor: change.User("taro"), Summary: "サンプルデータを作成"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(sampleValueIDs()))
		if err != nil {
			return err
		}
		return errors.Join(
			tx.WriteMembers().Append(member.Init(), member.Join("taro"), member.Join("hanako")),
			tx.WriteWorkspaces().Append(ws.ID, d,
				numbering.Draft(numbering.Projects, 1),
				numbering.Draft(numbering.Items, 1),
				numbering.Draft(numbering.Items, 2)),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
			tx.WriteItems().Append(epic.ID, item.Create(epic)),
			tx.WriteItems().Append(task.ID, item.Create(task)),
		)
	})
}
