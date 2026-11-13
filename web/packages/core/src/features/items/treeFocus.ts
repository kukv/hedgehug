import type { Item } from "../../model/item";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { visibleRows } from "./siblings";
import type { TreeNode } from "./tree";

// ツリーのフォーカスと開閉(04「ツリーのキー操作」、A4b-20)。見えている行の並び・いまの行・閉じている行を持つ。
// フォーカスは保存しない(画面を出しているあいだだけ)。ツリーのアイテムの行だけが対象(プロジェクトの行は E)。

export interface TreeFocusState {
  // rows は、見えている行を上から順に並べたもの(閉じている行の下は入れない)。
  rows: readonly Item[];
  // collapsed は、閉じている行の ID。初めはすべて開いている。
  collapsed: ReadonlySet<string>;
  // currentId は、フォーカスのある行。行がなければ、またはツリーに入っていなければ null。
  currentId: string | null;
}

export type TreeMove = "up" | "down" | "right" | "left";

export interface TreeFocus {
  state: Readable<TreeFocusState>;
  // setTree は、ツリーが変わったとき(読み直し・削除・移動)に呼ぶ。フォーカスのある行が見えなくなったら、
  // 次の見えている行へ、なければ前の見えている行へ移す。
  setTree(tree: readonly TreeNode[]): void;
  // enter は、ツリーに入ったとき(Tab)に呼ぶ。先頭の行にフォーカスを付ける。keep なら、いまの行が見えているかぎり動かさない
  // (キーで開いたメニューを閉じて戻ったとき、行を押して入ったとき)。
  enter(keep?: boolean): void;
  // focus は、行を指定してフォーカスを付ける(行を押したとき)。見えていない行は無視する。
  focus(id: string): void;
  // move は、↑↓←→。端で止まる(回り込まない)。
  move(dir: TreeMove): void;
  // expand は、閉じている行を開く。開いている行には何もしない。
  expand(id: string): void;
  // toggle は、開いていれば閉じ、閉じていれば開く。閉じた行の下にフォーカスがあったら、その行へ移す。
  toggle(id: string): void;
}

// settle は、行の並びが変わったあとの、フォーカスのある行。見えなくなっていたら、前の並びで次の行、なければ前の行。
function settle(rows: readonly Item[], before: readonly Item[], id: string | null): string | null {
  if (id === null) return null;
  const visible = new Set(rows.map((r) => r.id));
  if (visible.has(id)) return id;
  const at = before.findIndex((r) => r.id === id);
  if (at < 0) return null;
  const after = before.slice(at + 1).find((r) => visible.has(r.id));
  const earlier = before
    .slice(0, at)
    .reverse()
    .find((r) => visible.has(r.id));
  return (after ?? earlier)?.id ?? null;
}

export function createTreeFocus(): TreeFocus {
  let tree: readonly TreeNode[] = [];
  // parentOf は、ツリーの全アイテムの親(閉じている行の下も)。hasKids は、子のある行。
  let parentOf = new Map<string, string | null>();
  let hasKids = new Set<string>();
  const state = writable<TreeFocusState>({ rows: [], collapsed: new Set(), currentId: null });

  const current = () => get(state);
  const under = (id: string, ancestor: string): boolean => {
    for (let p = parentOf.get(id); p; p = parentOf.get(p)) if (p === ancestor) return true;
    return false;
  };

  function commit(collapsed: ReadonlySet<string>, wanted: string | null) {
    const before = current();
    const rows = visibleRows(tree, collapsed);
    state.set({ rows, collapsed, currentId: settle(rows, before.rows, wanted) });
  }

  function setCollapsed(id: string, closed: boolean) {
    const s = current();
    if (s.collapsed.has(id) === closed) return;
    const next = new Set(s.collapsed);
    if (closed) next.add(id);
    else next.delete(id);
    // 閉じた行の下にフォーカスがあったら、その行へ移す(仕様にない。A4b の決めごと)。
    const inside = closed && s.currentId !== null && under(s.currentId, id);
    commit(next, inside ? id : s.currentId);
  }

  function index(nodes: readonly TreeNode[]) {
    for (const n of nodes) {
      parentOf.set(n.item.id, n.item.parentId);
      if (n.children.length > 0) hasKids.add(n.item.id);
      index(n.children);
    }
  }

  function move(dir: TreeMove) {
    const s = current();
    const at = s.rows.findIndex((r) => r.id === s.currentId);
    const row = s.rows[at];
    if (!row) return;
    const go = (to: Item | undefined) => {
      if (to) state.set({ ...s, currentId: to.id });
    };
    const open = hasKids.has(row.id) && !s.collapsed.has(row.id);
    if (dir === "up") go(s.rows[at - 1]);
    else if (dir === "down") go(s.rows[at + 1]);
    else if (dir === "right") {
      // 閉じていれば開き、開いていれば最初の子へ。子のない行では何もしない。
      if (!hasKids.has(row.id)) return;
      if (open) go(s.rows[at + 1]);
      else setCollapsed(row.id, false);
    }
    // ←: 開いていれば閉じ、閉じている(または子のない)行では親へ。一番上の階層では何もしない。
    else if (open) setCollapsed(row.id, true);
    else go(s.rows.find((r) => r.id === row.parentId));
  }

  return {
    state: readonly(state),
    setTree(next) {
      tree = next;
      parentOf = new Map();
      hasKids = new Set();
      index(next);
      const s = current();
      commit(s.collapsed, s.currentId);
    },
    enter(keep = false) {
      const s = current();
      if (keep && s.currentId !== null && s.rows.some((r) => r.id === s.currentId)) return;
      state.set({ ...s, currentId: s.rows[0]?.id ?? null });
    },
    focus(id) {
      const s = current();
      if (s.rows.some((r) => r.id === id)) state.set({ ...s, currentId: id });
    },
    move,
    expand: (id) => setCollapsed(id, false),
    toggle: (id) => setCollapsed(id, !current().collapsed.has(id)),
  };
}
