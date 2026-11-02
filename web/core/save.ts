import { type Api, ApiError, type Schemas, unwrap } from "./api/client";
import type { TaskCache } from "./cache";
import type { Connection } from "./connection";
import { errorMessage } from "./messages";

export type PatchField =
  | "title"
  | "kind"
  | "status"
  | "assignees"
  | "watchers"
  | "labels"
  | "repos"
  | "blocked_by"
  | "body"
  | `props.${string}`;

export type SaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "saved"; pendingPush: boolean }
  | { kind: "error"; message: string };

type Patch = Schemas["PatchInputBody"];

const DELAY_MS = 2000;

// refused is whether the server turned the request down for good: a client
// error that sending the same again does not change (408, 409 and 429 do).
function refused(e: unknown): boolean {
  return (
    e instanceof ApiError &&
    e.status >= 400 &&
    e.status < 500 &&
    ![408, 409, 429].includes(e.status)
  );
}

function safeMessage(e: unknown): string {
  try {
    return errorMessage(e);
  } catch {
    return String(e);
  }
}

// patchOf makes the body of one PATCH from the fields edited: props.<name>
// goes into props, the others are fields of the body.
function patchOf(fields: Map<PatchField, unknown>): Patch {
  const body: Record<string, unknown> = {};
  const props: Record<string, unknown> = {};
  for (const [field, value] of fields) {
    if (field.startsWith("props.")) props[field.slice("props.".length)] = value;
    else body[field] = value;
  }
  if (Object.keys(props).length > 0) body.props = props;
  return body as Patch;
}

// SaveController sends the edits of a task as one PATCH (one commit): when
// the input stopped for two seconds, or at flush (the field lost focus, the
// screen is left). It keeps the state of the save for each task.
export class SaveController {
  private readonly api: Api;
  private readonly cache: TaskCache;
  private readonly conn: Connection;
  private readonly delayMs: number;
  private readonly setTimer: (fn: () => void, ms: number) => unknown;
  private readonly clearTimer: (t: unknown) => void;
  // The fields edited and not sent yet, by task.
  private readonly pending = new Map<string, Map<PatchField, unknown>>();
  // The fields of a PATCH on its way, by task.
  private readonly sending = new Map<string, Set<PatchField>>();
  // The fields the server refused, by task, with the words of the error:
  // they stay in pending, but are sent again only when edited again or when
  // nothing else is waiting (a retry).
  private readonly rejected = new Map<string, Map<PatchField, string>>();
  private readonly timers = new Map<string, unknown>();
  private readonly stores = new Map<string, string>();
  private readonly states = new Map<string, SaveState>();
  private readonly chains = new Map<string, Promise<void>>();
  private readonly subs = new Set<() => void>();

  constructor(opts: {
    api: Api;
    cache: TaskCache;
    conn: Connection;
    delayMs?: number;
    setTimer?: typeof setTimeout;
    clearTimer?: typeof clearTimeout;
  }) {
    this.api = opts.api;
    this.cache = opts.cache;
    this.conn = opts.conn;
    this.delayMs = opts.delayMs ?? DELAY_MS;
    this.setTimer = (fn, ms) => (opts.setTimer ?? setTimeout)(fn, ms);
    this.clearTimer = (t) => (opts.clearTimer ?? clearTimeout)(t as never);
  }

  // edit takes the value of a field and starts the wait again. The latest
  // value of a field wins.
  edit(ulid: string, store: string, field: PatchField, value: unknown): void {
    let fields = this.pending.get(ulid);
    if (!fields) {
      fields = new Map();
      this.pending.set(ulid, fields);
    }
    fields.set(field, value);
    this.rejected.get(ulid)?.delete(field);
    this.stores.set(ulid, store);
    const t = this.timers.get(ulid);
    if (t !== undefined) this.clearTimer(t);
    this.timers.set(
      ulid,
      this.setTimer(() => {
        this.timers.delete(ulid);
        void this.flush(ulid);
      }, this.delayMs),
    );
    this.notify();
  }

