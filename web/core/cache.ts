import { type Api, ApiError, type ShowView, type TaskView, unwrap } from "./api/client";
import type { Connection, TasksChanged } from "./connection";

export type LiveMark = { label: "CLI で更新 · 今" | "同期で更新 · 今"; until: number };
export type ListQuery = { q: string; tree: boolean; store?: string };
export type TreeNode = { ulid: string; children: TreeNode[] };

// How long a task stays marked as changed from outside.
const MARK_MS = 10_000;
// The ops that sync makes: a change by one of them came from a remote.
const SYNC_OPS = new Set(["merge", "repair"]);
// How many of the commits this tab made to remember.
const OWN_COMMITS = 100;
// What ShowView has on top of TaskView; the shared row of a task has none of it.
const SHOW_ONLY = [
  "children",
  "body",
  "watchers",
  "repos",
  "comments",
  "reverse",
  "referenced_by",
  "history",
] as const;

type QueryState = {
  query: ListQuery;
  refs: number;
  ulids: string[];
  roots: TreeNode[];
  cycle: string[];
  error: ApiError | null;
  loading: boolean;
  seq: number;
  released: boolean;
};

type DetailState = {
  key: string;
  refs: number;
  ulid: string | null;
  error: ApiError | null;
  seq: number;
};

type Mark = { mark: LiveMark; commit: string; timer: ReturnType<typeof setTimeout> };

function sameList<T>(a: readonly T[], b: readonly T[]): boolean {
  return a.length === b.length && a.every((x, i) => x === b[i]);
}

// reuseNodes returns prev, or the parts of it, where next says the same, so
// that a tree that did not change is the same object.
function reuseNodes(next: TreeNode[], prev: TreeNode[]): TreeNode[] {
  const before = new Map(prev.map((n) => [n.ulid, n]));
  const out = next.map((n) => {
    const p = before.get(n.ulid);
    if (!p) return n;
    const children = reuseNodes(n.children, p.children);
    return children === p.children ? p : { ulid: n.ulid, children };
  });
  return sameList(out, prev) ? prev : out;
}

function toApiError(e: unknown): ApiError {
  if (e instanceof ApiError) return e;
  return new ApiError({
    status: 0,
    type: "network",
    detail: e instanceof Error ? e.message : String(e),
  });
}

// rowOf is the row of a task: a TaskView without its children, and without
// what a ShowView adds.
function rowOf(v: object): TaskView {
  const o = { ...v } as Record<string, unknown>;
  for (const k of SHOW_ONLY) delete o[k];
  return o as TaskView;
}

// TaskCache keeps the tasks the screen shows, once each, by ULID. A list or
// a tree is a query that holds the ULIDs of its result. A task that did not
// change when it was fetched again stays the same object (===), so that the
// views that show it do not draw it again.
export class TaskCache {
  private readonly api: Api;
  private readonly now: () => number;
  private readonly rows = new Map<string, TaskView>();
  private readonly rowSigs = new Map<string, string>();
  private readonly shows = new Map<string, ShowView>();
  private readonly showSigs = new Map<string, string>();
  private readonly queries = new Map<string, QueryState>();
  private readonly details = new Map<string, DetailState>();
  private readonly marks = new Map<string, Mark>();
  private readonly own = new Set<string>();
  private readonly subs = new Set<() => void>();
  // epoch counts the writes; lastWrite holds the epoch of the latest write of
  // each task, so that a fetch sent before a write does not undo it.
  private epoch = 0;
  private readonly lastWrite = new Map<string, number>();
  private dirty = false;
  private refreshQueued = false;
  private refreshAll = false;
  private refreshUlids = new Set<string>();

  constructor(opts: { api: Api; conn: Connection; now?: () => number }) {
    this.api = opts.api;
    this.now = opts.now ?? Date.now;
    opts.conn.on("tasks.changed", (e) => this.onTasksChanged(e));
    opts.conn.on("resync", () => this.queueRefresh(null));
  }

  task(ulid: string): TaskView | undefined {
    return this.rows.get(ulid);
  }

  detail(ulid: string): ShowView | undefined {
    return this.shows.get(ulid);
  }

