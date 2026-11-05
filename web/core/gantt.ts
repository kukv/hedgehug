// The gantt as numbers: which rows it has, where their bars go, and what is wrong with them. A day
// is a whole number of days since 1970-01-01, so a day has no time zone and no daylight saving.
import type { TaskView } from "./api/client";
import type { TreeNode } from "./cache";
import type { Category } from "./categories";
import type { GanttSettings } from "./views";

export type Span = { s: number; e: number };
export type Scale = "day" | "week" | "month";

const DAY_MS = 24 * 60 * 60 * 1000;

/** The day of a date of the files (2026-11-03); null for anything else. */
export function dayOf(date: unknown): number | null {
  if (typeof date !== "string") return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  if (!m) return null;
  const t = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  return Math.floor(t / DAY_MS);
}

export function dateOf(day: number): string {
  return new Date(day * DAY_MS).toISOString().slice(0, 10);
}

/** Today where the person is. */
export const todayOf = (now: Date): number =>
  Math.floor(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate()) / DAY_MS);

/** 11/03 */
export const shortDate = (day: number): string => dateOf(day).slice(5).replace("-", "/");

/** The span of a start and an end: one day when only one is there; an end before the start is the start. */
export function spanOf(start: unknown, end: unknown): Span | null {
  const s = dayOf(start);
  const e = dayOf(end);
  if (s !== null && e !== null) return { s, e: Math.max(s, e) };
  if (s !== null) return { s, e: s };
  return e !== null ? { s: e, e } : null;
}

const union = (a: Span | null, b: Span | null): Span | null => {
  if (!a || !b) return a ?? b;
  return { s: Math.min(a.s, b.s), e: Math.max(a.e, b.e) };
};

const inside = (inner: Span, outer: Span): boolean => inner.s >= outer.s && inner.e <= outer.e;

// work: the task's own dates. sprint: it has none, so the period of its sprint, dashed. rollup: the
// earliest to the latest of its children.
export type BarKind = "work" | "sprint" | "rollup";

export type GanttRow = {
  ulid: string;
  task: TaskView;
  depth: number;
  hasChildren: boolean;
  expanded: boolean;
  category: Category;
  bar: Span | null;
  kind: BarKind;
  own: Span | null;
  // A child runs out of the dates set on this task.
  over: boolean;
};

export type RowsInput = {
  roots: TreeNode[];
  taskOf: (ulid: string) => TaskView | undefined;
  settings: GanttSettings;
  collapsed: ReadonlySet<string>;
  isContainer: (t: TaskView) => boolean;
  categoryOf: (t: TaskView) => Category;
  sprintSpan: (t: TaskView) => Span | null;
};

/**
 * The rows, in the order of the tree. A parent's bar is counted from all that is under it, whether
 * or not that is shown: a collapsed row and a roadmap add up the same.
 */
export function ganttRows(i: RowsInput): GanttRow[] {
  const out: GanttRow[] = [];
  const spans = new Map<string, Span | null>();
  const ownOf = (t: TaskView) => spanOf(t.props[i.settings.start], t.props[i.settings.end]);
  const rollup = (n: TreeNode) =>
    n.children.reduce<Span | null>((a, c) => union(a, spanUnder(c)), null);

  function spanUnder(n: TreeNode): Span | null {
    if (spans.has(n.ulid)) return spans.get(n.ulid) ?? null;
    const t = i.taskOf(n.ulid);
    const v = rollup(n) ?? (t ? (ownOf(t) ?? i.sprintSpan(t)) : null);
    spans.set(n.ulid, v);
    return v;
  }

  const walk = (nodes: TreeNode[], depth: number) => {
    for (const n of nodes) {
      const task = i.taskOf(n.ulid);
      if (!task || (i.settings.roadmap && !i.isContainer(task))) continue;
      const shown = i.settings.roadmap
        ? n.children.filter((c) => {
            const t = i.taskOf(c.ulid);
            return t !== undefined && i.isContainer(t);
          })
        : n.children;
      const expanded = shown.length > 0 && !i.collapsed.has(n.ulid);
      const own = ownOf(task);
      const rolled = rollup(n);
      out.push({
        ulid: n.ulid,
        task,
        depth,
        hasChildren: shown.length > 0,
        expanded,
        category: i.categoryOf(task),
        bar: spanUnder(n),
        kind: rolled ? "rollup" : own ? "work" : "sprint",
        own,
        over: rolled !== null && own !== null && !inside(rolled, own),
      });
      if (expanded) walk(shown, depth + 1);
    }
  };
  walk(i.roots, 0);
  return out;
}

