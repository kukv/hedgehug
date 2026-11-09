import { ApiError } from "../../core/api/client";
import { errorMessage } from "../../core/messages";
import type { SchemaView } from "../../core/schema";
import { propNames } from "../../core/schema-edit";
import type { SchemaUsage } from "../../core/schema-usage";
import { kbdLabel } from "../keys";

/** The keys that move a row: ⌥↑ / ⌥↓ on a Mac, Alt+↑ / Alt+↓ elsewhere. */
export const moveKeys = (): string => `${kbdLabel("alt+arrowup")} / ${kbdLabel("alt+arrowdown")}`;

/** The number of tasks, or a dash until the tasks are read. */
export const count = (usage: SchemaUsage | null, n: number | undefined): string =>
  usage && n !== undefined ? String(n) : "—";

export const propNote = (p: { multi?: boolean }): string => (p.multi ? "複数" : "");

type PropEntry = [string, SchemaView["props"][string]];

// The properties of the schema, in the order of the schema.
export const propEntries = (schema: SchemaView | undefined): PropEntry[] =>
  schema
    ? propNames(schema).flatMap((n) => {
        const p = schema.props[n];
        return p ? [[n, p] as PropEntry] : [];
      })
    : [];

/** What a refused edit says, and the lines of the tasks a change would break. */
export function failureOf(e: unknown): { text: string; violations?: string[] } {
  if (e instanceof ApiError) {
    const v = e.args.violations;
    if (e.type === "schema.tasks_invalid" && Array.isArray(v)) {
      return { text: errorMessage(e), violations: v.map(String) };
    }
  }
  return { text: errorMessage(e) };
}
