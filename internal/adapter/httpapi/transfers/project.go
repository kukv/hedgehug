package transfers

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	apptransfers "github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ProjectTransferTarget は、プロジェクトの移す先の候補のワークスペース(A5b-22)。
type ProjectTransferTarget struct {
	StoreID       string `json:"storeId"`
	StoreName     string `json:"storeName"`
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceName string `json:"workspaceName"`
}

// ProjectNotCarried は、移動先に引き継げないルール(A5b-8)。文は画面が作る。
type ProjectNotCarried struct {
	Kind    string   `json:"kind" doc:"ルールの種類"`
	Name    string   `json:"name" doc:"ルールの名前"`
	Missing []string `json:"missing" doc:"移動先にない名前"`
	Count   int      `json:"count" doc:"引き継げないルールの数"`
}

// ProjectTransferPreview は、プロジェクトの移動のダイアログの予告(A5b-23)。
type ProjectTransferPreview struct {
	Items                int                   `json:"items" doc:"削除されていないアイテムの数"`
	Deleted              int                   `json:"deleted" doc:"削除済みのアイテムの数"`
	Prefix               string                `json:"prefix" doc:"移動先の接頭辞"`
	Source               string                `json:"source" doc:"移動元の文"`
	Destination          string                `json:"destination" doc:"移動先の文"`
	CrossStore           bool                  `json:"crossStore" doc:"別の Store への移動か"`
	RemoteNotice         bool                  `json:"remoteNotice" doc:"別の Store で、移動先に remote がなく移動元にあるか"`
	DestinationHasRemote bool                  `json:"destinationHasRemote" doc:"移動先の Store に remote があるか"`
	HasRules             bool                  `json:"hasRules" doc:"プロジェクト別のルールか専用ルールがあるか"`
	NotCarried           []ProjectNotCarried   `json:"notCarried" doc:"移動先に引き継げないルール"`
	Replacements         []TransferReplacement `json:"replacements" doc:"行の並びは移動元のワークスペースのステータスの並び"`
}

// TransferProjectBody は、プロジェクトの移動の本文(A5b-22)。
type TransferProjectBody struct {
	ToStore       string            `json:"toStore" doc:"移動先の Store の ID"`
	ToWorkspace   string            `json:"toWorkspace" doc:"移動先のワークスペースの内部 ID"`
	StatusChoices map[string]string `json:"statusChoices" doc:"ステータスの置き換え(移動元のステータスの内部 ID → 移動先のステータスの内部 ID)"`
}

// ProjectTransferResult は、プロジェクトの移動の結果。
type ProjectTransferResult struct {
	ToStore     string `json:"toStore" doc:"移動先の Store の ID"`
	ProjectID   string `json:"projectId" doc:"移したプロジェクトの内部 ID(変わらない)"`
	Display     string `json:"display" doc:"移したプロジェクトの新しい ID(MB-P2)"`
	Destination string `json:"destination" doc:"移動先の文"`
}

// LocatedProject は、画面のアドレスのプロジェクトの ID の解決の結果(A5b-17)。文は画面が作る。
type LocatedProject struct {
	Kind       string            `json:"kind" enum:"found,gone"`
	StoreID    string            `json:"storeId,omitempty" doc:"found のとき。開くプロジェクトの Store"`
	ProjectID  string            `json:"projectId,omitempty" doc:"found のとき"`
	MovedFrom  *LocatedMovedFrom `json:"movedFrom,omitempty" doc:"found で、転送記録から見つけたとき"`
	Reason     string            `json:"reason,omitempty" enum:"deleted,purged,hidden,elsewhere" doc:"gone のとき"`
	Old        string            `json:"old,omitempty" doc:"gone のとき。旧 ID"`
	At         string            `json:"at,omitempty" doc:"gone のとき。旧 ID が一致した記録の日時"`
	NewDisplay string            `json:"newDisplay,omitempty" doc:"gone のとき。旧 ID が一致した記録の新 ID"`
	StoreName  string            `json:"storeName,omitempty" doc:"gone の hidden・elsewhere のとき。表示中でない Store の名前"`
}

type projectInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type projectPreviewInput struct {
	Store       string `path:"store"`
	Project     string `path:"project"`
	ToStore     string `query:"toStore" required:"true"`
	ToWorkspace string `query:"toWorkspace" required:"true"`
}

type projectTransferInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    TransferProjectBody
}

type locateProjectInput struct {
	Store string `query:"store" required:"true" doc:"画面のアドレスの Store 名"`
	ID    string `query:"id" required:"true" doc:"画面のアドレスのプロジェクトの ID(HH-P1)"`
}

type projectTargetsOutput struct{ Body []ProjectTransferTarget }

type projectPreviewOutput struct{ Body ProjectTransferPreview }

type projectTransferOutput struct{ Body ProjectTransferResult }

type locateProjectOutput struct{ Body LocatedProject }

