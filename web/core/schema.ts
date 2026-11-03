import { type Api, type Schemas, unwrap } from "./api/client";
import type { Connection } from "./connection";
import { callAll } from "./listeners";

export type SchemaView = Schemas["SchemaView"];
export type KindDef = Schemas["KindDef"];

// The name that stands for "no parent" in the parents of a kind.
const ROOT = "root";

// SchemaCache keeps the schema.yaml of the stores the screen has asked for,
// and fetches it again when the store says it changed. A schema that did not
// change stays the same object.
export class SchemaCache {
  private readonly api: Api;
  private readonly schemas = new Map<string, SchemaView>();
  private readonly sigs = new Map<string, string>();
  private readonly loading = new Map<string, Promise<SchemaView>>();
  private readonly seqs = new Map<string, number>();
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection }) {
    this.api = opts.api;
    opts.conn.on("schema.changed", (e) => this.refetch([e.store]));
    opts.conn.on("resync", () => this.refetch([...this.schemas.keys()]));
  }

  get(store: string): SchemaView | undefined {
    return this.schemas.get(store);
  }

  // load is the schema of the store: the one held, or the one fetched.
  load(store: string): Promise<SchemaView> {
    const have = this.schemas.get(store);
    if (have) return Promise.resolve(have);
    return this.fetch(store);
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  private fetch(store: string): Promise<SchemaView> {
    const running = this.loading.get(store);
    if (running) return running;
    const seq = (this.seqs.get(store) ?? 0) + 1;
    this.seqs.set(store, seq);
    const p = unwrap(this.api.GET("/api/schema", { params: { query: { store } } }))
      .then((s) => {
        // An older fetch that ends after a newer one has started gives way to it.
        if (this.seqs.get(store) !== seq) {
          return this.loading.get(store) ?? this.schemas.get(store) ?? s;
        }
        const sig = JSON.stringify(s);
        if (this.sigs.get(store) === sig) return this.schemas.get(store) as SchemaView;
        this.schemas.set(store, s);
        this.sigs.set(store, sig);
        callAll(this.subs);
        return s;
      })
      .finally(() => {
        if (this.loading.get(store) === p) this.loading.delete(store);
      });
    this.loading.set(store, p);
    return p;
  }

  // refetch fetches again the schemas it holds of stores; one nobody asked
  // for is left alone. A failure leaves the schema held; the next change or
  // resync tries again.
  private refetch(stores: string[]): void {
    for (const store of stores) {
      if (!this.schemas.has(store)) continue;
      this.loading.delete(store); // a fetch on its way may have read the old file
      void this.fetch(store).catch(() => undefined);
    }
  }
}

const kindOf = (s: SchemaView, name: string): KindDef | undefined =>
  (s.kinds ?? []).find((k) => k.name === name);

const kindLabel = (s: SchemaView, name: string): string => {
  const k = kindOf(s, name);
  return k?.label || name;
};

// canParent is whether a task of childKind may sit under a parent of
// parentKind (null: at the top, with no parent). Parents that the kind does
// not list (null) mean anywhere; the top is written "root" in the list.
export function canParent(s: SchemaView, childKind: string, parentKind: string | null): boolean {
  const k = kindOf(s, childKind);
  if (!k) return false;
  if (!k.parents) return true;
  return k.parents.includes(parentKind ?? ROOT);
}

function listJa(names: string[]): string {
  if (names.length <= 1) return names.join("");
  return `${names.slice(0, -1).join("、")}と${names[names.length - 1]}`;
}

// parentRule says where a kind can be put, for the tooltip of a kind that
// cannot be chosen: 「エピックを置ける親はルートとプロジェクトだけです」.
export function parentRule(s: SchemaView, childKind: string): string {
  const label = kindLabel(s, childKind);
  const parents = kindOf(s, childKind)?.parents;
  if (!parents) return `${label}はどこにでも置けます`;
  if (parents.length === 0) return `${label}は親に置けません`;
  if (parents.length === 1 && parents[0] === ROOT) return `${label}はルートにだけ置けます`;
  const names = parents.map((p) => (p === ROOT ? "ルート" : kindLabel(s, p)));
  return `${label}を置ける親は${listJa(names)}だけです`;
}

// defaultStatusIn is the status a task gets when it moves into a category
// (todo, doing, done or canceled): the one the schema marks default, else the
// first of the category. It is "" when the category has none.
export function defaultStatusIn(s: SchemaView, category: string): string {
  const of = (s.statuses ?? []).filter((x) => x.category === category);
  return (of.find((x) => x.default) ?? of[0])?.name ?? "";
}

export type PropEditor =
  | "status"
  | "kind"
  | "enum"
  | "multi_enum"
  | "date"
  | "datetime"
  | "number"
  | "bool"
  | "duration"
  | "url"
  | "taskref"
  | "user"
  | "text";

// The fields every task has, and what edits them.
const BUILTIN: Record<string, PropEditor> = {
  status: "status",
  kind: "kind",
  assignees: "user",
  watchers: "user",
  blocked_by: "taskref",
  title: "text",
  labels: "text",
  repos: "text",
  body: "text",
};

// The types of a property of the schema (internal/schema PropType) and the
// editor each gets.
const BY_TYPE: Record<string, PropEditor> = {
  text: "text",
  number: "number",
  bool: "bool",
  date: "date",
  datetime: "datetime",
  duration: "duration",
  enum: "enum",
  multi_enum: "multi_enum",
  user: "user",
  task_ref: "taskref",
  url: "url",
};

// propEditorOf is the editor for the field or property name, or null when
// the schema does not define it.
export function propEditorOf(s: SchemaView, name: string): PropEditor | null {
  const builtin = BUILTIN[name];
  if (builtin) return builtin;
  const p = s.props[name];
  if (!p) return null;
  return BY_TYPE[p.type] ?? "text";
}
