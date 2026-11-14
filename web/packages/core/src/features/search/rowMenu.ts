import type { Item } from "../../model/item";
import type { ItemMenuHostContext } from "../items/menuHost";
import { compareSiblings } from "../items/tree";

// rowMenuContext は、検索のテーブルの行末の「…」で開く、02 のアイテムのメニューの文脈(E2a-17)。items は、行のアイテムの
// プロジェクトのすべてのアイテム。rows は兄弟の並び(compareSiblings)で、「上へ移動」「下へ移動」は同じ親の兄弟の間で動く。
// アイテムが一覧にないときは null(メニューを開かない)。
export function rowMenuContext(
  items: Item[],
  itemId: string,
): { item: Item; ctx: ItemMenuHostContext } | null {
  const item = items.find((it) => it.id === itemId);
  if (!item) return null;
  const rows = [...items].sort(compareSiblings);
  const children = items.filter((it) => it.parentId === itemId);
  return { item, ctx: { rows, children, row: true } };
}
