import { type Api, ApiError, type Schemas, unwrap } from "./api/client";
import type { Connection } from "./connection";
import { callAll } from "./listeners";

export type Account = Schemas["AuthStatus"];
export type AccountSource = "gh" | "env" | "keychain" | "device" | "none";

// source is where the token of an account comes from: AuthStatus.token is
// gh, env:VAR, keychain or device, and absent when the account has none.
export function source(a: Account): AccountSource {
  const t = a.token ?? "";
  if (t === "gh" || t === "keychain" || t === "device") return t;
  if (t.startsWith("env:")) return "env";
  return "none";
}

// envVar is the name of the environment variable of an account whose token
// comes from one, and null for any other account.
export function envVar(a: Account): string | null {
  return source(a) === "env" ? (a.token ?? "").slice("env:".length) : null;
}

// AccountsCache keeps GET /api/accounts. The server asks GitHub for each
// account, so nothing reads it until a screen that shows accounts calls
// load; once it was read it is read again at stores.changed and resync, and
// after an operation of ours (reload).
export class AccountsCache {
  private readonly api: Api;
  private list: Account[] = [];
  private isLoaded = false;
  private err: ApiError | null = null;
  private seq = 0;
  private loading: Promise<void> | null = null;
  private reloadQueued = false;
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection }) {
    this.api = opts.api;
    opts.conn.on("stores.changed", () => this.queueReload());
    opts.conn.on("resync", () => this.queueReload());
  }

  accounts(): Account[] {
    return this.list;
  }

  loaded(): boolean {
    return this.isLoaded;
  }

  // error is why the accounts could not be read, when they could not.
  error(): ApiError | null {
    return this.err;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  // load reads the accounts unless they are read or being read. After a
  // failure it tries again.
  load(): Promise<void> {
    if (this.isLoaded) return Promise.resolve();
    return this.loading ?? this.reload();
  }

  // reload reads the accounts now. A newer reload makes the result of an
  // older one be dropped. It does not throw: a failure is held in error().
  reload(): Promise<void> {
    const seq = ++this.seq;
    const p: Promise<void> = this.read(seq).finally(() => {
      if (this.loading === p) this.loading = null;
    });
    this.loading = p;
    return p;
  }

  private async read(seq: number): Promise<void> {
    let next: Account[];
    try {
      next = (await unwrap(this.api.GET("/api/accounts"))) ?? [];
    } catch (e) {
      if (seq !== this.seq) return;
      const err =
        e instanceof ApiError
          ? e
          : new ApiError({
              status: 0,
              type: "network",
              detail: e instanceof Error ? e.message : String(e),
            });
      const changed = this.err?.type !== err.type || this.err?.detail !== err.detail;
      this.err = err;
      if (changed) callAll(this.subs);
      return;
    }
    if (seq !== this.seq) return;
    const changed =
      !this.isLoaded || this.err !== null || JSON.stringify(next) !== JSON.stringify(this.list);
    if (changed) this.list = next;
    this.isLoaded = true;
    this.err = null;
    if (changed) callAll(this.subs);
  }

  private queueReload(): void {
    if (!this.isLoaded || this.reloadQueued) return;
    this.reloadQueued = true;
    queueMicrotask(() => {
      this.reloadQueued = false;
      void this.reload();
    });
  }
}
