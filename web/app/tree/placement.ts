import type { TaskView } from "../../core/api/client";
import { canPlace, defaultKindUnder, kindLabel, parentRule } from "../../core/kinds";

// The body of POST /tasks/{id}/move. before and after name a sibling under the
// parent the task ends up in; parent and root set that parent, and leaving
// both out keeps the current one.
export type MoveArgs = { parent?: string; root?: boolean; before?: string; after?: string };

// Why a placement is refused: a short title and the full reason.
export type Rejection = { title: string; reason: string };

// checkParent reports why child cannot sit under parent (null for the root),
// or null when it can. Whether parent is a descendant of child is the
// caller's to check.
export function checkParent(
  child: TaskView,
  parent: TaskView | null,
  idOf: (t: TaskView) => string = (t) => t.id,
): Rejection | null {
  if (parent && parent.store !== child.store) {
    return {
      title: "別の Store のタスクの下には置けません",
      reason: `「${idOf(child)} ${child.title}」は ${child.store}、「${idOf(parent)} ${parent.title}」は ${parent.store} にあります。Store をまたぐ移動はドラッグではできません。`,
    };
  }
  if (canPlace(child.kind, parent ? parent.kind : null)) return null;
  const mine = kindLabel(child.kind);
  return {
    title: parent
      ? `${mine} は ${kindLabel(parent.kind)} の下に置けません`
      : `${mine} は最上位に置けません`,
    reason: `「${idOf(child)} ${child.title}」は ${mine} です。${parentRule(child.kind)}`,
  };
}

// pickKind is the kind a new task gets under a parent of parentKind (null for the
// top): the default kind of the parent, or null when nothing can sit under it.
export function pickKind(parentKind: string | null): string | null {
  const kind = defaultKindUnder(parentKind);
  return canPlace(kind, parentKind) ? kind : null;
}
