// The reports as the screens draw them. The server sends the numbers of hh report --json; the axes,
// the stacks and the words are made here.
import type { Schemas } from "./api/client";
import { dayOf } from "./gantt";

export type BurndownView = Schemas["BurndownView"];
export type VelocityRow = Schemas["VelocityRow"];
export type CFDView = Schemas["CFDView"];
export type CycleView = Schemas["CycleView"];
export type EstimateView = Schemas["EstimateView"];

const sum = (ns: readonly number[]): number => ns.reduce((a, b) => a + b, 0);

const round = (n: number): number => Math.round(n * 10) / 10;

/** 12 pt or 3 件, as the report counts. */
export const amountLabel = (n: number, by: string): string =>
  by === "count" ? `${round(n)} 件` : `${round(n)} pt`;

export type BurndownPlot = {
  xs: number[]; // days
  remaining: (number | null)[];
  scope: (number | null)[];
  ideal: number[];
};

/**
 * The days of the sprint, from its first day to its last. The report stops at today, so the days to
 * come have no values, and neither have the days before the sprint held anything (it started later
 * than its date). The ideal line runs from the first scope to 0 on the last day.
 */
export function burndownPlot(v: BurndownView): BurndownPlot {
  const all = v.days ?? [];
  const held = all.findIndex((d) => d.scope > 0);
  const rows = held < 0 ? [] : all.slice(held);
  const known = new Map(rows.map((d) => [dayOf(d.date), d]));
  const days = all.map((d) => dayOf(d.date)).filter((d): d is number => d !== null);
  const first = Math.min(...days, dayOf(v.sprint.start) ?? Number.POSITIVE_INFINITY);
  const last = Math.max(...days, dayOf(v.sprint.end) ?? Number.NEGATIVE_INFINITY);
  if (!Number.isFinite(first) || !Number.isFinite(last)) {
    return { xs: [], remaining: [], scope: [], ideal: [] };
  }
  const xs = Array.from({ length: last - first + 1 }, (_, i) => first + i);
  const start = rows[0]?.scope ?? 0;
  return {
    xs,
    remaining: xs.map((x) => known.get(x)?.remaining ?? null),
    scope: xs.map((x) => known.get(x)?.scope ?? null),
    ideal: xs.map((_, i) => (xs.length > 1 ? round(start * (1 - i / (xs.length - 1))) : start)),
  };
}

export type BurndownFacts = { remaining: number; scope: number; added: number };

/** Where the sprint stands: the last day of the report, and what joined after the start. */
export function burndownFacts(v: BurndownView): BurndownFacts {
  const rows = v.days ?? [];
  const last = rows[rows.length - 1];
  return {
    remaining: last?.remaining ?? 0,
    scope: last?.scope ?? 0,
    added: sum(rows.map((d) => d.added)),
  };
}

export type VelocityPlot = {
  names: string[];
  points: number[];
  tasks: number[];
  // The mean of the last three; null with no sprint.
  average: number | null;
};

export function velocityPlot(rows: readonly VelocityRow[]): VelocityPlot {
  const points = rows.map((r) => r.points);
  const last = points.slice(-3);
  return {
    names: rows.map((r) => r.sprint.name),
    points,
    tasks: rows.map((r) => r.tasks),
    average: last.length > 0 ? round(sum(last) / last.length) : null,
  };
}

export type CfdPlot = {
  xs: number[];
  // The tallest first: each is drawn from 0, over the one before it. The first column is on top.
  stacks: { name: string; top: number[]; values: number[] }[];
};

/** The columns in stacks, the last column at the bottom, as a flow diagram builds up from done. */
export function cfdPlot(v: CFDView): CfdPlot {
  const days = v.days ?? [];
  const columns = v.columns ?? [];
  const xs = days.map((d) => dayOf(d.date) ?? 0);
  const below = days.map(() => 0);
  const built = [...columns].reverse().map((name) => {
    const values = days.map((d) => d.counts[name] ?? 0);
    const top = values.map((n, i) => {
      below[i] = (below[i] ?? 0) + n;
      return below[i] ?? 0;
    });
    return { name, top, values };
  });
  return { xs, stacks: built.reverse() };
}

/** The hours of a duration of the files: 1h30m is 1.5, 45m is 0.75. Null for none. */
export function hoursOf(duration: string | undefined): number | null {
  if (!duration) return null;
  const m = /^(?:(\d+)h)?(?:(\d+)m)?$/.exec(duration);
  if (!m || (m[1] === undefined && m[2] === undefined)) return null;
  return Number(m[1] ?? 0) + Number(m[2] ?? 0) / 60;
}

/** A time of the cycle report: 3.5 日, or as it is under a day. */
export function daysLabel(duration: string): string {
  const h = hoursOf(duration);
  return h !== null && h >= 24 ? `${round(h / 24)} 日` : duration;
}

export type EstimateLine = {
  ulid: string;
  id: string;
  title: string;
  estimate: string;
  actual: string;
  // Where the marks go, in percent of the widest.
  estimateAt: number | null;
  actualAt: number | null;
  over: boolean;
};

export function estimateLines(v: EstimateView): EstimateLine[] {
  const tasks = v.tasks ?? [];
  const hours = tasks.flatMap((t) => [hoursOf(t.estimate) ?? 0, hoursOf(t.actual) ?? 0]);
  const widest = Math.max(...hours, 0);
  const at = (h: number | null) => (h === null || widest === 0 ? null : (h / widest) * 100);
  return tasks.map((t) => {
    const e = hoursOf(t.estimate);
    const a = hoursOf(t.actual);
    return {
      ulid: t.ulid,
      id: t.id,
      title: t.title,
      estimate: t.estimate || "—",
      actual: t.actual || "—",
      estimateAt: at(e),
      actualAt: at(a),
      over: e !== null && a !== null && a > e,
    };
  });
}
