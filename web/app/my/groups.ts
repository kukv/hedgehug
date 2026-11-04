import type { TaskView } from "../../core/api/client";
import { propText } from "../tree/marks";

// What the first screen of a person asks for: the tasks given to them that are not finished.
// The filter field may change it; the count of the sidebar always asks this.
export const MY_QUERY = "assignee:@me -status:done -status:canceled";

export type MyGroupBy = "due" | "project" | "workspace";
export type DueBucket = "overdue" | "today" | "week" | "later" | "none";
export type MyGroup = {
  key: string;
  name: string;
  // alert: the due dates that need the eye (overdue, today)
  tone: "alert" | "plain";
  // store: the Store of a workspace group (for its badge)
  store?: string;
  tasks: TaskView[];
};

export const NO_PROJECT = "プロジェクトなし";

const DAY = 86_400_000;
const DATE = /^(\d{4})-(\d{2})-(\d{2})/;

const BUCKETS: { key: DueBucket; name: string; tone: "alert" | "plain" }[] = [
  { key: "overdue", name: "期限切れ", tone: "alert" },
  { key: "today", name: "今日", tone: "alert" },
  { key: "week", name: "今週", tone: "plain" },
  { key: "later", name: "来週以降", tone: "plain" },
  { key: "none", name: "期限なし", tone: "plain" },
];

// The number of a calendar day: the same for every hour of it, and summer time does not move it.
function dayNumber(y: number, m: number, d: number): number {
  return Math.floor(Date.UTC(y, m - 1, d) / DAY);
}

/** The day a task is due as a day number, or null when it has no due date. */
export function dueDay(t: TaskView): number | null {
  const s = propText(t, "due");
  const m = s ? DATE.exec(s) : null;
  return m ? dayNumber(Number(m[1]), Number(m[2]), Number(m[3])) : null;
}

/** Which part of the time a due day is in, by the date of the browser. The week begins on Monday. */
export function bucketOf(day: number | null, today: Date): DueBucket {
  if (day === null) return "none";
  const now = dayNumber(today.getFullYear(), today.getMonth() + 1, today.getDate());
  if (day < now) return "overdue";
  if (day === now) return "today";
  // Sunday is the last day of the week (getDay() is 0), so only the days up to it are "this week".
  const sunday = now + ((7 - today.getDay()) % 7);
  return day <= sunday ? "week" : "later";
}

// By due date, soonest first; a task with none goes last. The sort is stable.
function byDue(a: TaskView, b: TaskView): number {
  const x = dueDay(a);
  const y = dueDay(b);
  if (x === y) return 0;
  if (x === null) return 1;
  if (y === null) return -1;
  return x - y;
}

/**
 * The tasks in groups: by due date (overdue, today, this week, later, none), by project, or by
 * workspace. order is the keys ("store/workspace ulid") of the workspaces in the order of the
 * sidebar; the ones not in it come last, by name.
 */
export function groupMyTasks(
  tasks: TaskView[],
  by: MyGroupBy,
  today: Date,
  order: readonly string[] = [],
): MyGroup[] {
  const sorted = [...tasks].sort(byDue);
  if (by === "workspace") {
    const byWorkspace = new Map<string, MyGroup>();
    for (const t of sorted) {
      const key = `${t.store}/${t.workspace.ulid}`;
      let g = byWorkspace.get(key);
      if (!g) {
        g = { key, name: t.workspace.name, tone: "plain", store: t.store, tasks: [] };
        byWorkspace.set(key, g);
      }
      g.tasks.push(t);
    }
    const rank = (g: MyGroup) => {
      const i = order.indexOf(g.key);
      return i < 0 ? order.length : i;
    };
    return [...byWorkspace.values()].sort(
      (a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name, "ja"),
    );
  }
  if (by === "due") {
    return BUCKETS.map((b) => ({
      ...b,
      tasks: sorted.filter((t) => bucketOf(dueDay(t), today) === b.key),
    })).filter((g) => g.tasks.length > 0);
  }
  const byProject = new Map<string, MyGroup>();
  for (const t of sorted) {
    const key = t.project?.ulid ?? "";
    let g = byProject.get(key);
    if (!g) {
      g = { key, name: t.project?.title ?? NO_PROJECT, tone: "plain", tasks: [] };
      byProject.set(key, g);
    }
    g.tasks.push(t);
  }
  // Named projects by name; the tasks outside any project last.
  return [...byProject.values()].sort((a, b) =>
    a.key === "" ? 1 : b.key === "" ? -1 : a.name.localeCompare(b.name, "ja"),
  );
}

/** How many workspaces the tasks are in ("3 ワークスペース · 9 件"). */
export function workspaceCount(tasks: TaskView[]): number {
  return new Set(tasks.map((t) => `${t.store}/${t.workspace.ulid}`)).size;
}

/** The logins of the accounts of the stores ("host/login" is what the stores list), once each. */
export function loginsOf(stores: { account?: string }[]): string[] {
  const out: string[] = [];
  for (const s of stores) {
    if (!s.account) continue;
    const login = s.account.slice(s.account.lastIndexOf("/") + 1);
    if (!out.includes(login)) out.push(login);
  }
  return out;
}
