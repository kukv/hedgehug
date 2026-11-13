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
	SampleSystemWeb        change.ID = "01SAMPLEVAL000000000000011"
)

// Mobile・Team のサンプルの内部 ID。Sample と重ならない(別の Store へ移すとき、同じ内部 ID があると写せないため)。
const (
	SampleMobileWorkspace workspace.ID = "01SAMPLEWS0000000000000002"
	SampleMobileProject   project.ID   = "01SAMPLEPR0000000000000002"
	SampleMobileEpic      item.ID      = "01SAMPLEIT0000000000000003"
	SampleTeamWorkspace   workspace.ID = "01SAMPLEWS0000000000000003"
	SampleTeamProject     project.ID   = "01SAMPLEPR0000000000000003"
	SampleTeamEpic        item.ID      = "01SAMPLEIT0000000000000004"
)

// 値の内部 ID の接頭辞(続けて 15 桁の連番)。ワークスペースごとに分ける。
const (
	sampleValues       = "01SAMPLEVAL"
	sampleMobileValues = "01SAMPLEMBV"
	sampleTeamValues   = "01SAMPLETMV"
)

// valueID は、値の内部 ID の接頭辞 prefix の n 番目(DefaultSettings が振る順。1 は Todo、8 は優先度 Medium)。
func valueID(prefix string, n int) change.ID {
	return change.ID(fmt.Sprintf("%s%015d", prefix, n))
}

func valueIDs(prefix string) func() change.ID {
	n := 0
	return func() change.ID {
		n++
		return valueID(prefix, n)
	}
}

// Sample は、メンバー 2 人(taro・hanako)、ワークスペース「Web」(HH)、プロジェクト HH-P1、
// Epic HH-1 とその下の Task HH-2 を、1 回の保存で書く。
func Sample(ctx context.Context, s port.Stores, id port.StoreID) error {
	ws := workspace.Workspace{ID: SampleWorkspace, Name: "Web", Prefix: "HH"}
	pr := project.Project{ID: SampleProject, Place: project.Place{Workspace: ws.ID, Number: 1}, Rank: "V", Title: "ログイン", Owner: "taro", Status: project.Todo}
	epic := item.Item{ID: SampleEpic, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Number: 1}, Kind: item.Epic, Rank: rank.First, Title: "認証", Watchers: []string{"taro"}, Status: SampleStatusTodo, Priority: SamplePriorityMedium}
	parent := epic.ID
	task := item.Item{ID: SampleTask, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Parent: &parent, Number: 2}, Kind: item.Task, Rank: rank.First, Title: "ログイン画面を作る", Watchers: []string{"taro"}, Status: SampleStatusTodo, Priority: SamplePriorityMedium}
	op := port.Operation{Actor: change.User("taro"), Summary: "サンプルデータを作成"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(valueIDs(sampleValues)))
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

// SampleMobile は、Sample のあとの Store id に、ワークスペース「Mobile」(MB)、プロジェクト MB-P1「アプリ」、
// Epic MB-1「iOS」を、1 回の保存で足す。メンバーは Sample のもの。
func SampleMobile(ctx context.Context, s port.Stores, id port.StoreID) error {
	op := port.Operation{Actor: change.User("taro"), Summary: "サンプルデータを作成"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		return writeSampleWorkspace(tx, sampleWorkspace{
			ws:      workspace.Workspace{ID: SampleMobileWorkspace, Name: "Mobile", Prefix: "MB"},
			project: SampleMobileProject, projectTitle: "アプリ", epic: SampleMobileEpic, epicTitle: "iOS", values: sampleMobileValues,
		})
	})
}

// SampleTeam は、空の Store id に、メンバー 2 人(taro・hanako)、ワークスペース「Team」(TM)、プロジェクト
// TM-P1「運営」、Epic TM-1「チーム運営」を、1 回の保存で書く。
func SampleTeam(ctx context.Context, s port.Stores, id port.StoreID) error {
	op := port.Operation{Actor: change.User("taro"), Summary: "サンプルデータを作成"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteMembers().Append(member.Init(), member.Join("taro"), member.Join("hanako")),
			writeSampleWorkspace(tx, sampleWorkspace{
				ws:      workspace.Workspace{ID: SampleTeamWorkspace, Name: "Team", Prefix: "TM"},
				project: SampleTeamProject, projectTitle: "運営", epic: SampleTeamEpic, epicTitle: "チーム運営", values: sampleTeamValues,
			}),
		)
	})
}

// sampleWorkspace は、ワークスペースと、その中のプロジェクト(番号 1)と Epic(番号 1)。values は値の内部 ID の接頭辞。
type sampleWorkspace struct {
	ws                      workspace.Workspace
	project                 project.ID
	projectTitle, epicTitle string
	epic                    item.ID
	values                  string
}

// writeSampleWorkspace は、w を既定の設定で書く。Epic は Todo・優先度 Medium で、taro がウォッチする。
func writeSampleWorkspace(tx port.Tx, w sampleWorkspace) error {
	d, err := workspace.CreateWithSettings(w.ws, workspace.DefaultSettings(valueIDs(w.values)))
	if err != nil {
		return err
	}
	pr := project.Project{ID: w.project, Place: project.Place{Workspace: w.ws.ID, Number: 1}, Rank: "V", Title: w.projectTitle, Owner: "taro", Status: project.Todo}
	epic := item.Item{
		ID: w.epic, Place: item.Place{Workspace: w.ws.ID, Project: pr.ID, Number: 1}, Kind: item.Epic, Rank: rank.First, Title: w.epicTitle,
		Watchers: []string{"taro"}, Status: valueID(w.values, 1), Priority: valueID(w.values, 8),
	}
	return errors.Join(
		tx.WriteWorkspaces().Append(w.ws.ID, d, numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1)),
		tx.WriteProjects().Append(pr.ID, project.Create(pr)),
		tx.WriteItems().Append(epic.ID, item.Create(epic)),
	)
}

// DeleteItems は、アイテム root と、その下の below を、1 回の保存で削除済みにする(テスト用。A4b)。
// root は利用者が選んだもの(root: true)、below は下にあるもの(root: false)。
func DeleteItems(ctx context.Context, s port.Stores, id port.StoreID, root item.ID, below ...item.ID) error {
	op := port.Operation{Actor: change.User("taro"), Summary: "アイテムを削除"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		errs := []error{tx.WriteItems().Append(root, change.NewDeleteEntity(true))}
		for _, b := range below {
			errs = append(errs, tx.WriteItems().Append(b, change.NewDeleteEntity(false)))
		}
		return errors.Join(errs...)
	})
}

// DeleteWorkspace は、ワークスペースを削除済みにする(テスト用。B5b)。中のプロジェクト・アイテムには書かない。
func DeleteWorkspace(ctx context.Context, s port.Stores, id port.StoreID, ws workspace.ID) error {
	op := port.Operation{Actor: change.User("taro"), Summary: "ワークスペースを削除"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(ws, change.NewDeleteEntity(true))
	})
}

// DeleteProject は、プロジェクトを削除済みにする(テスト用。A4b)。下のアイテムには書かない。
func DeleteProject(ctx context.Context, s port.Stores, id port.StoreID, p project.ID) error {
	op := port.Operation{Actor: change.User("taro"), Summary: "プロジェクトを削除"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		return tx.WriteProjects().Append(p, change.NewDeleteEntity(true))
	})
}
