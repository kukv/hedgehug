import { ApiError } from "../../core/api/client";
import { errorMessage } from "../../core/messages";
import type { SchemaView } from "../../core/schema";
import { propNames } from "../../core/schema-edit";
import type { SchemaUsage } from "../../core/schema-usage";

/** The number of tasks, or a dash until the tasks are read. */
export const count = (usage: SchemaUsage | null, n: number | undefined): string =>
  usage && n !== undefined ? String(n) : "—";

export const propNote = (p: { multi?: boolean; symmetric?: boolean; inverse?: string }): string =>
  [p.multi ? "複数" : "", p.symmetric ? "双方向" : "", p.inverse ? `逆方向: ${p.inverse}` : ""]
    .filter(Boolean)
    .join(" · ");

type PropEntry = [string, SchemaView["props"][string]];

// The properties and the relations of the schema, in the order of the schema. A task_ref property is
// a relation; the relations tab has them, the properties tab the rest.
const allProps = (schema: SchemaView | undefined): PropEntry[] =>
  schema
    ? propNames(schema).flatMap((n) => {
        const p = schema.props[n];
        return p ? [[n, p] as PropEntry] : [];
      })
    : [];
export const propEntries = (schema: SchemaView | undefined): PropEntry[] =>
  allProps(schema).filter(([, p]) => p.type !== "task_ref");
export const relationEntries = (schema: SchemaView | undefined): PropEntry[] =>
  allProps(schema).filter(([, p]) => p.type === "task_ref");

/** What a refused edit says: the server's own sentence for a definition in use, the lines a change would break. */
export function failureOf(e: unknown): { text: string; violations?: string[] } {
  if (e instanceof ApiError) {
    if (e.type === "schema.in_use" && e.detail) return { text: e.detail };
    const v = e.args.violations;
    if (e.type === "schema.tasks_invalid" && Array.isArray(v)) {
      return { text: errorMessage(e), violations: v.map(String) };
    }
  }
  return { text: errorMessage(e) };
}
