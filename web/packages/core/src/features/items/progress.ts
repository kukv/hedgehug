import type { Item } from "../../model/item";
import type { Category, WorkspaceSettings } from "../../model/workspace";
import { formatDay } from "../dates/date";

// 04「集計の定義」の「完了/対象」と、期限の表示。

export function categoryOf(settings: WorkspaceSettings, statusId: string): Category | undefined {
  return settings.statuses.find((s) => s.id === statusId)?.category;
}

// childCount は、子のうち完了の数と、中止を除いた数(詳細パネルの見出しの「3/8」)。
// 一覧にないステータスは未完了として数える(決定 11)。
export function childCount(children: Item[], settings: WorkspaceSettings) {
  const counted = children.filter((c) => categoryOf(settings, c.status) !== "canceled");
  const done = counted.filter((c) => categoryOf(settings, c.status) === "done").length;
  return { done, total: counted.length };
}

// stepProgress は、ツリーの Story・Task・Bug の行の「完了/対象」。対象が 0 件なら null。
export function stepProgress(items: Item[], parentId: string, settings: WorkspaceSettings) {
  const steps = items.filter((it) => it.parentId === parentId && it.kind === "Step");
  const c = childCount(steps, settings);
  return c.total === 0 ? null : c;
}

const pad = (n: number) => String(n).padStart(2, "0");

// dueLabel は、期限の表示。未完了で期限切れ・今日なら強調する(04「集計の定義」)。
export function dueLabel(end: string, category: Category | undefined, today: Date) {
  if (end === "") return null;
  const text = formatDay(end, today.getFullYear());
  const open = category !== "done" && category !== "canceled";
  const t = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`;
  if (open && end < t) return { text: `期限切れ ${text}`, emphasis: "overdue" as const };
  if (open && end === t) return { text: `今日 ${text}`, emphasis: "today" as const };
  return { text, emphasis: null };
}
