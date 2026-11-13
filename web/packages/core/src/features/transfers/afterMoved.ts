import type { Item } from "../../model/item";
import type { TransferResult } from "../../model/transfer";
import type { Detail } from "../items/detail";
import type { ItemRefresh } from "../items/refresh";

// 自分で移したあとの詳細パネルの切り替えと読み直し(A5a-18)。panelItem は「移動」を押したときにパネルが出していた
// アイテム(移したもの自身か、その下のもの)の内部 ID。別の Store へ移しても内部 ID は変わらないので、移動先の Store で
// 同じ内部 ID を開き直す。送っているあいだの読み直しでパネルが閉じていても開き直す(反映の届く順に左右されない)。
// 先に確認なしで閉じるのは、別のアイテムを開くときの書きかけの確認を出さないため(書きかけはダイアログを開く前に聞いた)。

export interface AfterMovedDeps {
  detail: Pick<Detail, "close" | "open">;
  refresh: Pick<ItemRefresh, "afterChange">;
}

export function createAfterMoved(
  deps: AfterMovedDeps,
): (from: Item, to: TransferResult, panelItem: string | undefined) => Promise<void> {
  return async (_from, to, panelItem) => {
    if (panelItem !== undefined) {
      deps.detail.close();
      await deps.detail.open(to.toStore, panelItem);
    }
    await deps.refresh.afterChange();
  };
}
