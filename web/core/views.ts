import { type Api, ApiError, type Schemas, unwrap } from "./api/client";
import type { TaskCache } from "./cache";
import type { Connection } from "./connection";
import { callAll } from "./listeners";
import type { StoreDirectory, StoreTone } from "./stores";

export type Layout = "tree" | "kanban" | "table";
export type SavedView = Schemas["SavedView"];
export type ViewSaveBody = Schemas["ViewSaveInputBody"];

export type KanbanSettings = {
  columns: "status" | "category";
  swimlane: "none" | "assignee" | "project";
};
export type TableSettings = {
  columns: string[];
  groupBy: "none" | "parent" | "status" | "assignee" | "project";
};
// What a screen of a view holds while it is open: the saved settings, and what was changed since.
export type ViewDraft = {
  layout: Layout;
  query: string;
  kanban: KanbanSettings;
  table: TableSettings;
};

// The defaults of internal/view: what a view that leaves a setting out has.
export const DEFAULT_KANBAN: KanbanSettings = { columns: "status", swimlane: "none" };
export const DEFAULT_TABLE: TableSettings = {
  columns: ["id", "title", "status", "assignee", "due"],
  groupBy: "none",
};

const LAYOUTS: readonly string[] = ["tree", "kanban", "table"];
const KANBAN_COLUMNS: readonly string[] = ["status", "category"];
const SWIMLANES: readonly string[] = ["none", "assignee", "project"];
const GROUP_BYS: readonly string[] = ["none", "parent", "status", "assignee", "project"];

// draftOf reads a saved view; a value this client does not know (a newer server) is the default.
export function draftOf(v: SavedView): ViewDraft {
  const columns = v.table.columns ?? [];
  return {
    layout: LAYOUTS.includes(v.layout) ? (v.layout as Layout) : "tree",
    query: v.query,
    kanban: {
      columns: KANBAN_COLUMNS.includes(v.kanban.columns)
        ? (v.kanban.columns as KanbanSettings["columns"])
        : DEFAULT_KANBAN.columns,
      swimlane: SWIMLANES.includes(v.kanban.swimlane)
        ? (v.kanban.swimlane as KanbanSettings["swimlane"])
        : DEFAULT_KANBAN.swimlane,
    },
    table: {
      columns: columns.length > 0 ? [...columns] : [...DEFAULT_TABLE.columns],
      groupBy: GROUP_BYS.includes(v.table.group_by)
        ? (v.table.group_by as TableSettings["groupBy"])
        : DEFAULT_TABLE.groupBy,
    },
  };
}

// saveBodyOf is the body of a PUT that saves every setting of d: hh view save with all its flags.
export function saveBodyOf(d: ViewDraft): ViewSaveBody {
  return {
    layout: d.layout,
    query: d.query,
    columns: [...d.table.columns],
    group_by: d.table.groupBy,
    kanban_columns: d.kanban.columns,
    swimlane: d.kanban.swimlane,
  };
}

// sameDraft says whether a and b hold the same settings. The layout is left out: a screen changes it
// at once (PUT {layout}) and does not count it as an unsaved change.
export function sameDraft(a: ViewDraft, b: ViewDraft): boolean {
  return (
    a.query.trim() === b.query.trim() &&
    a.kanban.columns === b.kanban.columns &&
    a.kanban.swimlane === b.kanban.swimlane &&
    a.table.groupBy === b.table.groupBy &&
    a.table.columns.length === b.table.columns.length &&
    a.table.columns.every((c, i) => c === b.table.columns[i])
  );
}

export type ViewEntry = { store: string; tone: StoreTone; view: SavedView };

const byName = (a: SavedView, b: SavedView): number => a.name.localeCompare(b.name, "ja");

function sameEntry(a: ViewEntry, b: ViewEntry): boolean {
  return (
    a.store === b.store && a.tone === b.tone && JSON.stringify(a.view) === JSON.stringify(b.view)
  );
}