export type GanttLink = { from: GanttRow; to: GanttRow; late: boolean };

/** The arrows of blocked_by between rows that have a bar. late: the blocker ends after the other starts. */
export function ganttLinks(rows: readonly GanttRow[]): GanttLink[] {
  const by = new Map(rows.map((r) => [r.ulid, r]));
  const out: GanttLink[] = [];
  for (const to of rows) {
    if (!to.bar) continue;
    for (const ref of to.task.blocked_by ?? []) {
      const from = by.get(ref.ulid);
      if (from?.bar) out.push({ from, to, late: from.bar.e > to.bar.s });
    }
  }
  return out;
}

export type GanttWarning = { key: string; id: string; text: string };

/** What the colors of the chart say, in words. */
export function ganttWarnings(
  rows: readonly GanttRow[],
  links: readonly GanttLink[],
): GanttWarning[] {
  const out: GanttWarning[] = [];
  for (const l of links) {
    if (!l.late || !l.from.bar || !l.to.bar) continue;
    const a = l.from.task.id;
    const b = l.to.task.id;
    out.push({
      key: `${l.from.ulid}-${l.to.ulid}`,
      id: `${a} → ${b}`,
      text: `ブロックしている ${a} の終了（${shortDate(l.from.bar.e)}）が、${b} の開始（${shortDate(l.to.bar.s)}）より後です。`,
    });
  }
  for (const r of rows) {
    if (!r.over || !r.own || !r.bar) continue;
    out.push({
      key: r.ulid,
      id: r.task.id,
      text: `子の期間（${shortDate(r.bar.s)} – ${shortDate(r.bar.e)}）が、設定した期間（${shortDate(r.own.s)} – ${shortDate(r.own.e)}）からはみ出しています。`,
    });
  }
  return out;
}

export const PX_PER_DAY: Record<Scale, number> = { day: 38, week: 14, month: 4 };

const PAD_DAYS: Record<Scale, number> = { day: 3, week: 7, month: 30 };

/** The days the chart shows: all the bars, the marks given, and today, with some room around them. */
export function chartRange(
  rows: readonly GanttRow[],
  marks: readonly number[],
  today: number,
  scale: Scale,
): Span {
  const days = [today, ...marks];
  for (const r of rows) if (r.bar) days.push(r.bar.s, r.bar.e);
  const pad = PAD_DAYS[scale];
  return { s: Math.min(...days) - pad, e: Math.max(...days) + pad };
}

export type Tick = { day: number; label: string; weekend: boolean };

/** The marks of the axis: every day, every Monday, or the first of every month. */
export function ticks(range: Span, scale: Scale): Tick[] {
  const out: Tick[] = [];
  for (let day = range.s; day <= range.e; day++) {
    const d = new Date(day * DAY_MS);
    const dow = d.getUTCDay();
    const dom = d.getUTCDate();
    const month = d.getUTCMonth() + 1;
    if (scale === "day") {
      const label = dom === 1 ? `${month}/1` : String(dom);
      out.push({ day, label, weekend: dow === 0 || dow === 6 });
    } else if (scale === "week" && dow === 1) {
      out.push({ day, label: `${month}/${dom}`, weekend: false });
    } else if (scale === "month" && dom === 1) {
      out.push({ day, label: `${d.getUTCFullYear()}/${month}`, weekend: false });
    }
  }
  return out;
}

export type DragMode = "move" | "start" | "end";

/** The span after a drag by delta days. An edge stops at the other edge. */
export function dragged(bar: Span, mode: DragMode, delta: number): Span {
  if (mode === "move") return { s: bar.s + delta, e: bar.e + delta };
  if (mode === "start") return { s: Math.min(bar.s + delta, bar.e), e: bar.e };
  return { s: bar.s, e: Math.max(bar.e + delta, bar.s) };
}

/** The properties a drop changes: what differs from the task, in one PATCH. */
export function dropProps(
  row: Pick<GanttRow, "task">,
  to: Span,
  settings: GanttSettings,
): Record<string, string> {
  const props: Record<string, string> = {};
  const put = (name: string, day: number) => {
    const v = dateOf(day);
    if (row.task.props[name] !== v) props[name] = v;
  };
  put(settings.start, to.s);
  put(settings.end, to.e);
  return props;
}
