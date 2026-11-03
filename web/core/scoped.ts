import type { SchemaView } from "./schema";

type ScopedDef = NonNullable<SchemaView["scoped"]>[number];

/** Status to the statuses it may move to, or to the fields needed to enter it. */
export type Lists = Record<string, string[]>;

/**
 * The override of a project as the screen edits it. statuses is empty while the project shows the
 * store's statuses; the others are only what the project adds to the store's.
 */
export type ScopedDraft = { statuses: string[]; transitions: Lists; required: Lists };

/** The body of PUT /api/schema/scoped/{project}: only the parts that changed, each as a whole. */
type ScopedBody = Partial<ScopedDraft>;

// The built-in fields a workflow may require, besides the properties (internal/schema requirableFields).
const REQUIRABLE_BUILTINS = [
  "parent",
  "assignees",
  "watchers",
  "labels",
  "repos",
  "blocked_by",
  "sprint",
  "releases",
  "links",
];

const listsOf = (m: Record<string, string[] | null> | undefined): Lists =>
  Object.fromEntries(Object.entries(m ?? {}).map(([k, v]) => [k, [...(v ?? [])]]));

/** The draft of an override as the server holds it; none (undefined) is the store's values. */
export function draftOf(def: ScopedDef | undefined): ScopedDraft {
  return {
    statuses: [...(def?.statuses ?? [])],
    transitions: listsOf(def?.workflow?.transitions),
    required: listsOf(def?.workflow?.required),
  };
}

const namesOf = (schema: SchemaView): string[] => (schema.statuses ?? []).map((s) => s.name);

/** The statuses the project shows, in the order of the schema. */
export function shownStatuses(schema: SchemaView, draft: ScopedDraft): string[] {
  const all = namesOf(schema);
  return draft.statuses.length === 0 ? all : all.filter((n) => draft.statuses.includes(n));
}

/** The statuses that cannot be hidden: the default of each category. */
export function lockedStatuses(schema: SchemaView): Set<string> {
  return new Set((schema.statuses ?? []).filter((s) => s.default).map((s) => s.name));
}

/** The draft with a status shown or hidden; showing them all is the store's (an empty list). */
export function setShown(
  schema: SchemaView,
  draft: ScopedDraft,
  name: string,
  on: boolean,
): ScopedDraft {
  if (!on && lockedStatuses(schema).has(name)) return draft;
  const next = new Set(shownStatuses(schema, draft));
  if (on) next.add(name);
  else next.delete(name);
  const all = namesOf(schema);
  const listed = all.filter((n) => next.has(n));
  return { ...draft, statuses: listed.length === all.length ? [] : listed };
}

/** The draft's lists with values added to a key (the ones it has are not repeated). */
export function addValues(lists: Lists, key: string, values: string[]): Lists {
  return { ...lists, [key]: [...new Set([...(lists[key] ?? []), ...values])] };
}

/** The draft's lists without a value; a key left with none is taken off. */
export function removeValue(lists: Lists, key: string, value: string): Lists {
  const { [key]: old, ...rest } = lists;
  const left = (old ?? []).filter((v) => v !== value);
  return left.length === 0 ? rest : { ...rest, [key]: left };
}

/** The fields a status may require: the properties by name, then the built-in fields. */
export function requirableFields(schema: SchemaView): string[] {
  return [...Object.keys(schema.props).sort(), ...REQUIRABLE_BUILTINS];
}

const sameSet = (a: readonly string[], b: readonly string[]): boolean =>
  a.length === b.length && a.every((v) => b.includes(v));

function sameLists(a: Lists, b: Lists): boolean {
  const keys = Object.keys(a);
  return (
    keys.length === Object.keys(b).length &&
    keys.every((k) => Object.hasOwn(b, k) && sameSet(a[k] ?? [], b[k] ?? []))
  );
}

/**
 * The body of the PUT that saves draft over saved: the parts that changed, each as the whole of
 * the draft's part. An empty part goes back to the store's. {} while nothing changed.
 */
export function putBody(schema: SchemaView, saved: ScopedDraft, draft: ScopedDraft): ScopedBody {
  const body: ScopedBody = {};
  if (!sameSet(shownStatuses(schema, saved), shownStatuses(schema, draft))) {
    body.statuses = draft.statuses;
  }
  if (!sameLists(saved.transitions, draft.transitions)) body.transitions = draft.transitions;
  if (!sameLists(saved.required, draft.required)) body.required = draft.required;
  return body;
}

/** The query for the tasks of a project that break the schema. */
export const violationQuery = (project: string): string => `is:invalid project:${project}`;