  // flush sends what is waiting for ulid now. It resolves when the save is
  // over, whether it worked or not (the state tells).
  flush(ulid: string): Promise<void> {
    const t = this.timers.get(ulid);
    if (t !== undefined) {
      this.clearTimer(t);
      this.timers.delete(ulid);
    }
    // One PATCH of a task at a time, in the order of the edits.
    const next = (this.chains.get(ulid) ?? Promise.resolve())
      .then(() => this.send(ulid))
      .catch((e) => {
        this.states.set(ulid, { kind: "error", message: safeMessage(e) });
        this.notify();
      });
    this.chains.set(ulid, next);
    void next.then(() => {
      if (this.chains.get(ulid) === next) this.chains.delete(ulid);
    });
    return next;
  }

  state(ulid: string): SaveState {
    const s = this.states.get(ulid) ?? { kind: "idle" };
    if (s.kind !== "saved") return s;
    const store = this.stores.get(ulid);
    const unpushed = store ? (this.conn.sync.get(store)?.unpushed ?? 0) : 0;
    return { kind: "saved", pendingPush: unpushed >= 1 };
  }

  // editing is whether the field has an edit that the server has not
  // answered. The screen does not overwrite its input with a change from
  // outside while it is true.
  editing(ulid: string, field: PatchField): boolean {
    return !!this.pending.get(ulid)?.has(field) || !!this.sending.get(ulid)?.has(field);
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  private notify(): void {
    for (const fn of [...this.subs]) fn();
  }

  private async send(ulid: string): Promise<void> {
    const all = this.pending.get(ulid);
    const store = this.stores.get(ulid);
    if (!all || all.size === 0 || !store) return;
    // The fields the server refused wait, unless they are all there is.
    const refusedNow = this.rejected.get(ulid);
    const fresh = new Map([...all].filter(([k]) => !refusedNow?.has(k)));
    const fields = fresh.size > 0 ? fresh : all;
    const rest = new Map([...all].filter(([k]) => !fields.has(k)));
    if (rest.size > 0) this.pending.set(ulid, rest);
    else this.pending.delete(ulid);
    this.sending.set(ulid, new Set(fields.keys()));
    this.states.set(ulid, { kind: "saving" });
    this.notify();
    try {
      const w = await unwrap(
        this.api.PATCH("/api/tasks/{id}", {
          params: { path: { id: ulid }, query: { store } },
          body: patchOf(fields),
        }),
      );
      for (const k of fields.keys()) this.rejected.get(ulid)?.delete(k);
      this.cache.applyWrite({ task: w.task, commit: w.commit });
      const body = fields.get("body");
      if (typeof body === "string") this.cache.setBody(ulid, body);
      // An edit still waiting means the task is not saved yet; a field the
      // server refused is still an error.
      const left = this.rejected.get(ulid);
      const waiting =
        [...(this.pending.get(ulid)?.keys() ?? [])].some((k) => !left?.has(k)) ||
        this.timers.has(ulid);
      const message = left?.values().next().value;
      this.states.set(
        ulid,
        waiting
          ? { kind: "saving" }
          : message !== undefined
            ? { kind: "error", message }
            : { kind: "saved", pendingPush: false },
      );
    } catch (e) {
      // Keep the edits for the retry, unless the field was edited again.
      const again = this.pending.get(ulid) ?? new Map<PatchField, unknown>();
      const kept: PatchField[] = [];
      for (const [k, v] of fields) {
        if (again.has(k)) continue;
        again.set(k, v);
        kept.push(k);
      }
      this.pending.set(ulid, again);
      const message = safeMessage(e);
      if (refused(e)) {
        const left = this.rejected.get(ulid) ?? new Map<PatchField, string>();
        for (const k of kept) left.set(k, message);
        this.rejected.set(ulid, left);
      }
      this.states.set(ulid, { kind: "error", message });
    } finally {
      this.sending.delete(ulid);
      this.notify();
    }
  }
}
