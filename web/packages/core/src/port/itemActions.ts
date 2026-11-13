import type { Kind } from "../model/item";
import type {
  DeletePreview,
  MoveCandidate,
  MovePreview,
  MoveTarget,
  ProjectDeletePreview,
} from "../model/itemAction";

// アイテムとプロジェクトの操作(A4b)の口。実装は api パッケージにある。並べ替えは ItemsPort.reorder。
// 失敗は、断りなら Refused、ないものなら NotFound、それ以外はそのまま投げる。
// 変更の応答は使わない(`blocked` がいつも false のため)。呼んだあとは、ツリーと詳細パネルを読み直す。
export interface ItemActionsPort {
  // deletePreview は、アイテムの削除の確認ダイアログの中身。
  deletePreview(storeId: string, itemId: string): Promise<DeletePreview>;
  // deleteItem は、アイテムと下にあるものを、同じ 1 回の削除として消す。
  deleteItem(storeId: string, itemId: string): Promise<void>;
  projectDeletePreview(storeId: string, projectId: string): Promise<ProjectDeletePreview>;
  deleteProject(storeId: string, projectId: string): Promise<void>;
  // moveCandidates は、「移動…」の候補をすべて返す(絞るのは画面)。
  moveCandidates(storeId: string, itemId: string): Promise<MoveCandidate[]>;
  // movePreview は、移動の確認ダイアログに出す件数を読む(保存しない。B5a-19)。
  movePreview(storeId: string, itemId: string, to: MoveTarget): Promise<MovePreview>;
  // move の statuses は、プロジェクトが変わるときの、移動先が使わないステータス → 置き換え先。指定のないものは、
  // 同じ分類の移動先の既定になる。
  move(
    storeId: string,
    itemId: string,
    to: MoveTarget,
    statuses?: Record<string, string>,
  ): Promise<void>;
  changeKind(storeId: string, itemId: string, kind: Kind): Promise<void>;
  // duplicate は、新しいアイテムの内部 ID を返す。
  duplicate(storeId: string, itemId: string): Promise<string>;
}
