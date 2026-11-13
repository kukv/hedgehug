import type { Item } from "../../model/item";
import type { TreeNode } from "./tree";

// 見えている兄弟(02「上から順の構成」、04「ツリー」の「行」)。「上へ移動」「下へ移動」は、いま画面に見えている兄弟の
// 間で動かす。rows は、画面に見えている行を見えている順に並べたもの(ツリーなら、開いている行を上から順に。
// 子や孫の行が兄弟のあいだに混ざっていてよい)。絞り込みはないので、見えている兄弟 = すべての兄弟(Item には削除済みが来ない)。
// 返すのは、同じ親(Epic なら同じプロジェクトの Epic)の、見えている直前と直後の兄弟。id の行がなければ、どちらもない。
export function visibleNeighbors(rows: readonly Item[], id: string): { prev?: Item; next?: Item } {
  const at = rows.findIndex((r) => r.id === id);
  const me = rows[at];
  if (!me) return {};
  const sibling = (r: Item) => r.parentId === me.parentId && r.projectId === me.projectId;
  let prev: Item | undefined;
  for (let i = at - 1; i >= 0 && !prev; i--) {
    const r = rows[i];
    if (r && sibling(r)) prev = r;
  }
  const next = rows.slice(at + 1).find(sibling);
  return { ...(prev ? { prev } : {}), ...(next ? { next } : {}) };
}

// visibleRows は、ツリーの見えている行を、上から順に並べる(閉じている行の下は入れない)。collapsed は、閉じている行の ID。
export function visibleRows(tree: readonly TreeNode[], collapsed: ReadonlySet<string>): Item[] {
  const rows: Item[] = [];
  const walk = (nodes: readonly TreeNode[]) => {
    for (const n of nodes) {
      rows.push(n.item);
      if (!collapsed.has(n.item.id)) walk(n.children);
    }
  };
  walk(tree);
  return rows;
}