func registerProject(api huma.API, svc apptransfers.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-project-transfer-targets", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/transfer-targets", Summary: "プロジェクトの移す先の候補のワークスペースを返す"},
		func(ctx context.Context, in *projectInput) (*projectTargetsOutput, error) {
			ts, err := svc.ProjectTargets(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &projectTargetsOutput{Body: []ProjectTransferTarget{}}
			for _, t := range ts {
				out.Body = append(out.Body, ProjectTransferTarget{StoreID: string(t.Store.ID), StoreName: t.Store.Name, WorkspaceID: string(t.Workspace.ID), WorkspaceName: t.Workspace.Name})
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-project-transfer-preview", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/transfer-preview", Summary: "プロジェクトの移動のダイアログの予告を返す"},
		func(ctx context.Context, in *projectPreviewInput) (*projectPreviewOutput, error) {
			to := apptransfers.ProjectMoveInput{ToStore: port.StoreID(in.ToStore), ToWorkspace: workspace.ID(in.ToWorkspace)}
			p, err := svc.ProjectPreview(ctx, port.StoreID(in.Store), project.ID(in.Project), to)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &projectPreviewOutput{Body: toProjectPreview(p)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "transfer-project", Method: http.MethodPost, Path: "/api/stores/{store}/projects/{project}/transfer", Summary: "プロジェクトをアイテムごと別のワークスペース・別の Store へ移す"},
		func(ctx context.Context, in *projectTransferInput) (*projectTransferOutput, error) {
			m, err := svc.MoveProject(ctx, port.StoreID(in.Store), project.ID(in.Project), toProjectMoveInput(in.Body))
			// 取り消しもできなかった(両方の Store に残る)は、送れなかったより先に見る。
			if ue := (*apptransfers.UndoError)(nil); errors.As(err, &ue) {
				return nil, huma.Error409Conflict(ue.Error())
			}
			if se := (*apptransfers.SendError)(nil); errors.As(err, &se) {
				return nil, huma.Error422UnprocessableEntity(se.Error())
			}
			if err != nil {
				return nil, apierr.From(err)
			}
			return &projectTransferOutput{Body: ProjectTransferResult{ToStore: string(m.ToStore), ProjectID: string(m.Project), Display: m.Display, Destination: m.Destination}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "locate-project", Method: http.MethodGet, Path: "/api/locate-project", Summary: "画面のアドレスのプロジェクトの ID(旧 ID を含む)が指すプロジェクトを探す"},
		func(ctx context.Context, in *locateProjectInput) (*locateProjectOutput, error) {
			l, err := svc.LocateProject(ctx, in.Store, in.ID)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &locateProjectOutput{Body: toLocatedProject(l)}, nil
		})
}

func toProjectMoveInput(b TransferProjectBody) apptransfers.ProjectMoveInput {
	in := apptransfers.ProjectMoveInput{ToStore: port.StoreID(b.ToStore), ToWorkspace: workspace.ID(b.ToWorkspace), StatusChoices: map[change.ID]change.ID{}}
	for from, to := range b.StatusChoices {
		in.StatusChoices[change.ID(from)] = change.ID(to)
	}
	return in
}

func toProjectPreview(p apptransfers.ProjectPreview) ProjectTransferPreview {
	out := ProjectTransferPreview{
		Items: p.Items, Deleted: p.Deleted, Prefix: p.Prefix, Source: p.Source, Destination: p.Destination,
		CrossStore: p.CrossStore, RemoteNotice: p.RemoteNotice, DestinationHasRemote: p.DestinationHasRemote, HasRules: p.HasRules,
		NotCarried: []ProjectNotCarried{}, Replacements: []TransferReplacement{},
	}
	for _, n := range p.NotCarried {
		missing := n.Missing
		if missing == nil {
			missing = []string{}
		}
		out.NotCarried = append(out.NotCarried, ProjectNotCarried{Kind: n.Kind, Name: n.Name, Missing: missing, Count: n.Count})
	}
	for _, r := range p.Replacements {
		row := TransferReplacement{Status: string(r.Status), Name: r.Name, Count: r.Count, Choices: []TransferChoice{}, Default: string(r.Default)}
		for _, c := range r.Choices {
			row.Choices = append(row.Choices, TransferChoice{ID: string(c.ID), Name: c.Name})
		}
		out.Replacements = append(out.Replacements, row)
	}
	return out
}

func toLocatedProject(l apptransfers.LocatedProject) LocatedProject {
	if !l.Found {
		g := l.Gone
		return LocatedProject{Kind: "gone", Reason: g.Reason, Old: g.Old, At: g.At.Format(change.TimeLayout), NewDisplay: g.NewDisplay, StoreName: g.StoreName}
	}
	out := LocatedProject{Kind: "found", StoreID: string(l.Store), ProjectID: string(l.Project)}
	if m := l.MovedFrom; m != nil {
		out.MovedFrom = &LocatedMovedFrom{Old: m.Old, At: m.At.Format(change.TimeLayout), Display: m.Display, Current: m.Current}
	}
	return out
}
