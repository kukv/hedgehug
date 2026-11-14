package transfers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	apptransfers "github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// WorkspaceTransferTarget は、ワークスペースの移す先の候補の Store(B5c-19)。
type WorkspaceTransferTarget struct {
	StoreID   string `json:"storeId"`
	StoreName string `json:"storeName"`
}

// WorkspaceTransferTargets は、ワークスペースの移す先の候補(B5c-19)。
type WorkspaceTransferTargets struct {
	Targets    []WorkspaceTransferTarget `json:"targets" doc:"表示中で閲覧のみでなく自分でない Store(Store の並び)"`
	HiddenOnly bool                      `json:"hiddenOnly" doc:"候補が 0 件で、非表示の Store なら移せるか"`
}

// WorkspaceTransferPerson は、移動先のメンバーに足す人。
type WorkspaceTransferPerson struct {
	Login string `json:"login"`
	Name  string `json:"name" doc:"表示名"`
}

// WorkspaceTransferPreview は、ワークスペースの移動の確認ダイアログの予告(B5c-20)。
type WorkspaceTransferPreview struct {
	Source               string                    `json:"source" doc:"移動元の Store 名"`
	Destination          string                    `json:"destination" doc:"移動先の Store 名"`
	Projects             int                       `json:"projects" doc:"プロジェクトの数"`
	Items                int                       `json:"items" doc:"削除されていないアイテムの数"`
	Deleted              int                       `json:"deleted" doc:"削除済みのアイテムの数"`
	People               []WorkspaceTransferPerson `json:"people" doc:"移動先のメンバーに足す人(小文字のログイン名の順)"`
	RemoteNotice         bool                      `json:"remoteNotice" doc:"移動先に remote がなく移動元にあるか"`
	DestinationHasRemote bool                      `json:"destinationHasRemote" doc:"移動先の Store に remote があるか"`
}

// TransferWorkspaceBody は、ワークスペースの移動の本文(B5c-19)。
type TransferWorkspaceBody struct {
	ToStore string `json:"toStore" doc:"移動先の Store の ID"`
}

// WorkspaceTransferResult は、ワークスペースの移動の結果。
type WorkspaceTransferResult struct {
	ToStore     string `json:"toStore" doc:"移動先の Store の ID"`
	ToStoreName string `json:"toStoreName" doc:"移動先の Store 名"`
	WorkspaceID string `json:"workspaceId" doc:"移したワークスペースの内部 ID(変わらない)"`
	Prefix      string `json:"prefix" doc:"移したワークスペースの接頭辞"`
}

// LocatedWorkspace は、画面のアドレスのワークスペースの接頭辞の解決の結果(B5c-13)。文は画面が作る。
type LocatedWorkspace struct {
	Kind        string            `json:"kind" enum:"found,gone"`
	StoreID     string            `json:"storeId,omitempty" doc:"found のとき。開くワークスペースの Store"`
	WorkspaceID string            `json:"workspaceId,omitempty" doc:"found のとき"`
	MovedFrom   *LocatedMovedFrom `json:"movedFrom,omitempty" doc:"found で、転送記録から見つけたとき"`
	Reason      string            `json:"reason,omitempty" enum:"deleted,purged,hidden,elsewhere" doc:"gone のとき"`
	Old         string            `json:"old,omitempty" doc:"gone のとき。画面の接頭辞"`
	At          string            `json:"at,omitempty" doc:"gone のとき。一致した記録の日時"`
	NewDisplay  string            `json:"newDisplay,omitempty" doc:"gone のとき。記録の移動先の Store 名"`
	StoreName   string            `json:"storeName,omitempty" doc:"gone の hidden・elsewhere のとき。表示中でない Store の名前"`
}

type workspaceInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type workspacePreviewInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	ToStore   string `query:"toStore" required:"true"`
}

type workspaceTransferInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      TransferWorkspaceBody
}

type locateWorkspaceInput struct {
	Store  string `query:"store" required:"true" doc:"画面のアドレスの Store 名"`
	Prefix string `query:"prefix" required:"true" doc:"画面のアドレスのワークスペースの接頭辞"`
}

