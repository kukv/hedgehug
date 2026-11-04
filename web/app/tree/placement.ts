import type { TaskView } from "../../core/api/client";
import { canParent, parentRule, type SchemaView } from "../../core/schema";

// The body of POST /tasks/{id}/move. before and after name a sibling under the
// parent the task ends up in; parent and root set that parent, and leaving
// both out keeps the current one.
export type MoveArgs = { parent?: string; root?: boolean; before?: string; after?: string };

// Why a placement is refused: a short title and the full reason.
export type Rejection = { title: string; reason: string };

export function kindLabel(s: SchemaView, kind: string): string {
  return s.kinds?.find((k) => k.name === kind)?.label || kind;
}

// checkParent reports why child cannot sit under parent (null for the root),
// or null when it can. Whether parent is a descendant of child is the
// caller's to check.
export function checkParent(
  s: SchemaView,
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
  if (canParent(s, child.kind, parent ? parent.kind : null)) return null;
  const mine = kindLabel(s, child.kind);
  return {
    title: parent
      ? `${mine}は${kindLabel(s, parent.kind)}の下に置けません`
      : `${mine}はルートに置けません`,
    reason: `「${idOf(child)} ${child.title}」は${mine}です。${parentRule(s, child.kind)}（${child.store} の schema.yaml の kinds）。`,
  };
}

// pickKind is the kind a new task gets under a parent of parentKind: the
// default kind when it fits, else the first work kind that does.
export function pickKind(s: SchemaView, parentKind: string | null): string | null {
  const kinds = s.kinds ?? [];
  const fits = (name: string) => canParent(s, name, parentKind);
  const k =
    kinds.find((x) => x.default && fits(x.name)) ??
    kinds.find((x) => x.role === "work" && fits(x.name));
  return k?.name ?? null;
}
