import type { Item } from "../../model/item";
import type { TransferResult } from "../../model/transfer";
import { get, type Readable } from "../../reactive/store";
import type { Detail } from "../items/detail";
import type { ItemRefresh } from "../items/refresh";

// 自分で移したあとの詳細パネルの切り替えと読み直し(A5a-18)。ダイアログは「移動」を押したときに
// afterMoved(from, panelItem) を呼び、成功したら返った関数に結果を渡す。panelItem は押したときにパネルが出していた
// アイテム(移したもの自身か、その下のもの)の内部 ID。別の Store へ移しても内部 ID は変わらないので、移動先の Store で
// 同じ内部 ID を開き直す。送っているあいだの読み直しでパネルが閉じていても開き直す(反映の届く順に左右されない)。
// ただし、送っているあいだに別のアイテムを開いた・画面を移ったときは、パネルに触らず読み直すだけにする
// (開いたものの書きかけを確認なしに捨てない)。
// 先に確認なしで閉じるのは、別の Store の同じ内部 ID を開くときに書きかけの確認を出さないため(移すアイテムの
// 書きかけはダイアログを開く前に聞いた)。

export interface AfterMovedDeps {
  detail: Pick<Detail, "view" | "close" | "open">;
  refresh: Pick<ItemRefresh, "afterChange">;
  // route は、画面のアドレス。変わった回数だけを見る。
  route: Readable<unknown>;
}

export type AfterMoved = (
  from: Item,
  panelItem: string | undefined,
) => (to: TransferResult) => Promise<void>;

export function createAfterMoved(deps: AfterMovedDeps): AfterMoved {
  let routes = 0;
  deps.route.subscribe(() => {
    routes += 1;
  });
  return (from, panelItem) => {
    const at = routes;
    return async (to) => {
      const shown = get(deps.detail.view)?.item;
      // パネルが閉じているか、panelItem(移動元か移動先の Store の同じ内部 ID)を出したままのときだけ開き直す。
      const still =
        shown === undefined ||
        (shown.id === panelItem &&
          (shown.storeId === from.storeId || shown.storeId === to.toStore));
      if (panelItem !== undefined && at === routes && still) {
        deps.detail.close();
        await deps.detail.open(to.toStore, panelItem);
      }
      await deps.refresh.afterChange();
    };
  };
}
