import type { Kind } from "../model/item";
import type {
  DeletePreview,
  MoveCandidate,
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
  move(storeId: string, itemId: string, to: MoveTarget): Promise<void>;
  changeKind(storeId: string, itemId: string, kind: Kind): Promise<void>;
  // duplicate は、新しいアイテムの内部 ID を返す。
  duplicate(storeId: string, itemId: string): Promise<string>;
}