// ViewDirectory is the saved views of every store, for the sidebar, the palette and the screens of a
// view. The API lists them store by store (GET /api/views?store=), so each store is fetched on its
// own; views.changed fetches that store again, resync and a change of the stores fetch them all.
export class ViewDirectory {
  private readonly api: Api;
  private readonly cache: TaskCache;
  private readonly dir: StoreDirectory;
  private names: string[] = [];
  private readonly byStore = new Map<string, SavedView[]>();
  private readonly fetched = new Set<string>();
  private readonly unreadable = new Set<string>();
  private readonly seqs = new Map<string, number>();
  private result: ViewEntry[] = [];
  private isLoaded = false;
  private failedKey = "";
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection; stores: StoreDirectory; cache: TaskCache }) {
    this.api = opts.api;
    this.cache = opts.cache;
    this.dir = opts.stores;
    opts.stores.subscribe(() => this.onStores());
    opts.conn.on("views.changed", (e) => {
      if (this.names.includes(e.store)) void this.fetch(e.store);
    });
    opts.conn.on("resync", () => {
      for (const n of this.names) void this.fetch(n);
    });
    this.onStores();
  }

  // list is every view: in the order of the stores, then by name.
  list(): ViewEntry[] {
    return this.result;
  }

  get(store: string, name: string): ViewEntry | undefined {
    return this.result.find((e) => e.store === store && e.view.name === name);
  }

  // loaded is true once the stores are known and each has been asked for its views (a store that
  // could not be read counts: its views are none).
  loaded(): boolean {
    return this.isLoaded;
  }

  // failed is true when the last read of the views of a store did not succeed: what list() holds for
  // it may be missing views, so a screen must not take it as the whole.
  failed(store: string): boolean {
    return this.unreadable.has(store);
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  // refresh reads the views of a store again: for a change the server does not tell (an undo of a
  // write that its poll did not see leaves the store as it was).
  refresh(store: string): void {
    if (this.names.includes(store)) void this.fetch(store);
  }

  // save creates the view, or changes the settings body holds (PUT: what is not sent stays).
  async save(store: string, name: string, body: ViewSaveBody): Promise<SavedView> {
    const r = await unwrap(
      this.api.PUT("/api/views/{name}", { params: { path: { name }, query: { store } }, body }),
    );
    this.cache.noteOwnCommit(r.commit);
    this.apply(store, (vs) => [...vs.filter((v) => v.name !== name), r.view]);
    return r.view;
  }

  // update changes the settings body holds of a view that exists (PATCH): unlike save it never
  // creates the view, so it fails with view.not_found for one that was renamed or removed.
  async update(store: string, name: string, body: ViewSaveBody): Promise<SavedView> {
    let r: Awaited<ReturnType<typeof this.patch>>;
    try {
      r = await this.patch(store, name, body);
    } catch (e) {
      // The view is gone (another client renamed or removed it): the list drops it too.
      if (e instanceof ApiError && e.type === "view.not_found")
        this.apply(store, (vs) => vs.filter((v) => v.name !== name));
      throw e;
    }
    this.cache.noteOwnCommit(r.commit);
    this.apply(store, (vs) => [...vs.filter((v) => v.name !== name), r.view]);
    return r.view;
  }

  private patch(store: string, name: string, body: ViewSaveBody) {
    return unwrap(
      this.api.PATCH("/api/views/{name}", { params: { path: { name }, query: { store } }, body }),
    );
  }

  async rename(store: string, name: string, to: string): Promise<SavedView> {
    const r = await unwrap(
      this.api.PATCH("/api/views/{name}", {
        params: { path: { name }, query: { store } },
        body: { name: to },
      }),
    );
    this.cache.noteOwnCommit(r.commit);
    this.apply(store, (vs) => [...vs.filter((v) => v.name !== name && v.name !== to), r.view]);
    return r.view;
  }

  async remove(store: string, name: string): Promise<void> {
    const r = await unwrap(
      this.api.DELETE("/api/views/{name}", { params: { path: { name }, query: { store } } }),
    );
    this.cache.noteOwnCommit(r.commit);
    this.apply(store, (vs) => vs.filter((v) => v.name !== name));
  }

  // onStores follows the stores: it fetches the views of one that came, drops those of one that went,
  // and builds the list again (the colour of a store may have changed).
  private onStores(): void {
    const names = this.dir.stores().map((s) => s.name);
    if (names.join("\0") !== this.names.join("\0")) {
      const old = this.names;
      this.names = names;
      for (const n of old.filter((o) => !names.includes(o))) {
        this.byStore.delete(n);
        this.fetched.delete(n);
        this.unreadable.delete(n);
        // seqs keeps its counter: a fetch still on its way must not match a later one.
      }
      for (const n of names.filter((x) => !old.includes(x))) void this.fetch(n);
    }
    this.rebuild();
  }

  // fetch reads the views of a store. A fetch that a newer one or a write has overtaken is dropped; one
  // that fails leaves the views the store had.
  private async fetch(store: string): Promise<void> {
    const seq = this.bump(store);
    let views: SavedView[] | null = null;
    try {
      views = (await unwrap(this.api.GET("/api/views", { params: { query: { store } } }))) ?? [];
    } catch {
      views = null;
    }
    if (this.seqs.get(store) !== seq || !this.names.includes(store)) return;
    if (views) this.byStore.set(store, [...views].sort(byName));
    if (views) this.unreadable.delete(store);
    else this.unreadable.add(store);
    this.fetched.add(store);
    this.rebuild();
  }

  private bump(store: string): number {
    const seq = (this.seqs.get(store) ?? 0) + 1;
    this.seqs.set(store, seq);
    return seq;
  }

  // apply takes in the answer of a write, and drops a fetch that is on its way: it may have read the
  // store before the write, and would take the written view away for a moment. That fetch may also
  // hold a change of another client, so the store is read again after the write.
  private apply(store: string, change: (vs: SavedView[]) => SavedView[]): void {
    this.bump(store);
    this.byStore.set(store, change(this.byStore.get(store) ?? []).sort(byName));
    this.fetched.add(store);
    this.rebuild();
    void this.fetch(store);
  }

  // rebuild makes the list again, keeping the entries that did not change as they were, and tells the
  // subscribers if the list or loaded() is not the same.
  private rebuild(): void {
    const next: ViewEntry[] = [];
    for (const store of this.names) {
      const tone = this.dir.tone(store);
      for (const view of this.byStore.get(store) ?? []) {
        const entry: ViewEntry = { store, tone, view };
        const prev = this.result.find((e) => e.store === store && e.view.name === view.name);
        next.push(prev && sameEntry(prev, entry) ? prev : entry);
      }
    }
    const loaded = this.dir.loaded() && this.names.every((n) => this.fetched.has(n));
    const same = next.length === this.result.length && next.every((e, i) => e === this.result[i]);
    const failedKey = [...this.unreadable].sort().join("\0");
    if (same && loaded === this.isLoaded && failedKey === this.failedKey) return;
    this.failedKey = failedKey;
    if (!same) this.result = next;
    this.isLoaded = loaded;
    callAll(this.subs);
  }
}
