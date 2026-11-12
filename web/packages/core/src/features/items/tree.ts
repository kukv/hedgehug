import type { Item } from "../../model/item";

// アイテムの親子(04「ツリー」の「行」)。兄弟は、並び順の値 → 作成の時刻 → 内部 ID の順(A2 の計画の決定 12)。

export interface TreeNode {
  item: Item;
  children: TreeNode[];
}

export function compareSiblings(a: Item, b: Item): number {
  if (a.rank !== b.rank) return a.rank < b.rank ? -1 : 1;
  const at = Date.parse(a.createdAt) - Date.parse(b.createdAt);
  if (at !== 0) return at;
  return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}

// buildTree は、プロジェクトのアイテムを木にする。根は親のないもの(Epic)。親が一覧にないアイテムは出さない。
export function buildTree(items: Item[]): TreeNode[] {
  const byParent = new Map<string | null, Item[]>();
  for (const it of items) {
    const list = byParent.get(it.parentId) ?? [];
    list.push(it);
    byParent.set(it.parentId, list);
  }
  const grow = (parentId: string | null, seen: Set<string>): TreeNode[] =>
    [...(byParent.get(parentId) ?? [])]
      .filter((it) => !seen.has(it.id))
      .sort(compareSiblings)
      .map((it) => ({ item: it, children: grow(it.id, new Set(seen).add(it.id)) }));
  return grow(null, new Set());
}

// ancestorsOf は、id のアイテムの祖先を、根から順に返す(自分は入れない)。
export function ancestorsOf(items: Item[], id: string): Item[] {
  const byId = new Map(items.map((it) => [it.id, it]));
  const out: Item[] = [];
  const seen = new Set([id]);
  let parentId = byId.get(id)?.parentId ?? null;
  while (parentId !== null && !seen.has(parentId)) {
    const parent = byId.get(parentId);
    if (!parent) break;
    out.unshift(parent);
    seen.add(parentId);
    parentId = parent.parentId;
  }
  return out;
}
