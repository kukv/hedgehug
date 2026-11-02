import type { TaskView } from "../../core/api/client";

type Problem = { code: string; field: string; message: string };

const LABELS: Record<string, string> = {
  unknown_kind: "未定義の種類です",
  unknown_status: "未定義のステータスです",
  missing_title: "タイトルがありません",
  placement: "置き場所が合いません",
  transition: "ステータスの遷移が合いません",
  missing_ref: "参照先がありません",
  cycle: "循環しています",
  no_id: "ID がありません",
  duplicate_id: "ID が重複しています",
  id_prefix: "ID の接頭辞が違います",
  parent_deleted: "親が削除されています",
};

// problemLabel is the short Japanese a row shows for what is wrong with a task.
export function problemLabel(p: Problem): string {
  if (p.code === "required") return `${p.field} 未入力`;
  if (p.code === "bad_value") return `${p.field} の値が不正です`;
  return LABELS[p.code] ?? p.code;
}

// propText is a property of the task when it holds a string.
export function propText(t: TaskView, name: string): string | undefined {
  const v = (t.props as Record<string, unknown> | undefined)?.[name];
  return typeof v === "string" ? v : undefined;
}

// openBlocker is the ID of the first blocker that still blocks the task: one
// whose state is in the cache and not done or canceled, or, when the blocker is
// not in the cache, the first one if the server says the task is blocked.
export function openBlocker(
  t: TaskView,
  taskOf: (ulid: string) => TaskView | undefined,
  isClosed: (blocker: TaskView) => boolean,
  serverBlocked: boolean,
): string | undefined {
  for (const ref of t.blocked_by ?? []) {
    const b = taskOf(ref.ulid);
    if (b ? !b.deleted_at && !isClosed(b) : serverBlocked) return ref.id;
  }
  return undefined;
}
