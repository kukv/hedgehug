import type { Item } from "../../model/item";
import type { MoveCandidate } from "../../model/itemAction";
import type { WorkspaceSettings } from "../../model/workspace";
import { categoryOf } from "./progress";

// ツリーの書き込みのキーが使う、純粋な決め方(04「ツリーのキー操作」)。

export type Dir = "up" | "down";

// reparentTarget は、Alt+Shift+↑↓ の新しい親。フォーカスのある行から、上(up)か下(down)へ見える行を順にたどり、
// 最初に見つかった「移動…」の候補(アイテムのもの。プロジェクトの候補は見える行にない)を返す。自分の子孫の行は飛ばす。
// candidates は API の候補(自分と子孫・いまの親・削除済みは、もともと入っていない)。見つからなければ null。
export function reparentTarget(
  rows: readonly Item[],
  id: string,
  dir: Dir,
  candidates: readonly MoveCandidate[],
): MoveCandidate | null {
  const at = rows.findIndex((r) => r.id === id);
  if (at < 0) return null;
  const byId = new Map(rows.map((r) => [r.id, r]));
  const inside = (r: Item): boolean => {
    for (let p = r.parentId && byId.get(r.parentId); p; p = p.parentId && byId.get(p.parentId)) {
      if (p.id === id) return true;
    }
    return false;
  };
  const items = new Map(candidates.filter((c) => c.kind === "item").map((c) => [c.id, c]));
  const walk = dir === "up" ? rows.slice(0, at).reverse() : rows.slice(at + 1);
  for (const r of walk) {
    const c = items.get(r.id);
    if (c && !inside(r)) return c;
  }
  return null;
}

// doneToggleTarget は、Space の行き先のステータス。分類が完了でなければ完了の既定のステータス、完了なら未着手の既定のステータス
// (03)。既定がなければ null(何もしない)。遷移の制約に反するときは、保存の側が断る。
export function doneToggleTarget(settings: WorkspaceSettings, status: string): string | null {
  const done = categoryOf(settings, status) === "done";
  return settings.categoryDefaults[done ? "todo" : "done"] ?? null;
}
