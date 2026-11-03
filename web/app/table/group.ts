import type { TaskView } from "../../core/api/client";
import { statusRank } from "../../core/status-order";
import type { TableSettings } from "../../core/views";
import type { SortCtx } from "./columns";

export type GroupBy = TableSettings["groupBy"];

// The height of a row and of the heading of a group, in px; the CSS uses --row-h and
// --control-h, which have the same sizes.
export const ROW_HEIGHT = 40;
export const GROUP_HEIGHT = 36;

// The heading of a group. id and ulid are those of the parent or the project the group is of;
// none is the group of the tasks that have no parent, assignee or project.
export type GroupHead = {
  type: "group";
  key: string;
  label: string;
  id?: string;
  ulid?: string;
  progress?: { done: number; total: number };
  count: number;
  none: boolean;
};
export type TableItem = GroupHead | { type: "row"; key: string; task: TaskView };

type Bucket = { head: Omit<GroupHead, "count">; tasks: TaskView[] };

const NONE_LABEL: Record<Exclude<GroupBy, "none" | "status">, string> = {
  parent: "親なし",
  assignee: "未割り当て",
  project: "プロジェクトなし",
};

function bucketsOf(t: TaskView, by: Exclude<GroupBy, "none">, ctx: SortCtx): Bucket["head"][] {
  switch (by) {
    case "parent": {
      if (!t.parent) return [{ type: "group", key: "", label: NONE_LABEL.parent, none: true }];
      const p = ctx.taskOf(t.parent);
      return [
        {
          type: "group",
          key: t.parent,
          label: p ? p.title : ctx.loading?.(t.parent) ? "読み込み中…" : "不明な親",
          id: p?.id,
          ulid: t.parent,
          progress: p?.progress && p.progress.total > 0 ? p.progress : undefined,
          none: false,
        },
      ];
    }
    case "status": {
      const label = ctx.schemaOf(t.store)?.statuses?.find((s) => s.name === t.status)?.label;
      return [{ type: "group", key: t.status, label: label || t.status, none: false }];
    }
    case "assignee": {
      const names = t.assignees ?? [];
      if (names.length === 0) {
        return [{ type: "group", key: "", label: NONE_LABEL.assignee, none: true }];
      }
      return names.map((n) => ({ type: "group", key: n, label: n, none: false }));
    }
    case "project": {
      if (!t.project) return [{ type: "group", key: "", label: NONE_LABEL.project, none: true }];
      return [
        {
          type: "group",
          key: t.project.ulid,
          label: t.project.title,
          id: t.project.id,
          ulid: t.project.ulid,
          none: false,
        },
      ];
    }
  }
}

// groupTasks lays the tasks out as the table draws them, flat: a heading, then its rows. The
// tasks come in the order the table sorts them. The groups of a parent or a project come in the
// order they first appear; those of a status in the order of sort:status; those of an assignee by
// name. The group of the tasks with none goes last. A task with several assignees is in the group
// of each, as group:assignee of the query has it.
export function groupTasks(tasks: TaskView[], by: GroupBy, ctx: SortCtx): TableItem[] {
  if (by === "none") return tasks.map((task) => ({ type: "row", key: task.ulid, task }));
  const buckets = new Map<string, Bucket>();
  for (const task of tasks) {
    for (const head of bucketsOf(task, by, ctx)) {
      const b = buckets.get(head.key) ?? { head, tasks: [] };
      b.tasks.push(task);
      buckets.set(head.key, b);
    }
  }
  const list = [...buckets.values()];
  if (by === "status") {
    const rank = (b: Bucket) =>
      Math.min(
        ...b.tasks.map((t) =>
          statusRank(ctx.schemaOf(t.store), t.status, ctx.storeOrder.indexOf(t.store)),
        ),
      );
    list.sort((a, b) => rank(a) - rank(b));
  } else if (by === "assignee") {
    list.sort((a, b) => a.head.label.localeCompare(b.head.label, "ja"));
  }
  const ordered = [...list.filter((b) => !b.head.none), ...list.filter((b) => b.head.none)];
  return ordered.flatMap((b) => [
    { ...b.head, count: b.tasks.length },
    ...b.tasks.map((task) => ({
      type: "row" as const,
      key: `${b.head.key}\u0000${task.ulid}`,
      task,
    })),
  ]);
}

// windowWithHeld is the indexes the virtualizer draws: its window with the overscan, and the rows
// that are held (the one being edited), so that a row keeps its element, and its focus, when it is
// scrolled out of the window.
export function windowWithHeld(
  range: { startIndex: number; endIndex: number; overscan: number; count: number },
  held: number[],
): number[] {
  const out = new Set(held.filter((i) => i >= 0 && i < range.count));
  const from = Math.max(range.startIndex - range.overscan, 0);
  const to = Math.min(range.endIndex + range.overscan, range.count - 1);
  for (let i = from; i <= to; i++) out.add(i);
  return [...out].sort((a, b) => a - b);
}
