import { type Api, unwrap } from "./api/client";
import type { Connection } from "./connection";
import { callAll } from "./listeners";

// PeopleCache keeps the names that @ offers, by store: the people on the
// tasks of the store and the authors of its commits (GET /api/people). A
// change in the store (tasks.changed: a comment or an assignee may be new)
// makes the names stale; the next load fetches them again.
export class PeopleCache {
  private readonly api: Api;
  private readonly names = new Map<string, string[]>();
  private readonly stale = new Set<string>();
  private readonly loading = new Map<string, { p: Promise<string[]>; version: number }>();
  private readonly versions = new Map<string, number>();
  private readonly seqs = new Map<string, number>();
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection }) {
    this.api = opts.api;
    opts.conn.on("tasks.changed", (e) => this.touch(e.store));
    opts.conn.on("resync", () => {
      for (const store of this.names.keys()) this.touch(store);
    });
  }

  // touch makes the names of the store stale, and a fetch on its way is no
  // longer an answer to the next load (it may have read the older state).
  private touch(store: string): void {
    this.stale.add(store);
    this.versions.set(store, (this.versions.get(store) ?? 0) + 1);
  }

  // get is the names last read, which may be stale.
  get(store: string): string[] | undefined {
    return this.names.get(store);
  }

  // load is the names of the store: the ones held while they are fresh,
  // else the ones fetched.
  load(store: string): Promise<string[]> {
    const have = this.names.get(store);
    if (have && !this.stale.has(store)) return Promise.resolve(have);
    const version = this.versions.get(store) ?? 0;
    const running = this.loading.get(store);
    if (running && running.version === version) return running.p;
    const seq = (this.seqs.get(store) ?? 0) + 1;
    this.seqs.set(store, seq);
    const p = unwrap(this.api.GET("/api/people", { params: { query: { store } } }))
      .then((list) => {
        const next = list ?? [];
        // An older fetch that ends after a newer one has started gives way to it.
        if (this.seqs.get(store) !== seq) {
          return this.loading.get(store)?.p ?? this.names.get(store) ?? next;
        }
        // A change that came while this was on its way leaves the store stale.
        if (this.versions.get(store) === version) this.stale.delete(store);
        const old = this.names.get(store);
        if (old && old.length === next.length && old.every((n, i) => n === next[i])) return old;
        this.names.set(store, next);
        callAll(this.subs);
        return next;
      })
      .finally(() => {
        if (this.loading.get(store)?.p === p) this.loading.delete(store);
      });
    this.loading.set(store, { p, version });
    return p;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }
}

// matchPeople is the names that start with, or (after the first) contain,
// what was typed after @, case ignored; the ones that start with it come
// first.
export function matchPeople(names: readonly string[], typed: string, limit = 8): string[] {
  const t = typed.toLowerCase();
  const starts: string[] = [];
  const contains: string[] = [];
  for (const n of names) {
    const i = n.toLowerCase().indexOf(t);
    if (i === 0) starts.push(n);
    else if (i > 0) contains.push(n);
  }
  return [...starts, ...contains].slice(0, limit);
}
