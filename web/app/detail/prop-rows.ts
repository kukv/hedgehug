import type { SchemaView } from "../../core/schema";
import { type PropEditor, propEditorOf } from "../../core/schema";
import { propNames } from "../../core/schema-edit";
import type { Option } from "./field-types";

export type PropRow = {
  name: string;
  label: string;
  editor: PropEditor | null;
  multi: boolean;
  options: string[];
  value: unknown;
};

const builtIn = new Set(["status", "kind", "blocked_by"]);

// propRows lists the properties the detail panel edits: those the schema defines (in the order of schema.yaml), then the keys the task has that the schema does not (read-only, editor null).
// Task references are not here: the relations section shows them.
export function propRows(sc: SchemaView, values: Record<string, unknown>): PropRow[] {
  const rows: PropRow[] = [];
  for (const name of propNames(sc)) {
    if (builtIn.has(name)) continue;
    const editor = propEditorOf(sc, name);
    if (editor === "taskref") continue;
    const def = sc.props[name];
    if (!def) continue;
    rows.push({
      name,
      label: def.label || name,
      editor,
      multi: def.multi === true,
      options: def.values ?? [],
      value: values[name] ?? null,
    });
  }
  for (const name of Object.keys(values)) {
    if (name in sc.props || builtIn.has(name)) continue;
    rows.push({ name, label: name, editor: null, multi: false, options: [], value: values[name] });
  }
  return rows;
}

// statusOptions are the statuses of the schema as the choices of a select: "進行中 · doing".
export function statusOptions(sc: SchemaView | undefined): Option[] {
  return (sc?.statuses ?? []).map((s) => ({
    value: s.name,
    label: `${s.label || s.name} · ${s.category}`,
  }));
}
