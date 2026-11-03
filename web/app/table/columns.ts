import type { TaskView } from "../../core/api/client";
import type { PatchField } from "../../core/save";
import type { SchemaView } from "../../core/schema";
import { statusRank } from "../../core/status-order";

// A column of the table. width is in rem.
export type ColumnSpec = { id: string; label: string; readonly: boolean; width: number };

// The columns a view can name besides the properties of the schema, in the order of
// internal/view Builtins. ID, parent, created and updated are shown, not edited.
const BUILTIN: ColumnSpec[] = [
  { id: "id", label: "ID", readonly: true, width: 6.5 },
  { id: "title", label: "タイトル", readonly: false, width: 22 },
  { id: "kind", label: "種類", readonly: false, width: 8 },
  { id: "status", label: "ステータス", readonly: false, width: 10 },
  { id: "assignee", label: "担当", readonly: false, width: 12 },
  { id: "label", label: "ラベル", readonly: false, width: 12 },
  { id: "parent", label: "親", readonly: true, width: 6.5 },
  { id: "created", label: "作成", readonly: true, width: 6.5 },
  { id: "updated", label: "更新", readonly: true, width: 6.5 },
];

// The first column of a table that lists the tasks of more than one store; it is not a column of a
// view, so it is never saved.
export const STORE_COLUMN: ColumnSpec = { id: "@store", label: "Store", readonly: true, width: 8 };

// isBuiltinColumn is whether the column is a field of a task, not a property of the schema.
export const isBuiltinColumn = (id: string): boolean =>
  id === STORE_COLUMN.id || BUILTIN.some((c) => c.id === id);

// Names of a schema that are not a column: the builtin ones, and blocked_by, which a view cannot
// name (the Relations section of the detail edits it).
const NOT_A_PROP = new Set([...BUILTIN.map((c) => c.id), "blocked_by"]);

function propWidth(def: { type: string; multi?: boolean }): number {
  if (def.multi || def.type === "multi_enum") return 12;
  if (def.type === "number" || def.type === "bool") return 6.5;
  if (def.type === "date") return 8;
  return 9;
}

// columnCandidates are the columns the menu offers: the builtin ones, then the properties of the
// schemas by name. Several stores give the union of their properties, and the label is the one of
// the first store that has the property.
export function columnCandidates(schemas: SchemaView[]): ColumnSpec[] {
  const props = new Map<string, ColumnSpec>();
  for (const sc of schemas) {
    for (const [name, def] of Object.entries(sc.props)) {
      if (NOT_A_PROP.has(name) || props.has(name)) continue;
      props.set(name, {
        id: name,
        label: def.label || name,
        readonly: def.type === "task_ref",
        width: propWidth(def),
      });
    }
  }
  return [...BUILTIN, ...[...props.keys()].sort().map((name) => props.get(name) as ColumnSpec)];
}

// shownColumns are the columns of a view in its order. A name that no schema defines is left out
// (the view keeps it; a store that has the property may be added later), and so is a repeated one.
export function shownColumns(selected: string[], candidates: ColumnSpec[]): ColumnSpec[] {
  const byId = new Map(candidates.map((c) => [c.id, c]));
  const out: ColumnSpec[] = [];
  for (const id of selected) {
    const spec = byId.get(id);
    if (spec && !out.includes(spec)) out.push(spec);
  }
  return out;
}

// toggleColumn adds a column at the end, or takes it out; the names the table does not show stay.
export function toggleColumn(selected: string[], id: string, on: boolean): string[] {
  const rest = selected.filter((c) => c !== id);
  return on ? [...rest, id] : rest;
}

// patchFieldOf is the field a cell edits; null for a column that is only shown.
export function patchFieldOf(spec: ColumnSpec): PatchField | null {
  if (spec.readonly) return null;
  switch (spec.id) {
    case "title":
    case "kind":
    case "status":
      return spec.id;
    case "assignee":
      return "assignees";
    case "label":
      return "labels";
    default:
      return `props.${spec.id}`;
  }
}

// What a column sorts by: a number, a text, or undefined for a task with no value (those go last
// whichever way the column is sorted).
export type SortKey = string | number | undefined;

// What sorting and grouping need to know besides the task.
export type SortCtx = {
  schemaOf: (store: string) => SchemaView | undefined;
  taskOf: (ulid: string) => TaskView | undefined;
  // The names of the stores in the order of GET /api/stores.
  storeOrder: string[];
};

const asKey = (v: string): SortKey => (v === "" ? undefined : v);

function propKey(t: TaskView, name: string, ctx: SortCtx): SortKey {
  const v = (t.props as Record<string, unknown>)[name];
  if (v === undefined || v === null || v === "") return undefined;
  if (Array.isArray(v)) return v.length === 0 ? undefined : v.map(String).join(",");
  if (typeof v === "boolean") return v ? 1 : 0;
  if (typeof v === "number") return v;
  const def = ctx.schemaOf(t.store)?.props[name];
  if (def?.type === "task_ref") return ctx.taskOf(String(v))?.id ?? String(v);
  const values = def?.values;
  if (values && values.length > 0) {
    const at = values.indexOf(String(v));
    return at >= 0 ? at : values.length;
  }
  return String(v);
}

// sortKey is the value the column sorts the task by.
export function sortKey(t: TaskView, id: string, ctx: SortCtx): SortKey {
  switch (id) {
    case "@store": {
      const i = ctx.storeOrder.indexOf(t.store);
      return i < 0 ? ctx.storeOrder.length : i;
    }
    case "id":
      return t.id;
    case "title":
      return asKey(t.title);
    case "kind":
      return t.kind;
    case "status":
      return statusRank(ctx.schemaOf(t.store), t.status, ctx.storeOrder.indexOf(t.store));
    case "assignee":
      return asKey(t.assignees?.[0] ?? "");
    case "label":
      return asKey((t.labels ?? []).join(","));
    case "parent":
      return t.parent ? (ctx.taskOf(t.parent)?.id ?? undefined) : undefined;
    case "created":
      return Date.parse(t.created);
    case "updated":
      return Date.parse(t.updated);
    default:
      return propKey(t, id, ctx);
  }
}

// compareKeys orders numbers as numbers and texts the way a person reads them (REL-2 before
// REL-10); a number comes before a text.
export function compareKeys(a: unknown, b: unknown): number {
  if (typeof a === "number" && typeof b === "number") return a - b;
  if (typeof a === "number") return -1;
  if (typeof b === "number") return 1;
  return String(a).localeCompare(String(b), "ja", { numeric: true });
}
