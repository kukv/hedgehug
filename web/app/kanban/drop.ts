import type { ShowView, TaskView } from "../../core/api/client";
import { CATEGORY_LABEL } from "../../core/categories";
import { defaultStatusIn, propEditorOf, type SchemaView } from "../../core/schema";
import type { KanbanSettings } from "../../core/views";
import { type Column, columnKeyOf } from "./board";

type Scoped = NonNullable<SchemaView["scoped"]>[number];
type Transitions = Record<string, string[] | null>;

// The override of a container (its scoped entry in schema.yaml, where: project:<ULID>), by the ULID
// of the container: the project a board is of, or the scope the server names for a task.
function scopedOf(schema: SchemaView, project: string | undefined): Scoped | undefined {
  if (!project) return undefined;
  return (schema.scoped ?? []).find((s) => s.where === `project:${project}`);
}

// scopedStatusesOf is the statuses a project shows, or undefined when it shows all of the store's.
export function scopedStatusesOf(
  schema: SchemaView,
  project: string | undefined,
): string[] | undefined {
  const list = scopedOf(schema, project)?.statuses;
  return list && list.length > 0 ? list : undefined;
}

// allows applies one workflow: only the statuses it lists are limited, and one it lists with no
// targets lets nothing out.
function allows(transitions: Transitions | undefined, from: string, to: string): boolean {
  if (!transitions || !Object.hasOwn(transitions, from)) return true;
  return (transitions[from] ?? []).includes(to);
}

// canTransition is whether a task may move from one status to another: the store and the
// project (when it has an override) both have to allow it, as the server asks.
export function canTransition(
  schema: SchemaView,
  project: string | undefined,
  from: string,
  to: string,
): boolean {
  if (from === to) return true;
  if (!allows(schema.workflow?.transitions, from, to)) return false;
  return allows(scopedOf(schema, project)?.workflow?.transitions, from, to);
}

// requiredFields are the fields a task needs to enter a status: the store's and the project's.
export function requiredFields(
  schema: SchemaView,
  project: string | undefined,
  status: string,
): string[] {
  const out = [...(schema.workflow?.required?.[status] ?? [])];
  for (const f of scopedOf(schema, project)?.workflow?.required?.[status] ?? []) {
    if (!out.includes(f)) out.push(f);
  }
  return out;
}

// What a required field of a task lacks: `fill` are the empty ones the dialog can take in (assignees,
// labels, a property it has an editor for); `elsewhere` are the ones that can only be filled in the
// task detail (the parent, blockers, watchers, repos, a task reference, and what the API does not
// show: sprint, releases, links), or whose value is not known here.
export type RequiredGap = { fill: string[]; elsewhere: string[] };

const empty = (v: unknown): boolean =>
  v === null || v === undefined || v === "" || (Array.isArray(v) && v.length === 0);

// requiredGap sorts the required fields the task lacks. detail is the full task (the list carries
// no watchers or repos): without it those two count as lacking. The fields the task has are in
// neither list.
export function requiredGap(
  t: TaskView,
  names: readonly string[],
  schema: SchemaView,
  detail?: ShowView,
): RequiredGap {
  const gap: RequiredGap = { fill: [], elsewhere: [] };
  for (const name of names) {
    switch (name) {
      case "assignees":
        if (empty(t.assignees)) gap.fill.push(name);
        break;
      case "labels":
        if (empty(t.labels)) gap.fill.push(name);
        break;
      case "parent":
        if (t.parent === null) gap.elsewhere.push(name);
        break;
      case "blocked_by":
        if (empty(t.blocked_by)) gap.elsewhere.push(name);
        break;
      case "watchers":
      case "repos":
        if (!detail || empty(detail[name])) gap.elsewhere.push(name);
        break;
      case "sprint":
      case "releases":
      case "links":
        gap.elsewhere.push(name);
        break;
      default: {
        if (!empty((t.props as Record<string, unknown> | undefined)?.[name])) break;
        const editor = propEditorOf(schema, name);
        (editor === null || editor === "taskref" ? gap.elsewhere : gap.fill).push(name);
      }
    }
  }
  return gap;
}

// silent: nothing to do and nothing to say (a drop on the column the card is in).
export type DropDecision =
  | { ok: true; status: string; fill: string[]; elsewhere: string[] }
  | { ok: false; silent: boolean; title: string; reason: string };

const silent: DropDecision = { ok: false, silent: true, title: "", reason: "" };

function refuse(title: string, reason: string): DropDecision {
  return { ok: false, silent: false, title, reason };
}

export type DropInput = {
  task: TaskView;
  column: Column;
  // The card is dropped in the row it came from.
  sameLane: boolean;
  swimlane: KanbanSettings["swimlane"];
  mode: KanbanSettings["columns"];
  // The schema of the store of the task.
  schema: SchemaView | undefined;
  // The full task, for the required fields the list does not carry (watchers, repos).
  detail?: ShowView;
  // Why the Store of the task cannot be written (in Japanese), or null/absent when it can.
  readOnly?: string | null;
};

// decideDrop says whether the card may go to the column, and the status it then gets. It reads no
// DOM; the checks come in the order the reasons are worth giving in.
export function decideDrop(i: DropInput): DropDecision {
  const { task, column, schema } = i;
  const name = `「${task.id} ${task.title}」`;

  if (i.readOnly) {
    return refuse(
      "閲覧のみの Store のタスクは動かせません",
      `${task.store} は閲覧のみです（${i.readOnly}）。`,
    );
  }
  if (!i.sameLane) {
    return i.swimlane === "project"
      ? refuse(
          "プロジェクトをまたぐ移動はできません",
          `${name}を別のプロジェクトへ移すことになります。ドラッグで変えられるのはステータスだけです。`,
        )
      : refuse(
          "担当者の行をまたぐ移動はできません",
          `${name}の担当者を変えることになります。ドラッグで変えられるのはステータスだけです。`,
        );
  }
  if (columnKeyOf(task, i.mode, schema) === column.key) return silent;
  if (column.unknown) {
    return refuse(`「${column.label}」には移せません`, "どのスキーマにもないステータスです。");
  }
  if (!schema) {
    return refuse(
      "少し待ってからやり直してください",
      `${task.store} の schema.yaml を読み込んでいます。`,
    );
  }

  const to = column.status ?? defaultStatusIn(schema, column.key);
  if (to === "" || !schema.statuses?.some((s) => s.name === to)) {
    return refuse(
      `「${column.label}」には移せません`,
      column.status === null
        ? `${task.store} の schema.yaml に${CATEGORY_LABEL[column.category]}のステータスがありません。`
        : `${name}は ${task.store} にあり、その schema.yaml に「${column.label}」というステータスがありません。`,
    );
  }
  // The override the server applies to the task: that of the nearest ancestor (or the task itself)
  // that has one, not necessarily its project (TaskView.scope).
  const project = task.scope || undefined;
  if (!canTransition(schema, project, task.status, to)) {
    const label = (s: string) => schema.statuses?.find((x) => x.name === s)?.label || s;
    return refuse(
      `「${label(task.status)}」から「${label(to)}」には移せません`,
      `${name}は、遷移の制約（schema.yaml の workflow.transitions）で許されていません。`,
    );
  }
  const gap = requiredGap(task, requiredFields(schema, project, to), schema, i.detail);
  return { ok: true, status: to, ...gap };
}
