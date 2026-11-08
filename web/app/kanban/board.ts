import type { TaskView } from "../../core/api/client";
import { CATEGORIES, CATEGORY_LABEL, type Category, categoryOf } from "../../core/categories";
import { type Rule, ruleHint } from "../../core/github";
import { defaultStatusIn, type SchemaView } from "../../core/schema";
import { compareStatusPlace, type StatusPlace, statusPlace } from "../../core/status-order";
import type { KanbanSettings } from "../../core/views";

// The key of the row that holds the tasks without an assignee (or outside every project).
export const NO_LANE = "#none";

export type Column = {
  // The status name; the category in the category mode.
  key: string;
  label: string;
  category: Category;
  // The status a card dropped here gets. null in the category mode (the schema of the task says
  // which status of the category) and for a status no schema lists.
  status: string | null;
  // A status the tasks have and no schema lists (or the schema of its store is not read).
  unknown: boolean;
  // The color of the status in the schema, when exactly one store gives the column its status.
  color?: string;
};

export type Lane = {
  // "" when there are no swimlanes; the assignee; the ULID of the project; NO_LANE.
  key: string;
  label: string;
  // The tasks of each column, in the order of Board.columns.
  cells: TaskView[][];
};

export type Board = { columns: Column[]; lanes: Lane[] };

export type BoardOptions = {
  mode: KanbanSettings["columns"];
  swimlane: KanbanSettings["swimlane"];
  schemas: ReadonlyMap<string, SchemaView>;
  // The statuses a project shows (its scoped statuses); a task in another status keeps its column.
  scopedStatuses?: readonly string[];
  // The names of the stores in the order of GET /api/stores.
  storeOrder: readonly string[];
  // The store of a screen of one store: its statuses show even where it has no task.
  store?: string;
};

// columnKeyOf is the key of the column a task is in.
export function columnKeyOf(
  task: TaskView,
  mode: KanbanSettings["columns"],
  schema: SchemaView | undefined,
): string {
  return mode === "category" ? categoryOf(schema, task.status) : task.status;
}

type Def = {
  name: string;
  label: string;
  category: Category;
  place: StatusPlace;
  color?: string;
  owners: number;
};

const asCategory = (c: string): Category =>
  c === "doing" || c === "done" || c === "canceled" ? c : "todo";

// statusColumns joins the statuses of the stores of the tasks (and of the screen): one column for
// a name, labelled by the first store that has it, placed as sort:status places them: by category,
// then by the place in the schema, then by the order of the stores. A canceled status has a column
// only while a task is in it.
function statusColumns(tasks: readonly TaskView[], o: BoardOptions): Column[] {
  const inUse = new Set(tasks.map((t) => t.status));
  const stores = new Set(tasks.map((t) => t.store));
  if (o.store) stores.add(o.store);
  const order = [...o.storeOrder, ...[...stores].filter((s) => !o.storeOrder.includes(s))];
  const defs = new Map<string, Def>();
  order.forEach((store, storeAt) => {
    if (!stores.has(store)) return;
    const schema = o.schemas.get(store);
    for (const s of schema?.statuses ?? []) {
      const had = defs.get(s.name);
      if (had) {
        had.owners++;
        continue;
      }
      defs.set(s.name, {
        owners: 1,
        color: s.color,
        name: s.name,
        label: s.label || s.name,
        category: asCategory(s.category),
        place: statusPlace(schema, s.name, storeAt),
      });
    }
  });
  const shown = o.scopedStatuses ? new Set(o.scopedStatuses) : null;
  const kept = [...defs.values()].filter(
    (d) =>
      inUse.has(d.name) || (d.category !== "canceled" && (shown === null || shown.has(d.name))),
  );
  kept.sort((a, b) => compareStatusPlace(a.place, b.place));
  const columns: Column[] = kept.map((d) => ({
    key: d.name,
    label: d.label,
    category: d.category,
    status: d.name,
    unknown: false,
    color: d.owners === 1 ? d.color : undefined,
  }));
  const unknown = [...inUse].filter((s) => !defs.has(s)).sort();
  for (const s of unknown) {
    columns.push({ key: s, label: s, category: "todo", status: null, unknown: true });
  }
  return columns;
}

