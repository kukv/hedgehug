// Fakes for the tests of core: an API over a function, a Connection that
// the test drives, and tasks to put in answers.
import { type Api, createApi, type ShowView, type TaskView } from "./api/client";
import type { Connection, SyncStatus } from "./connection";

export function tv(ulid: string, title = ulid, extra: Partial<TaskView> = {}): TaskView {
  return {
    ulid,
    id: `T-${ulid}`,
    store: "shared",
    workspace: { ulid: "W1", name: "main", id_prefix: "T" },
    kind: "task",
    title,
    parent: null,
    rank: "a",
    status: "todo",
    assignees: [],
    labels: [],
    blocked_by: [],
    props: {},
    created: "2026-11-01T00:00:00Z",
    updated: "2026-11-01T00:00:00Z",
    deleted_at: null,
    problems: [],
    ...extra,
  };
}

export function showOf(t: TaskView, extra: Partial<ShowView> = {}): ShowView {
  return {
    ...t,
    body: "",
    watchers: [],
    repos: [],
    comments: [],
    reverse: {},
    referenced_by: [],
    history: [],
    ...extra,
  };
}

export type Req = { method: string; path: string; query: URLSearchParams; body: unknown };

// problem is the answer of the server to a request it refuses.
export function problem(status: number, type: string, args: Record<string, unknown> = {}) {
  return new Response(JSON.stringify({ type, title: "x", status, detail: type, args }), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

// fakeApi is an Api over route: an object it returns is a 200 answer in
// JSON, a Response is sent as it is.
export function fakeApi(route: (r: Req) => unknown): { api: Api; calls: Req[] } {
  const calls: Req[] = [];
  const f = (async (input: RequestInfo | URL) => {
    const req = input as Request;
    const url = new URL(req.url);
    let body: unknown;
    if (req.method !== "GET" && req.method !== "DELETE") body = await req.json().catch(() => null);
    const r: Req = { method: req.method, path: url.pathname, query: url.searchParams, body };
    calls.push(r);
    const out = await route(r);
    return out instanceof Response ? out : Response.json(out ?? {});
  }) as unknown as typeof fetch;
  return { api: createApi({ token: () => "tok", fetch: f }), calls };
}

type Listener = (e: never) => void;

// FakeConn stands for Connection: the test emits the events.
export class FakeConn {
  state = "open";
  lastOpenAt: number | null = 1000;
  sync = new Map<string, SyncStatus>();
  unread = 0;
  private readonly listeners = new Map<string, Set<Listener>>();
  private readonly subs = new Set<() => void>();

  on(event: string, fn: Listener): () => void {
    const set = this.listeners.get(event) ?? new Set();
    this.listeners.set(event, set);
    set.add(fn);
    return () => set.delete(fn);
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  emit(event: string, payload?: unknown): void {
    for (const fn of [...(this.listeners.get(event) ?? [])]) (fn as (e: unknown) => void)(payload);
  }

  // changeSync replaces the sync state of a store and tells the subscribers.
  changeSync(s: SyncStatus): void {
    this.sync = new Map(this.sync).set(s.store, s);
    for (const fn of [...this.subs]) fn();
  }

  // retainSync drops the sync state of the stores that are not in names.
  retainSync(names: string[]): void {
    const keep = new Set(names);
    const next = new Map([...this.sync].filter(([store]) => keep.has(store)));
    if (next.size === this.sync.size) return;
    this.sync = next;
    for (const fn of [...this.subs]) fn();
  }

  asConnection(): Connection {
    return this as unknown as Connection;
  }
}

export function changed(
  store: string,
  ulids: string[],
  commit: string,
  op = "set",
  removed: string[] = [],
) {
  return { store, ulids, removed, commit, op, author: "a" };
}

// settle lets the requests that are on their way finish.
export const settle = () => new Promise<void>((r) => setTimeout(r, 0));