type workspaceTargetsOutput struct{ Body WorkspaceTransferTargets }

type workspacePreviewOutput struct{ Body WorkspaceTransferPreview }

type workspaceTransferOutput struct{ Body WorkspaceTransferResult }

type locateWorkspaceOutput struct{ Body LocatedWorkspace }

func registerWorkspace(api huma.API, svc apptransfers.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-workspace-transfer-targets", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}/transfer-targets", Summary: "ワークスペースの移す先の候補の Store を返す"},
		func(ctx context.Context, in *workspaceInput) (*workspaceTargetsOutput, error) {
			ts, err := svc.WorkspaceTargets(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &workspaceTargetsOutput{Body: WorkspaceTransferTargets{Targets: []WorkspaceTransferTarget{}, HiddenOnly: ts.HiddenOnly}}
			for _, s := range ts.Stores {
				out.Body.Targets = append(out.Body.Targets, WorkspaceTransferTarget{StoreID: string(s.ID), StoreName: s.Name})
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-workspace-transfer-preview", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}/transfer-preview", Summary: "ワークスペースの移動の確認ダイアログの予告を返す"},
		func(ctx context.Context, in *workspacePreviewInput) (*workspacePreviewOutput, error) {
			p, err := svc.WorkspacePreview(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), port.StoreID(in.ToStore))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &workspacePreviewOutput{Body: toWorkspacePreview(p)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "transfer-workspace", Method: http.MethodPost, Path: "/api/stores/{store}/workspaces/{workspace}/transfer", Summary: "ワークスペースを中身ごと別の Store へ移す"},
		func(ctx context.Context, in *workspaceTransferInput) (*workspaceTransferOutput, error) {
			m, err := svc.MoveWorkspace(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), port.StoreID(in.Body.ToStore))
			if err != nil {
				return nil, moveError(err)
			}
			return &workspaceTransferOutput{Body: WorkspaceTransferResult{ToStore: string(m.ToStore), ToStoreName: m.ToStoreName, WorkspaceID: string(m.Workspace), Prefix: m.Prefix}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "locate-workspace", Method: http.MethodGet, Path: "/api/locate-workspace", Summary: "画面のアドレスのワークスペースの接頭辞(旧い接頭辞・移動元のアドレスを含む)が指すワークスペースを探す"},
		func(ctx context.Context, in *locateWorkspaceInput) (*locateWorkspaceOutput, error) {
			l, err := svc.LocateWorkspace(ctx, in.Store, in.Prefix)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &locateWorkspaceOutput{Body: toLocatedWorkspace(l)}, nil
		})
}

func toWorkspacePreview(p apptransfers.WorkspacePreview) WorkspaceTransferPreview {
	out := WorkspaceTransferPreview{
		Source: p.Source, Destination: p.Destination, Projects: p.Counts.Projects, Items: p.Counts.Items, Deleted: p.Counts.Deleted,
		People: []WorkspaceTransferPerson{}, RemoteNotice: p.RemoteNotice, DestinationHasRemote: p.DestinationHasRemote,
	}
	for _, m := range p.People {
		out.People = append(out.People, WorkspaceTransferPerson{Login: m.Login, Name: m.Name})
	}
	return out
}

func toLocatedWorkspace(l apptransfers.LocatedWorkspace) LocatedWorkspace {
	if !l.Found {
		g := l.Gone
		return LocatedWorkspace{Kind: "gone", Reason: g.Reason, Old: g.Old, At: g.At.Format(change.TimeLayout), NewDisplay: g.NewDisplay, StoreName: g.StoreName}
	}
	out := LocatedWorkspace{Kind: "found", StoreID: string(l.Store), WorkspaceID: string(l.Workspace)}
	if m := l.MovedFrom; m != nil {
		out.MovedFrom = &LocatedMovedFrom{Old: m.Old, At: m.At.Format(change.TimeLayout), Display: m.Display, Current: m.Current}
	}
	return out
}
