// The sprints and the releases as the screens show them. The server sends no text.
import type { Schemas, TaskView } from "./api/client";
import type { WorkspaceEntry } from "./stores";

export type Sprint = Schemas["SprintView"];
export type Release = Schemas["ReleaseView"];
export type Template = Schemas["TemplateView"];
export type ReleaseState = "unreleased" | "released" | "archived";

export const RELEASE_STATES: readonly ReleaseState[] = ["unreleased", "released", "archived"];

// The tabs of the plan show on a workspace that has a sprint or a release (the spec's 6d).
export const hasPlans = (w: Pick<WorkspaceEntry, "sprints" | "releases">): boolean =>
  w.sprints + w.releases > 0;

const onDay = (date: string): number => {
  const [y = 0, m = 1, d = 1] = date.split("-").map(Number);
  return new Date(y, m - 1, d).getTime();
};

const today = (now: Date): number =>
  new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();

const DAY = 24 * 60 * 60 * 1000;

// daysLeft counts the days to end, end included: the last day of a sprint has 1 left. Past the end
// it is 0.
export function daysLeft(end: string, now: Date): number {
  return Math.max(0, Math.round((onDay(end) - today(now)) / DAY) + 1);
}

const short = (date: string): string => date.slice(5).replace("-", "/");

/** 11/02 – 11/13, as much of it as the sprint has. */
export function sprintDates(s: Pick<Sprint, "start" | "end">): string {
  if (!s.start && !s.end) return "日付なし";
  return `${s.start ? short(s.start) : "—"} – ${s.end ? short(s.end) : "—"}`;
}

/** 予定 2026-11-12（あと 8 日）, or what the date is past. */
export function releaseDue(date: string | undefined, now: Date): string {
  if (!date) return "予定日なし";
  const n = Math.round((onDay(date) - today(now)) / DAY);
  if (n > 0) return `予定 ${date}（あと ${n} 日）`;
  return n === 0 ? `予定 ${date}（今日）` : `予定 ${date}（${-n} 日前）`;
}

// byStart orders planned sprints as the server picks next: the earliest start, then the ULID. One
// without a start comes last.
function byStart(a: Sprint, b: Sprint): number {
  const x = a.start || "9999-99-99";
  const y = b.start || "9999-99-99";
  return x < y ? -1 : x > y ? 1 : a.ulid < b.ulid ? -1 : a.ulid > b.ulid ? 1 : 0;
}

/** The sprint --to next goes to: the first planned one. */
export const nextSprint = (sprints: readonly Sprint[]): Sprint | undefined =>
  sprints.filter((s) => s.state === "planned").sort(byStart)[0];

const STATE_ORDER: Readonly<Record<string, number>> = { active: 0, planned: 1, closed: 2 };

/** The sprints as a list shows them: active, planned, then closed, the last one closed first. */
export function sprintsInOrder(sprints: readonly Sprint[]): Sprint[] {
  return [...sprints].sort((a, b) => {
    const s = (STATE_ORDER[a.state] ?? 3) - (STATE_ORDER[b.state] ?? 3);
    if (s !== 0) return s;
    return a.state === "closed" ? byStart(b, a) : byStart(a, b);
  });
}

const needsQuotes = /[\s"]/;

/** A value of a query: quoted when it holds a space. */
export const queryValue = (v: string): string =>
  needsQuotes.test(v) ? `"${v.replaceAll('"', '\\"')}"` : v;

export const BACKLOG_QUERY = "sprint:none status:todo,doing";

const byRank = (a: TaskView, b: TaskView): number =>
  a.rank < b.rank ? -1 : a.rank > b.rank ? 1 : a.ulid < b.ulid ? -1 : a.ulid > b.ulid ? 1 : 0;

export const inRankOrder = (tasks: readonly TaskView[]): TaskView[] => [...tasks].sort(byRank);

/** The points of a task; null when it has none. */
export function pointsOf(t: Pick<TaskView, "props">): number | null {
  const p = t.props.points;
  return typeof p === "number" ? p : null;
}

export const sumPoints = (tasks: readonly TaskView[]): number =>
  tasks.reduce((n, t) => n + (pointsOf(t) ?? 0), 0);

/** 12 pt; a fraction as the sum has it. */
export const ptLabel = (n: number): string => `${Math.round(n * 10) / 10} pt`;

// placed is order with ulid moved to the slot at (an index among the others).
export function placed(order: readonly string[], ulid: string, at: number): string[] {
  const rest = order.filter((u) => u !== ulid);
  rest.splice(Math.min(at, rest.length), 0, ulid);
  return rest;
}

export type CloseTo = { kind: "next" } | { kind: "backlog" } | { kind: "sprint"; name: string };

/** The value of to of the close: next, backlog or the name of a sprint. */
export const closeToValue = (to: CloseTo): string => (to.kind === "sprint" ? to.name : to.kind);

/** The words of the button of the close dialog. */
export function closeLabel(n: number, to: CloseTo, next: Sprint | undefined): string {
  if (n === 0) return "閉じる";
  if (to.kind === "backlog") return `閉じて ${n} 件をバックログへ`;
  const name = to.kind === "sprint" ? to.name : next?.name;
  return `閉じて ${n} 件を ${name} へ`;
}

/** The kinds in the order the schema has them, each with its tasks: the list of a release. */
export function groupByKind(
  tasks: readonly TaskView[],
  kinds: readonly string[],
): { kind: string; tasks: TaskView[] }[] {
  const rest = [...new Set(tasks.map((t) => t.kind))].filter((k) => !kinds.includes(k));
  return [...kinds, ...rest]
    .map((kind) => ({ kind, tasks: tasks.filter((t) => t.kind === kind) }))
    .filter((g) => g.tasks.length > 0);
}

/** 2h / 見積 6h, or the time alone for a task that has no estimate. */
export function worklogLabel(total: string, estimate: unknown): string {
  return typeof estimate === "string" && estimate !== "" ? `${total} / 見積 ${estimate}` : total;
}

/** The templates a project may use, from the names the schema lists; all of them without a limit. */
export function usableTemplates<T extends { name: string }>(
  all: readonly T[],
  only: readonly string[] | null | undefined,
): T[] {
  return only && only.length > 0 ? all.filter((t) => only.includes(t.name)) : [...all];
}
