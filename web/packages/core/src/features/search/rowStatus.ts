import { Refused } from "../../model/errors";
import type { ItemsPort } from "../../port/items";
import { sprintSaveFailed } from "../planning/sprintDialogs";
import type { Toasts } from "../toasts/toasts";

export interface RowStatusDeps {
  items: Pick<ItemsPort, "setProperty">;
  toasts: Pick<Toasts, "show">;
  // reload は、検索を読み直す。
  reload: () => Promise<void>;
}

// setRowStatus は、検索のテーブルで 1 行だけ選んでいるときの、パレットの「ステータスを変更」(E2a-18)。
// バックログと同じく、保存の断りはその文を、ほかの失敗は「保存できませんでした」をエラーのトーストに出し、
// どちらも読み直す(F2a-9)。
export async function setRowStatus(
  deps: RowStatusDeps,
  storeId: string,
  itemId: string,
  statusId: string,
): Promise<void> {
  try {
    await deps.items.setProperty(storeId, itemId, "status", statusId);
  } catch (e) {
    deps.toasts.show(e instanceof Refused ? e.message : sprintSaveFailed, { error: true });
  }
  await deps.reload();
}
