import type { TaskView } from "../../core/api/client";
import type { TreeNode } from "../../core/cache";

// orderRootsByStore puts the roots of a tree read over every store in the order of the stores
// (the order of GET /api/stores), keeping the order they came in within a store. A root whose task is
// not in the cache, or of a store that is not listed, goes last. It reads the cache at the moment it
// is called and holds nothing, so it needs no care about when the tree and the tasks arrive: it is
// called again with every change of either. It returns roots itself when nothing moves.
export function orderRootsByStore(
  roots: TreeNode[],
  storeOrder: readonly string[],
  taskOf: (ulid: string) => TaskView | undefined,
): TreeNode[] {
  const rank = (n: TreeNode): number => {
    const store = taskOf(n.ulid)?.store;
    const i = store === undefined ? -1 : storeOrder.indexOf(store);
    return i < 0 ? storeOrder.length : i;
  };
  const sorted = roots
    .map((node, at) => ({ node, at, rank: rank(node) }))
    .sort((a, b) => a.rank - b.rank || a.at - b.at)
    .map((x) => x.node);
  return sorted.every((n, i) => n === roots[i]) ? roots : sorted;
}

// storesOf lists the stores the roots belong to, in the order of the roots. A child is always in
// the store of its parent, so the roots hold every store of the tree.
export function storesOf(
  roots: TreeNode[],
  taskOf: (ulid: string) => TaskView | undefined,
): string[] {
  const out: string[] = [];
  for (const n of roots) {
    const store = taskOf(n.ulid)?.store;
    if (store !== undefined && !out.includes(store)) out.push(store);
  }
  return out;
}
