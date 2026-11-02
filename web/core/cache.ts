import { type Api, ApiError, type ShowView, type TaskView, unwrap } from "./api/client";
import type { Connection, TasksChanged } from "./connection";
import { callAll } from "./listeners";

export type LiveMark = { label: "CLI で更新 · 今" | "同期で更新 · 今"; until: number };
export type ListQuery = { q: string; tree: boolean; store?: string };
// context is true for a task the tree holds only as the ancestor of a task the
// query found. It is a mark of this tree, not of the task: another query may
// find the same task, so the row does not keep it.
export type TreeNode = { ulid: string; children: TreeNode[]; context?: boolean };

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
  // the latest fetch (null: its answer is held; else why not): a fetch that a newer one overtook waits for it
  last: Promise<ApiError | null>;
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
    if (children === p.children && p.context === n.context) return p;
    return n.context ? { ulid: n.ulid, children, context: true } : { ulid: n.ulid, children };
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

// rowOf is the row of a task: a TaskView without its children, without
// what a ShowView adds, and without context (that belongs to the tree).
function rowOf(v: object): TaskView {
  const o = { ...v } as Record<string, unknown>;
  for (const k of SHOW_ONLY) delete o[k];
  delete o.context;
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
  // lastChange holds the epoch of the latest change an event brought (a change from outside, not
  // a write of this tab): a read that began before it is older than what the event started to fetch.
  private readonly lastChange = new Map<string, number>();
  // resyncAt is the epoch of the last resync: what changed while the connection was lost is not in lastChange.
  private resyncAt = 0;
  // rowSeqs holds the latest fetch (a number from rowSeq) of a row nobody holds, so that an answer a newer one
  // overtook is dropped. An entry lasts only while its fetch is on its way.
  private rowSeq = 0;
  private readonly rowSeqs = new Map<string, number>();
  private dirty = false;
  private refreshQueued = false;
  private refreshAll = false;
  private refreshUlids = new Set<string>();

  constructor(opts: { api: Api; conn: Connection; now?: () => number }) {
    this.api = opts.api;
    this.now = opts.now ?? Date.now;
    opts.conn.on("tasks.changed", (e) => this.onTasksChanged(e));
    opts.conn.on("resync", () => {
      this.resyncAt = ++this.epoch;
      this.queueRefresh(null);
    });
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
    const s = this.acquire({ ...q, tree: false });
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

  // epochNow is the point a read begins at: pass it to prime, taken before the request is sent.
  epochNow(): number {
    return this.epoch;
  }

  // prime takes in tasks a read brought (a search, a lookup): not a write, so a fetch that is on
  // its way is not held back by it. since is the epochNow of before the read was sent: a task
  // written or changed by an event after that is newer than what the read brought.
  prime(tasks: (TaskView | ShowView)[], since: number): void {
    for (const t of tasks) {
      if (!this.changedSince(t.ulid, since)) this.ingest(t, since);
      // Nothing fetches a row nobody holds when its event came: the read has the old row, so fetch it now.
      else if (!this.rows.has(t.ulid)) void this.fetchRow(t.ulid, t.store);
    }
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

  private changedSince(ulid: string, since: number): boolean {
    return Math.max(this.lastChange.get(ulid) ?? 0, this.resyncAt) > since;
  }

  // refreshTrees fetches the trees on screen again and resolves once they hold the answer, so
  // that a caller that has just written can decide on the order the write left. It rejects with
  // the error of a tree that could not be read: the order held is then the old one.
  async refreshTrees(): Promise<void> {
    const trees = [...this.queries.values()].filter((s) => s.query.tree);
    const errors = await Promise.all(trees.map((s) => this.fetchQuery(s)));
    const failed = errors.find((e) => e !== null);
    if (failed) throw failed;
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
        last: Promise.resolve(null),
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

  private fetchQuery(s: QueryState): Promise<ApiError | null> {
    const p = this.runFetch(s);
    s.last = p;
    return p;
  }

  private async runFetch(s: QueryState): Promise<ApiError | null> {
    const seq = ++s.seq;
    let failed: ApiError | null = null;
    const since = this.epoch;
    const { q, tree, store } = s.query;
    try {
      const out = await unwrap(
        this.api.GET("/api/tasks", { params: { query: { q, tree, store } } }),
      );
      if (seq !== s.seq || s.released) return seq !== s.seq ? s.last : null; // a newer fetch is on its way
      if (tree) {
        const roots = reuseNodes(this.ingestTrees(out.roots ?? [], since), s.roots);
        if (roots !== s.roots) {
          s.roots = roots;
          this.dirty = true;
        }
        const cycle = this.ingestTrees(out.cycle ?? [], since).map((n) => n.ulid);
        if (!sameList(cycle, s.cycle)) {
          s.cycle = cycle;
          this.dirty = true;
        }
      } else {
        const ulids = (out.tasks ?? [])
          .filter((t) => !this.gone(t.ulid, since))
          .map((t) => this.ingest(t, since).ulid);
        if (!sameList(ulids, s.ulids)) {
          s.ulids = ulids;
          this.dirty = true;
        }
      }
      if (s.error) this.dirty = true;
      s.error = null;
    } catch (e) {
      if (seq !== s.seq || s.released) return seq !== s.seq ? s.last : null;
      s.error = toApiError(e);
      failed = s.error;
      this.dirty = true;
    }
    if (s.loading) {
      s.loading = false;
      this.dirty = true;
    }
    this.flush();
    return failed;
  }

  // ingestTree takes in a task and, under it, its children.
  private ingestTrees(ts: TaskView[], since: number): TreeNode[] {
    return ts
      .filter((t) => !this.gone(t.ulid, since))
      .map((t) => {
        const node: TreeNode = {
          ulid: this.ingest(t, since).ulid,
          children: this.ingestTrees(t.children ?? [], since),
        };
        if (t.context) node.context = true;
        return node;
      });
  }

  // gone is whether a task was purged after the fetch began: the answer is
  // older than that, so it does not list the task.
  private gone(ulid: string, since: number): boolean {
    return this.wroteSince(ulid, since) && !this.rows.has(ulid);
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
      const gone = this.gone(show.ulid, since);
      const row = this.ingest(show, since);
      if (!gone && (!stale || !this.shows.has(show.ulid))) {
        this.putShow(stale ? { ...show, ...row } : show);
      }
      // The answer is older than a write (an event came before the answer of
      // the write): what only a ShowView has is fetched again after the write.
      if (stale && !gone) this.queueRefresh([show.ulid]);
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
    this.rowSeqs.delete(ulid);
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
    for (const u of [...ulids, ...removed]) this.lastChange.set(u, ++this.epoch);
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
      // A row no query or detail holds (the blocker of a task, a crumb) has nobody to fetch it again.
      // After a resync that is every row it holds: the events of the time it was not connected are lost.
      const held = this.heldUlids();
      for (const u of all ? [...this.rows.keys()] : us) {
        if (this.rows.has(u) && !held.has(u)) void this.fetchRow(u);
      }
    });
  }

  private heldUlids(): Set<string> {
    const held = new Set<string>();
    const walk = (ns: TreeNode[]) => {
      for (const n of ns) {
        held.add(n.ulid);
        walk(n.children);
      }
    };
    for (const s of this.queries.values()) {
      for (const u of s.ulids) held.add(u);
      walk(s.roots);
      for (const u of s.cycle) held.add(u);
    }
    for (const s of this.details.values()) if (s.ulid) held.add(s.ulid);
    return held;
  }

  private async fetchRow(ulid: string, store?: string): Promise<void> {
    const since = this.epoch;
    const seq = ++this.rowSeq;
    this.rowSeqs.set(ulid, seq);
    store ??= this.rows.get(ulid)?.store;
    try {
      const show = await unwrap(
        this.api.GET("/api/tasks/{id}", { params: { path: { id: ulid }, query: { store } } }),
      );
      if (seq !== this.rowSeqs.get(ulid)) return; // a newer fetch of this row is on its way
      this.rowSeqs.delete(ulid);
      if (!this.gone(ulid, since) && !this.changedSince(ulid, since)) this.ingest(show, since);
    } catch (e) {
      if (seq !== this.rowSeqs.get(ulid)) return;
      this.rowSeqs.delete(ulid);
      if (e instanceof ApiError && e.status === 404 && !this.wroteSince(ulid, since))
        this.forget(ulid);
    }
    this.flush();
  }

  private flush(): void {
    if (!this.dirty) return;
    this.dirty = false;
    callAll(this.subs);
  }
}
