import type { MovedFrom } from "../../model/transfer";
import type { Detail } from "../items/detail";
import type { MovedNotice } from "./movedNotice";

// 旧 ID から開いたアイテムの詳細パネルを開き、上部の 1 行(A5a-16)を出す。
// 開けなかったとき(パネルがそのアイテムを開こうとしていない)と、開くのに失敗したときは 1 行を捨てる(あとで同じ
// アイテムを普通に開いても出さない)。開く途中を画面への反映の読み直しが追い越したときは、パネルはまだ出ていないが
// 同じアイテムを開こうとしたままなので、捨てない(読み直しが出す)。失敗は投げない。

export interface OpenMovedDeps {
  detail: Pick<Detail, "open" | "target">;
  notice: Pick<MovedNotice, "show" | "clear">;
}

export function openMoved(
  deps: OpenMovedDeps,
  storeId: string,
  itemId: string,
  moved: MovedFrom,
): Promise<void> {
  deps.notice.show(storeId, itemId, moved);
  return deps.detail.open(storeId, itemId).then(
    () => {
      const t = deps.detail.target();
      if (t?.storeId !== storeId || t.itemId !== itemId) deps.notice.clear();
    },
    () => {
      deps.notice.clear();
    },
  );
}
