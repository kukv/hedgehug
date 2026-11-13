import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { ItemActionsPort } from "../../port/itemActions";
import type { Catalog } from "../catalog/catalog";
import type { Toasts } from "../toasts/toasts";
import type { Detail } from "./detail";
import type { ItemRefresh } from "./refresh";

// 「複製」(02「複製」)。同じ親の下に、もう 1 つ作る。終わると、詳細パネルは新しいアイテムに切り替わる。
// トーストは出さない(断りと失敗のときだけ出す)。

// 断りでも「ない」でもない失敗(通信の失敗など)のときの文言。
export const duplicateFailedMessage = "複製できませんでした";

export interface ItemDuplicateDeps {
  actions: Pick<ItemActionsPort, "duplicate">;
  catalog: Catalog;
  detail: Pick<Detail, "open">;
  toasts: Toasts;
  refresh: Pick<ItemRefresh, "afterChange">;
}

export interface ItemDuplicate {
  // run は、item を複製して、詳細パネルを新しいアイテムに切り替える。閲覧のみの Store では何もしない。
  run(item: Item): Promise<void>;
}

export function createItemDuplicate(deps: ItemDuplicateDeps): ItemDuplicate {
  // busy は、複製の実行中。そのあいだの run は受けない(メニューの 2 度押しで 2 つ作らない)。
  let busy = false;

  return {
    async run(item) {
      if (busy || deps.catalog.store(item.storeId)?.readOnly) return;
      busy = true;
      let created: string;
      try {
        created = await deps.actions.duplicate(item.storeId, item.id);
      } catch (e) {
        // すでに消えているのは、画面を読み直して消す(トーストなし)。
        if (e instanceof NotFound) await deps.refresh.afterChange();
        else
          deps.toasts.show(e instanceof Refused ? e.message : duplicateFailedMessage, {
            error: true,
          });
        return;
      } finally {
        busy = false;
      }
      // 書きかけの確認で「キャンセル」されたときは、パネルは元のアイテムのまま(複製はできている)。
      await deps.detail.open(item.storeId, created);
      await deps.refresh.afterChange();
    },
  };
}
