import { type Api, ApiError, type Schemas, type TaskView, unwrap } from "./api/client";
import type { TaskCache } from "./cache";
import type { Connection } from "./connection";

export type StoreTone =
  | "teal"
  | "orange"
  | "purple"
  | "sky"
  | "pink"
  | "olive"
  | "brown"
  | "readonly";

export type WorkspaceEntry = {
  store: string;
  tone: StoreTone;
  ulid: string;
  name: string;
  prefix: string;
  projects: TaskView[];
};

// The colours of the stores, in the order of GET /stores; the eighth store
// takes the first colour again. Grey (readonly) is for a store that is
// read-only.
const TONES: StoreTone[] = ["teal", "orange", "purple", "sky", "pink", "olive", "brown"];

const byRank = (a: TaskView, b: TaskView): number =>
  a.rank < b.rank ? -1 : a.rank > b.rank ? 1 : a.ulid < b.ulid ? -1 : a.ulid > b.ulid ? 1 : 0;

function sameEntry(a: WorkspaceEntry, b: WorkspaceEntry): boolean {
  return (
    a.store === b.store &&
    a.tone === b.tone &&
    a.ulid === b.ulid &&
    a.name === b.name &&
    a.prefix === b.prefix &&
    a.projects.length === b.projects.length &&
    a.projects.every((p, i) => p === b.projects[i])
  );
}

// StoreDirectory is what the sidebar and the dialogs list: the stores in
// the order of config.yaml, the workspaces of each, and the projects of each
// workspace. The projects come from the task cache (so they follow
// tasks.changed); the stores and workspaces are fetched again at resync and
// at tasks.changed, since a workspace is added by a commit too.
export class StoreDirectory {
  private readonly api: Api;
  private readonly conn: Connection;
  private readonly cache: TaskCache;
  private items: Schemas["StoreItem"][] = [];
  private spaces = new Map<string, Schemas["WorkspaceView"][]>();
  private isLoaded = false;
  private err: ApiError | null = null;
  private seq = 0;
  private reloadQueued = false;
  private result: WorkspaceEntry[] = [];
  private readOnlySig = "";
  private readonly projects: { ulids: () => string[] };
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection; cache: TaskCache }) {
    this.api = opts.api;
    this.conn = opts.conn;
    this.cache = opts.cache;
    this.projects = opts.cache.watchList({ q: "kind:project", tree: false });
    opts.cache.subscribe(() => this.update());
    opts.conn.subscribe(() => {
      const sig = this.readOnlyNow();
      if (sig !== this.readOnlySig) this.update();
    });
    // Before the first hello the constructor's reload is the first one: the
    // resync of that hello does not reload again.
    let first = opts.conn.lastOpenAt === null;
    opts.conn.on("resync", () => {
      if (first) {
        first = false;
        return;
      }
      this.queueReload();
    });
    opts.conn.on("tasks.changed", () => this.queueReload());
    void this.reload().catch(() => undefined);
  }

  // queueReload reloads once for the events that come in one turn.
  private queueReload(): void {
    if (this.reloadQueued) return;
    this.reloadQueued = true;
    queueMicrotask(() => {
      this.reloadQueued = false;
      void this.reload().catch(() => undefined);
    });
  }

  stores(): Schemas["StoreItem"][] {
    return this.items;
  }

  // tone is the colour of a store; a store that is read-only (its sync state
  // says why) and one that is not known are grey.
  tone(store: string): StoreTone {
    const i = this.items.findIndex((s) => s.name === store);
    if (i < 0 || this.conn.sync.get(store)?.read_only) return "readonly";
    return TONES[i % TONES.length] ?? "readonly";
  }

  // workspaces lists the workspaces of every store: in the order of the
  // stores, then by name.
  workspaces(): WorkspaceEntry[] {
    return this.result;
  }

  workspace(store: string, ulid: string): WorkspaceEntry | undefined {
    return this.result.find((w) => w.store === store && w.ulid === ulid);
  }

  loaded(): boolean {
    return this.isLoaded;
  }

  // error is why the stores could not be listed, when they could not.
  error(): ApiError | null {
    return this.err;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  // reload fetches the stores and the workspaces of each again. A store whose
  // workspaces cannot be read keeps the ones it had.
  async reload(): Promise<void> {
    const seq = ++this.seq;
    let items: Schemas["StoreItem"][];
    try {
      items = (await unwrap(this.api.GET("/api/stores"))) ?? [];
    } catch (e) {
      if (seq === this.seq) {
        this.err =
          e instanceof ApiError
            ? e
            : new ApiError({
                status: 0,
                type: "network",
                detail: e instanceof Error ? e.message : String(e),
              });
        this.update(true);
      }
      throw e;
    }
    const got = await Promise.all(
      items.map(async (s): Promise<[string, Schemas["WorkspaceView"][]]> => {
        try {
          const ws = await unwrap(
            this.api.GET("/api/workspaces", { params: { query: { store: s.name } } }),
          );
          return [s.name, ws ?? []];
        } catch {
          return [s.name, this.spaces.get(s.name) ?? []];
        }
      }),
    );
    if (seq !== this.seq) return; // a newer reload is on its way
    // last_success changes at each sync and nothing here shows it (the sync
    // card reads sync.state); the list stays as it was when nothing else changed
    const sig = (xs: Schemas["StoreItem"][]) =>
      JSON.stringify(xs.map(({ last_success: _, ...rest }) => rest));
    const same = sig(items) === sig(this.items);
    const spaces = new Map(got);
    const changed =
      !this.isLoaded ||
      this.err !== null ||
      !same ||
      JSON.stringify([...spaces]) !== JSON.stringify([...this.spaces]);
    if (!same) this.items = items;
    this.spaces = spaces;
    this.isLoaded = true;
    this.err = null;
    this.update(changed);
  }

  private readOnlyNow(): string {
    return this.items
      .filter((s) => this.conn.sync.get(s.name)?.read_only)
      .map((s) => s.name)
      .join("\0");
  }

  // update builds the list again, keeping the entries that did not change
  // as they were, and tells the subscribers if the list is not the same.
  private update(force = false): void {
    this.readOnlySig = this.readOnlyNow();
    const projects = this.projects
      .ulids()
      .map((u) => this.cache.task(u))
      .filter((t): t is TaskView => !!t)
      .sort(byRank);
    const next: WorkspaceEntry[] = [];
    for (const s of this.items) {
      const ws = [...(this.spaces.get(s.name) ?? [])].sort((a, b) =>
        a.name.localeCompare(b.name, "ja"),
      );
      for (const w of ws) {
        const entry: WorkspaceEntry = {
          store: s.name,
          tone: this.tone(s.name),
          ulid: w.ulid,
          name: w.name,
          prefix: w.id_prefix,
          projects: projects.filter((p) => p.store === s.name && p.workspace.ulid === w.ulid),
        };
        const prev = this.result.find((e) => e.store === entry.store && e.ulid === entry.ulid);
        next.push(prev && sameEntry(prev, entry) ? prev : entry);
      }
    }
    const same = next.length === this.result.length && next.every((e, i) => e === this.result[i]);
    if (!same) this.result = next;
    if (!same || force) for (const fn of [...this.subs]) fn();
  }
}
