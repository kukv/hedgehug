import type {
  Located,
  LocatedProject,
  ProjectTransferInput,
  ProjectTransferPreview,
  ProjectTransferResult,
  ProjectTransferTarget,
  TransferDestination,
  TransferInput,
  TransferPreview,
  TransferResult,
  TransferTarget,
} from "../model/transfer";

// 別のワークスペース・別の Store への移動(A5a・A5b)の口。実装は api パッケージにある。
// 失敗は、断りなら Refused(message は画面にそのまま出す文)、ないものなら NotFound、それ以外はそのまま投げる。
export interface TransfersPort {
  // targets は、移す先の候補(絞り込みと並びはサーバーが決める。A5a-13)。
  targets(storeId: string, itemId: string): Promise<TransferTarget[]>;
  // preview は、移動先でのダイアログの予告(A5a-14)。
  preview(storeId: string, itemId: string, to: TransferDestination): Promise<TransferPreview>;
  transfer(storeId: string, itemId: string, input: TransferInput): Promise<TransferResult>;
  // locate は、アドレスの Store 名と ID(旧 ID・旧い接頭辞でもよい)をいまの場所にする。ないものは NotFound。
  locate(storeName: string, id: string): Promise<Located>;
  // projectTargets は、プロジェクトを移す先の候補(A5b-16)。
  projectTargets(storeId: string, projectId: string): Promise<ProjectTransferTarget[]>;
  // projectPreview は、プロジェクトの移動の確認ダイアログの予告(A5b-23)。
  projectPreview(
    storeId: string,
    projectId: string,
    to: { storeId: string; workspaceId: string },
  ): Promise<ProjectTransferPreview>;
  // transferProject は、プロジェクトをアイテムごと移す。取り消しもできなかったときは TransferUndoFailed。
  transferProject(
    storeId: string,
    projectId: string,
    input: ProjectTransferInput,
  ): Promise<ProjectTransferResult>;
  // locateProject は、アドレスの Store 名とプロジェクトの ID(旧 ID でもよい)をいまの場所にする。ないものは NotFound。
  locateProject(storeName: string, id: string): Promise<LocatedProject>;
}
