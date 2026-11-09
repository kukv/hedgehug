import type { TaskView } from "./api/client";
import type { SchemaView } from "./schema";

export type SchemaUsage = {
  statuses: Record<string, number>;
  /** The tasks that hold a value of the property. */
  props: Record<string, number>;
  /** The tasks that hold the value, of each value of an enum or multi_enum property. */
  values: Record<string, Record<string, number>>;
};

const empty = (v: unknown): boolean =>
  v === undefined || v === null || v === "" || (Array.isArray(v) && v.length === 0);

const zero = (names: string[]): Record<string, number> =>
  Object.fromEntries(names.map((n) => [n, 0]));

const bump = (m: Record<string, number>, k: string): void => {
  if (Object.hasOwn(m, k)) m[k] = (m[k] ?? 0) + 1;
};

/** How many of the tasks use each definition of the schema; one that nobody uses is 0. */
export function schemaUsage(schema: SchemaView, tasks: readonly TaskView[]): SchemaUsage {
  const u: SchemaUsage = {
    statuses: zero((schema.statuses ?? []).map((s) => s.name)),
    props: zero(Object.keys(schema.props)),
    values: {},
  };
  for (const [name, p] of Object.entries(schema.props)) {
    if (p.type === "enum" || p.type === "multi_enum") u.values[name] = zero(p.values ?? []);
  }
  for (const t of tasks) {
    bump(u.statuses, t.status);
    for (const name of Object.keys(u.props)) {
      const v = t.props?.[name];
      if (empty(v)) continue;
      bump(u.props, name);
      const counts = u.values[name];
      if (!counts) continue;
      // a multi_enum holds each of its values once for the task
      for (const x of new Set(Array.isArray(v) ? v : [v])) bump(counts, String(x));
    }
  }
  return u;
}
