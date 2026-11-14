import type { Kind } from "./item";

// アイテム・プロジェクトの削除・移動などの操作(02「削除」「同じワークスペースの中での移動」)の入出力。

// DeletePreview は、アイテムの削除の確認ダイアログの中身。count は、下にある削除されていないものの件数、
// kinds は、その種類(重ねない)、retentionDays は、その Store の削除済みの保持日数。
export interface DeletePreview {
  count: number;
  kinds: Kind[];
  retentionDays: number;
}

// ProjectDeletePreview は、プロジェクトの削除の確認ダイアログの中身(削除されていないアイテムの件数と保持日数)。
export interface ProjectDeletePreview {
  items: number;
  retentionDays: number;
}

// MoveCandidate は、「移動…」の候補。kind が "project" なら Epic の移し先のプロジェクト、"item" なら新しい親。
// display は ID の表記(HH-3・HH-P2)、title は名前、projectId は移したあとのプロジェクトの内部 ID
// (アイテムの候補なら属するプロジェクト、プロジェクトの候補なら自分)、projectTitle は、アイテムの候補が
// 属するプロジェクトの名前(プロジェクトの候補では空)。
export interface MoveCandidate {
  kind: "item" | "project";
  id: string;
  display: string;
  title: string;
  projectId: string;
  projectTitle: string;
}

// MovePreviewStatus は、移動先が使わないステータスと、それを持つ移すアイテム(削除済みと自分自身を含む)の件数。
// default は、置き換えの初めの値(同じ分類の、移動先の既定のステータス)。
export interface MovePreviewStatus {
  status: string;
  count: number;
  default: string;
}

// MovePreview は、移動の確認ダイアログに出す件数(B5a-19)。below は下のもの(削除済みを除く)、statuses は
// 移動先が使わないステータス(プロジェクトが変わらない移動なら空)。
export interface MovePreview {
  below: number;
  statuses: MovePreviewStatus[];
}

// MoveTarget は、移動の移し先。Epic は project、ほかは parent のどちらか一方。
export type MoveTarget = { parent: string } | { project: string };

// ItemRef は、一括削除の対象 1 件(内部 ID は Store の中でだけ一意なので Store と組で持つ)。
export interface ItemRef {
  storeId: string;
  itemId: string;
}

// BulkDeletePreview は、一括削除の確認ダイアログの中身。stores は対象の Store の数、retentionDays は保持日数
// (Store が 2 つ以上のときは文に出さない)、readOnly は閲覧のみの Store の件数(削除しない)。
export interface BulkDeletePreview {
  stores: number;
  retentionDays: number;
  readOnly: number;
}

// BulkDeleted は、一括削除の結果。deleted は削除した件数、readOnly は閲覧のみの Store のため残した件数。
export interface BulkDeleted {
  deleted: number;
  readOnly: number;
}
