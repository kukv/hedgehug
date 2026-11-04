import type { TaskView } from "../../core/api/client";
import { defaultStatusIn, type SchemaView } from "../../core/schema";
import type { FlatRow } from "./flatten";
import { checkParent, type MoveArgs, type Rejection } from "./placement";

// The key as the tree sees it. mod is ⌘ on macOS and Ctrl elsewhere.
export type KeyInput = { key: string; shift: boolean; alt: boolean; mod: boolean };

export type KeyContext = {
  rows: FlatRow[]; // the visible rows, in order
  selected: string | null;
  schema: SchemaView | undefined;
  viewRoot: string | null; // the project the view shows, or null for a workspace
  taskOf: (ulid: string) => TaskView | undefined;
  idOf?: (t: TaskView) => string; // the ID as the screen writes it (with its Store where shared)
};

// What the tree should do for a key. The caller runs it: this function only
// decides.
export type KeyAction =
  | { type: "none" }
  | { type: "select"; ulid: string }
  | { type: "toggle"; ulid: string; open: boolean }
  | { type: "add-below"; after: string; parent: string | null }
  | { type: "open-detail"; ulid: string }
  | { type: "move"; ulid: string; to: MoveArgs }
  | { type: "set-status"; ulid: string; status: string }
  | { type: "menu"; which: "status" | "kind"; ulid: string }
  | { type: "reject"; rejection: Rejection };

// The task a write action acts on (selected is the row a refusal belongs to), or null for an action
// that only moves the view.
export function writeTargetOf(a: KeyAction, selected: string | null): string | null {
  switch (a.type) {
    case "add-below":
      return a.after;
    case "move":
    case "set-status":
    case "menu":
      return a.ulid;
    case "reject":
      return selected;
    default:
      return null;
  }
}

const NONE: KeyAction = { type: "none" };

// The keys that act through the kinds and the statuses of the schema. Pressed before the schema is
// read, they wait for it instead of doing nothing.
export function needsSchema(k: KeyInput): boolean {
  if (k.mod || k.alt) return false;
  return k.key === "Tab" || k.key === " " || "sSkK".includes(k.key);
}

export function keyAction(k: KeyInput, c: KeyContext): KeyAction {
  const i = c.selected === null ? -1 : c.rows.findIndex((r) => r.ulid === c.selected);
  const row = i >= 0 ? c.rows[i] : undefined;

  if ((k.key === "ArrowDown" || k.key === "ArrowUp") && !k.mod) {
    if (k.alt) return k.shift || !row ? NONE : reorder(k.key === "ArrowUp" ? -1 : 1, row, c);
    if (c.rows.length === 0) return NONE;
    const down = k.key === "ArrowDown";
    const next =
      i < 0
        ? down
          ? 0
          : c.rows.length - 1
        : Math.min(Math.max(i + (down ? 1 : -1), 0), c.rows.length - 1);
    const target = c.rows[next];
    return target ? { type: "select", ulid: target.ulid } : NONE;
  }

  if (!row) return NONE;

  switch (k.key) {
    case "ArrowRight": {
      if (row.hasChildren && !row.expanded) return { type: "toggle", ulid: row.ulid, open: true };
      const first = c.rows[i + 1];
      return row.hasChildren && first?.parent === row.ulid
        ? { type: "select", ulid: first.ulid }
        : NONE;
    }
    case "ArrowLeft": {
      if (row.hasChildren && row.expanded) return { type: "toggle", ulid: row.ulid, open: false };
      const parent =
        row.parent === c.viewRoot ? undefined : c.rows.find((r) => r.ulid === row.parent);
      return parent ? { type: "select", ulid: parent.ulid } : NONE;
    }
    case "Enter":
      if (k.mod) return { type: "open-detail", ulid: row.ulid };
      if (k.shift || k.alt) return NONE;
      return { type: "add-below", after: row.ulid, parent: row.parent };
    case "Tab":
      if (k.mod || k.alt) return NONE;
      return k.shift ? outdent(row, c) : indent(row, c);
    case " ": {
      if (k.mod || k.alt || k.shift || !c.schema) return NONE;
      const done = c.schema.statuses?.find((s) => s.name === row.task.status)?.category === "done";
      const category = done ? "todo" : "done";
      const status = defaultStatusIn(c.schema, category);
      if (status === "") {
        // The schema has no status of that category: there is nothing to patch to.
        return {
          type: "reject",
          rejection: {
            title: done ? "未着手に戻せません" : "完了にできません",
            reason: `${row.task.store} の schema.yaml にカテゴリが ${category} のステータスがありません。`,
          },
        };
      }
      return { type: "set-status", ulid: row.ulid, status };
    }
    case "s":
    case "S":
      return k.mod || k.alt ? NONE : { type: "menu", which: "status", ulid: row.ulid };
    case "k":
    case "K":
      return k.mod || k.alt ? NONE : { type: "menu", which: "kind", ulid: row.ulid };
    default:
      return NONE;
  }
}

// The siblings share the parent and the store: the top rows of a view over several stores sit under
// the same null, and a task cannot be put before, after or under one of another store.
function siblingsOf(row: FlatRow, c: KeyContext): FlatRow[] {
  return c.rows.filter((r) => r.parent === row.parent && r.task.store === row.task.store);
}

function reorder(dir: -1 | 1, row: FlatRow, c: KeyContext): KeyAction {
  const sib = siblingsOf(row, c);
  const other = sib[sib.findIndex((r) => r.ulid === row.ulid) + dir];
  if (!other) return NONE;
  return {
    type: "move",
    ulid: row.ulid,
    to: dir < 0 ? { before: other.ulid } : { after: other.ulid },
  };
}

function indent(row: FlatRow, c: KeyContext): KeyAction {
  const sib = siblingsOf(row, c);
  const prev = sib[sib.findIndex((r) => r.ulid === row.ulid) - 1];
  if (!prev || !c.schema) return NONE;
  const rejection = checkParent(c.schema, row.task, prev.task, c.idOf);
  if (rejection) return { type: "reject", rejection };
  return { type: "move", ulid: row.ulid, to: { parent: prev.ulid } };
}

function outdent(row: FlatRow, c: KeyContext): KeyAction {
  if (row.parent === c.viewRoot || row.parent === null || !c.schema) return NONE;
  const p = c.taskOf(row.parent);
  if (!p) return NONE;
  const grand = p.parent ? (c.taskOf(p.parent) ?? null) : null;
  if (p.parent && !grand) return NONE;
  const rejection = checkParent(c.schema, row.task, grand, c.idOf);
  if (rejection) return { type: "reject", rejection };
  return {
    type: "move",
    ulid: row.ulid,
    to: p.parent ? { parent: p.parent, after: p.ulid } : { root: true, after: p.ulid },
  };
}