function categoryColumns(tasks: readonly TaskView[], o: BoardOptions): Column[] {
  const canceled = tasks.some((t) => categoryOf(o.schemas.get(t.store), t.status) === "canceled");
  return CATEGORIES.filter((c) => c !== "canceled" || canceled).map((c) => ({
    key: c,
    label: CATEGORY_LABEL[c],
    category: c,
    status: null,
    unknown: false,
  }));
}

// loginOf is the login of an assignee ("github.com/koki" is koki).
const loginOf = (assignee: string): string => assignee.slice(assignee.lastIndexOf("/") + 1);

type LaneDef = { key: string; label: string };

function laneKeysOf(t: TaskView, swimlane: BoardOptions["swimlane"]): string[] {
  if (swimlane === "assignee") {
    return t.assignees && t.assignees.length > 0 ? [...new Set(t.assignees)] : [NO_LANE];
  }
  if (swimlane === "project") return [t.project?.ulid ?? NO_LANE];
  return [""];
}

// lanesOf lists the rows: by name for the assignees, in the order they come for the projects, and
// the row for no assignee (no project) last.
function lanesOf(tasks: readonly TaskView[], swimlane: BoardOptions["swimlane"]): LaneDef[] {
  if (tasks.length === 0) return [];
  if (swimlane === "none") return [{ key: "", label: "" }];
  const found = new Map<string, string>();
  for (const t of tasks) {
    for (const key of laneKeysOf(t, swimlane)) {
      if (found.has(key)) continue;
      if (key === NO_LANE)
        found.set(key, swimlane === "assignee" ? "未割り当て" : "プロジェクトなし");
      else if (swimlane === "assignee") found.set(key, loginOf(key));
      else found.set(key, `${t.project?.id ?? ""} ${t.project?.title ?? ""}`.trim());
    }
  }
  const lanes = [...found].map(([key, label]) => ({ key, label }));
  if (swimlane === "assignee")
    lanes.sort((a, b) => a.label.localeCompare(b.label) || a.key.localeCompare(b.key));
  return [...lanes.filter((l) => l.key !== NO_LANE), ...lanes.filter((l) => l.key === NO_LANE)];
}

// buildBoard puts the tasks in the columns and the rows of the board. The tasks of a cell keep the
// order they come in (the rank). A task with two assignees is in the row of each.
export function buildBoard(tasks: readonly TaskView[], o: BoardOptions): Board {
  const columns = o.mode === "category" ? categoryColumns(tasks, o) : statusColumns(tasks, o);
  const at = new Map(columns.map((c, i) => [c.key, i]));
  const lanes: Lane[] = lanesOf(tasks, o.swimlane).map((l) => ({
    ...l,
    cells: columns.map(() => []),
  }));
  const laneAt = new Map(lanes.map((l, i) => [l.key, i]));
  for (const t of tasks) {
    const col = at.get(columnKeyOf(t, o.mode, o.schemas.get(t.store)));
    if (col === undefined) continue;
    for (const key of laneKeysOf(t, o.swimlane)) {
      lanes[laneAt.get(key) ?? -1]?.cells[col]?.push(t);
    }
  }
  return { columns, lanes };
}

/**
 * The words of the columns the rules of a board bring tasks to, by the key of the column. A rule
 * that is off, and a rule of a project the board is not of, say nothing; several rules on one
 * column are joined.
 */
export function autoHints(
  rules: readonly Rule[],
  schema: SchemaView,
  mode: KanbanSettings["columns"],
  project: string | undefined,
): Map<string, string> {
  const out = new Map<string, string>();
  for (const r of rules) {
    const hint = ruleHint(r);
    if (hint === null || !r.enabled || (r.project && r.project.ulid !== project)) continue;
    const status = r.status ?? defaultStatusIn(schema, r.category ?? "");
    const key = mode === "category" ? categoryOf(schema, status) : status;
    const had = out.get(key);
    if (had !== hint) out.set(key, had ? `${had} · ${hint}` : hint);
  }
  return out;
}