  live(ulid: string): LiveMark | undefined {
    return this.marks.get(ulid)?.mark;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  watchList(q: ListQuery) {
    const s = this.acquire(q);
    let done = false;
    return {
      ulids: () => s.ulids,
      error: () => s.error,
      loading: () => s.loading,
      dispose: () => {
        if (!done) {
          done = true;
          this.release(s);
        }
      },
    };
  }

  watchTree(q: ListQuery) {
    const s = this.acquire({ ...q, tree: true });
    let done = false;
    return {
      roots: () => s.roots,
      cycle: () => s.cycle,
      error: () => s.error,
      loading: () => s.loading,
      dispose: () => {
        if (!done) {
          done = true;
          this.release(s);
        }
      },
    };
  }

  watchDetail(idOrUlid: string) {
    let s = this.details.get(idOrUlid);
    if (!s) {
      s = { key: idOrUlid, refs: 0, ulid: null, error: null, seq: 0 };
      this.details.set(idOrUlid, s);
      void this.fetchDetail(s);
    }
    s.refs++;
    const st = s;
    let done = false;
    return {
      ulid: () => st.ulid,
      error: () => st.error,
      dispose: () => {
        if (done) return;
        done = true;
        if (--st.refs === 0) this.details.delete(st.key);
      },
    };
  }

  // noteOwnCommit remembers a commit this tab made, so that its
  // tasks.changed is not shown as a change from outside. The event may come
  // before the answer of the write; the marks it made are taken back.
  noteOwnCommit(commit: string | null | undefined): void {
    if (!commit) return;
    this.own.add(commit);
    if (this.own.size > OWN_COMMITS) this.own.delete(this.own.values().next().value as string);
    for (const [ulid, m] of this.marks) {
      if (m.commit === commit) this.dropMark(ulid);
    }
    this.flush();
  }

  // applyWrite takes in the answer of a write: the tasks as they are now,
  // and the ULIDs purged.
  applyWrite(r: {
    task?: TaskView;
    tasks?: TaskView[];
    ulids?: string[];
    commit?: string | null;
  }): void {
    for (const t of [...(r.task ? [r.task] : []), ...(r.tasks ?? [])]) {
      this.markWrite(t.ulid);
      const row = this.ingest(t);
      const show = this.shows.get(t.ulid);
      if (show) this.putShow({ ...show, ...row });
    }
    for (const u of r.ulids ?? []) {
      this.markWrite(u);
      this.forget(u);
    }
    this.noteOwnCommit(r.commit);
    this.flush();
  }

  // setBody puts the text a write sent into the detail: the answer of a write
  // is a row, which has no body.
  setBody(ulid: string, body: string): void {
    const show = this.shows.get(ulid);
    if (!show) return;
    this.markWrite(ulid);
    this.putShow({ ...show, body });
    this.flush();
  }

  private markWrite(ulid: string): void {
    this.lastWrite.set(ulid, ++this.epoch);
  }

  private wroteSince(ulid: string, since: number): boolean {
    return (this.lastWrite.get(ulid) ?? 0) > since;
  }

  // ---- queries

  private acquire(q: ListQuery): QueryState {
    const key = JSON.stringify([q.q, q.tree, q.store ?? null]);
    let s = this.queries.get(key);
    if (!s) {
      s = {
        query: q,
        refs: 0,
        ulids: [],
        roots: [],
        cycle: [],
        error: null,
        loading: true,
        seq: 0,
        released: false,
      };
      this.queries.set(key, s);
      void this.fetchQuery(s);
    }
    s.refs++;
    return s;
  }

  private release(s: QueryState): void {
    if (--s.refs > 0) return;
    s.released = true;
    for (const [k, v] of this.queries) if (v === s) this.queries.delete(k);
  }

  private async fetchQuery(s: QueryState): Promise<void> {
    const seq = ++s.seq;
    const since = this.epoch;
    const { q, tree, store } = s.query;
    try {
      const out = await unwrap(
        this.api.GET("/api/tasks", { params: { query: { q, tree, store } } }),
      );
      if (seq !== s.seq || s.released) return; // a newer fetch is on its way
      if (tree) {
        const roots = reuseNodes(
          (out.roots ?? []).map((t) => this.ingestTree(t, since)),
          s.roots,
        );
        if (roots !== s.roots) {
          s.roots = roots;
          this.dirty = true;
        }
        const cycle = (out.cycle ?? []).map((t) => this.ingestTree(t, since).ulid);
        if (!sameList(cycle, s.cycle)) {
          s.cycle = cycle;
          this.dirty = true;
        }
      } else {
        const ulids = (out.tasks ?? []).map((t) => this.ingest(t, since).ulid);
        if (!sameList(ulids, s.ulids)) {
          s.ulids = ulids;
          this.dirty = true;
        }
      }
      if (s.error) this.dirty = true;
      s.error = null;
    } catch (e) {
      if (seq !== s.seq || s.released) return;
      s.error = toApiError(e);
      this.dirty = true;
    }
    if (s.loading) {
      s.loading = false;
      this.dirty = true;
    }
    this.flush();
  }

  // ingestTree takes in a task and, under it, its children.
  private ingestTree(t: TaskView, since: number): TreeNode {
    const row = this.ingest(t, since);
    return { ulid: row.ulid, children: (t.children ?? []).map((c) => this.ingestTree(c, since)) };
  }

  // ingest takes in a row. since is the epoch when the fetch that brought it
  // began: a row of a task written after that is older than the one held.
  private ingest(t: TaskView | ShowView, since?: number): TaskView {
    const row = rowOf(t);
    const sig = JSON.stringify(row);
    const old = this.rows.get(row.ulid);
    if (since !== undefined && this.wroteSince(row.ulid, since)) return old ?? row;
    if (old && this.rowSigs.get(row.ulid) === sig) return old;
    this.rows.set(row.ulid, row);
    this.rowSigs.set(row.ulid, sig);
    this.dirty = true;
    return row;
  }

  // ---- details

  private async fetchDetail(s: DetailState): Promise<void> {
    const seq = ++s.seq;
    const since = this.epoch;
    try {
      const show = await unwrap(
        this.api.GET("/api/tasks/{id}", { params: { path: { id: s.key } } }),
      );
      if (seq !== s.seq) return;
      const stale = this.wroteSince(show.ulid, since);
      const gone = stale && !this.rows.has(show.ulid); // purged after the fetch began
      const row = this.ingest(show, since);
      if (!gone && (!stale || !this.shows.has(show.ulid))) {
        this.putShow(stale ? { ...show, ...row } : show);
      }
      if (s.error || s.ulid !== show.ulid) this.dirty = true;
      s.ulid = show.ulid;
      s.error = null;
    } catch (e) {
      if (seq !== s.seq) return;
      const err = toApiError(e);
      s.error = err;
      this.dirty = true;
      if (err.status === 404 && s.ulid) {
        this.shows.delete(s.ulid);
        this.showSigs.delete(s.ulid);
        s.ulid = null;
      }
    }
    this.flush();
  }

  private putShow(show: ShowView): void {
    const sig = JSON.stringify(show);
    if (this.showSigs.get(show.ulid) === sig) return;
    this.shows.set(show.ulid, show);
    this.showSigs.set(show.ulid, sig);
    this.dirty = true;
  }

  private forget(ulid: string): void {
    this.rows.delete(ulid);
    this.rowSigs.delete(ulid);
    this.shows.delete(ulid);
    this.showSigs.delete(ulid);
    this.dropMark(ulid);
    this.dirty = true;
  }

  // ---- events

  private onTasksChanged(e: TasksChanged): void {
    const removed = e.removed ?? [];
    const ulids = e.ulids ?? [];
    if (!this.own.has(e.commit)) {
      const label = SYNC_OPS.has(e.op) ? "同期で更新 · 今" : "CLI で更新 · 今";
      for (const u of ulids) this.setMark(u, label, e.commit);
    }
    for (const u of removed) {
      this.markWrite(u);
      this.forget(u);
    }
    this.queueRefresh([...ulids, ...removed]);
    this.flush();
  }

  private setMark(ulid: string, label: LiveMark["label"], commit: string): void {
    this.dropMark(ulid);
    const timer = setTimeout(() => {
      this.dropMark(ulid);
      this.flush();
    }, MARK_MS);
    this.marks.set(ulid, { mark: { label, until: this.now() + MARK_MS }, commit, timer });
    this.dirty = true;
  }

  private dropMark(ulid: string): void {
    const m = this.marks.get(ulid);
    if (!m) return;
    clearTimeout(m.timer);
    this.marks.delete(ulid);
    this.dirty = true;
  }

  // queueRefresh fetches again what is on screen: every query (a create, a
  // move or a delete changes the order), and the details of ulids; null is
  // every detail. Events that come in one turn make one round of requests.
  private queueRefresh(ulids: string[] | null): void {
    if (ulids === null) this.refreshAll = true;
    else for (const u of ulids) this.refreshUlids.add(u);
    if (this.refreshQueued) return;
    this.refreshQueued = true;
    queueMicrotask(() => {
      this.refreshQueued = false;
      const all = this.refreshAll;
      const us = this.refreshUlids;
      this.refreshAll = false;
      this.refreshUlids = new Set();
      for (const s of this.queries.values()) void this.fetchQuery(s);
      for (const s of this.details.values()) {
        if (all || us.has(s.key) || (s.ulid !== null && us.has(s.ulid))) void this.fetchDetail(s);
      }
    });
  }

  private flush(): void {
    if (!this.dirty) return;
    this.dirty = false;
    for (const fn of [...this.subs]) fn();
  }
}
