import type { Api, Schemas } from "./api/client";
import { parseEventStream } from "./sse";

export type ConnState = "connecting" | "open" | "reconnecting" | "down";
export type HelloEvent = Schemas["HelloEvent"];
export type TasksChanged = Schemas["TasksChanged"];
export type SyncStatus = Schemas["SyncStatus"];

// Seconds between the end of a connection (or a failed try) and the next
// try: 1, 2, 4, then 5 for ever.
const BACKOFF_MS = [1000, 2000, 4000, 5000];

type Handlers = {
  "tasks.changed": (e: TasksChanged) => void;
  "schema.changed": (e: { store: string }) => void;
  "views.changed": (e: { store: string }) => void;
  resync: () => void;
};

// Connection reads /api/events. It keeps the state of the connection, the
// sync state of each store and the unread count, and tells the caches when
// to fetch again (resync): at the first hello, and when a store's HEAD
// moved while the connection was down.
export class Connection {
  private readonly opts: {
    token: () => string | null;
    fetch: typeof fetch;
    api: Api;
    now: () => number;
    sleep: (ms: number) => Promise<void>;
    doc: Document | undefined;
  };
  private _state: ConnState = "connecting";
  private _lastOpenAt: number | null = null;
  private _unread = 0;
  private _sync = new Map<string, SyncStatus>();
  private heads = new Map<string, string>();
  private everOpened = false;
  private clientId: string | null = null;
  private running: object | null = null;
  private abort: AbortController | null = null;
  private readonly subs = new Set<() => void>();
  private readonly handlers: { [K in keyof Handlers]: Set<Handlers[K]> } = {
    "tasks.changed": new Set(),
    "schema.changed": new Set(),
    "views.changed": new Set(),
    resync: new Set(),
  };

  constructor(opts: {
    token: () => string | null;
    fetch?: typeof fetch;
    api: Api;
    now?: () => number;
    sleep?: (ms: number) => Promise<void>;
    doc?: Document;
  }) {
    this.opts = {
      token: opts.token,
      fetch: opts.fetch ?? ((input, init) => globalThis.fetch(input, init)),
      api: opts.api,
      now: opts.now ?? Date.now,
      sleep: opts.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms))),
      doc: opts.doc ?? (typeof document !== "undefined" ? document : undefined),
    };
  }

  get state(): ConnState {
    return this._state;
  }
  get lastOpenAt(): number | null {
    return this._lastOpenAt;
  }
  get sync(): Map<string, SyncStatus> {
    return this._sync;
  }
  get unread(): number {
    return this._unread;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  on(event: "tasks.changed", fn: (e: TasksChanged) => void): () => void;
  on(event: "schema.changed" | "views.changed", fn: (e: { store: string }) => void): () => void;
  on(event: "resync", fn: () => void): () => void;
  on(event: keyof Handlers, fn: (e: never) => void): () => void {
    const set = this.handlers[event] as Set<(e: never) => void>;
    set.add(fn);
    return () => set.delete(fn);
  }

  start(): void {
    if (this.running) return;
    const run = {};
    this.running = run;
    this.opts.doc?.addEventListener("visibilitychange", this.onVisibility);
    void this.loop(run);
  }

  stop(): void {
    this.running = null;
    this.abort?.abort();
    this.opts.doc?.removeEventListener("visibilitychange", this.onVisibility);
  }

  private notify(): void {
    for (const fn of [...this.subs]) this.guard(fn);
  }

  // guard runs one listener. A listener that throws must not break the
  // connection or skip the others, so its error is only reported.
  private guard(fn: () => void): void {
    try {
      fn();
    } catch (e) {
      console.error(e);
    }
  }

  private setState(s: ConnState): void {
    if (this._state === s) return;
    this._state = s;
    this.notify();
  }

  private emit<K extends keyof Handlers>(event: K, ...args: Parameters<Handlers[K]>): void {
    for (const fn of [...this.handlers[event]] as ((...a: unknown[]) => void)[]) {
      this.guard(() => fn(...args));
    }
  }

  // loop connects, reads until the stream ends, and tries again after a
  // pause. A stream that ended after its hello starts the pauses over.
  private async loop(run: object): Promise<void> {
    let failures = 0;
    while (this.running === run) {
      const opened = await this.connectOnce(run);
      if (this.running !== run) return;
      if (opened) {
        failures = 0;
        this.setState("reconnecting");
      } else {
        this.setState("down");
      }
      await this.opts.sleep(BACKOFF_MS[Math.min(failures, BACKOFF_MS.length - 1)] ?? 5000);
      failures++;
    }
  }

  // connectOnce reads one connection to its end. It returns whether the
  // hello came.
  private async connectOnce(run: object): Promise<boolean> {
    const ctl = new AbortController();
    this.abort = ctl;
    let opened = false;
    try {
      const headers: Record<string, string> = { Accept: "text/event-stream" };
      const t = this.opts.token();
      if (t) headers.Authorization = `Bearer ${t}`;
      const res = await this.opts.fetch("/api/events", { headers, signal: ctl.signal });
      if (!res.ok || !res.body) {
        await res.body?.cancel();
        return false;
      }
      const feed = parseEventStream((m) => {
        if (this.running !== run) return;
        if (m.event === "hello") opened = true;
        this.dispatch(m.event, m.data);
      });
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        feed(decoder.decode(value, { stream: true }));
      }
    } catch {
      // A broken connection is the same as one that ended.
    }
    return opened;
  }

  private dispatch(event: string, data: string): void {
    let body: unknown;
    try {
      body = JSON.parse(data);
    } catch {
      return;
    }
    switch (event) {
      case "hello":
        this.onHello(body as HelloEvent);
        break;
      case "tasks.changed": {
        const e = body as TasksChanged;
        this.heads.set(e.store, e.commit);
        this.emit("tasks.changed", { ...e, ulids: e.ulids ?? [], removed: e.removed ?? [] });
        break;
      }
      case "sync.state": {
        const s = body as SyncStatus;
        this._sync = new Map(this._sync).set(s.store, s);
        this.notify();
        break;
      }
      case "notifications.unread":
        this._unread = (body as { count: number }).count;
        this.notify();
        break;
      case "schema.changed":
        this.emit("schema.changed", body as { store: string });
        break;
      case "views.changed":
        this.emit("views.changed", body as { store: string });
        break;
    }
  }

  private onHello(h: HelloEvent): void {
    const stores = h.stores ?? [];
    const heads = new Map(stores.map((s) => [s.store, s.head] as const));
    const moved =
      !this.everOpened ||
      heads.size !== this.heads.size ||
      [...heads].some(([store, head]) => this.heads.get(store) !== head);
    this.heads = heads;
    this._sync = new Map(stores.map((s) => [s.store, s.sync] as const));
    this._unread = h.unread;
    this.clientId = h.client;
    this.everOpened = true;
    this._lastOpenAt = this.opts.now();
    this._state = "open";
    this.notify();
    void this.sendPresence();
    if (moved) this.emit("resync");
  }

  private onVisibility = (): void => {
    void this.sendPresence();
  };

  // sendPresence tells the server whether the page is on screen, for its
  // sync loop. It is sent after each hello too: the server counts a new
  // connection as visible, and the page may be hidden by then.
  private async sendPresence(): Promise<void> {
    if (!this.clientId) return;
    const visible = (this.opts.doc?.visibilityState ?? "visible") === "visible";
    try {
      await this.opts.api.PUT("/api/presence", { body: { client: this.clientId, visible } });
    } catch {
      // presence is a hint; a failure changes nothing for the user
    }
  }
}
