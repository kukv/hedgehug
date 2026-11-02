import { type Api, type Schemas, unwrap } from "./api/client";
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
    const next = (this.chains.get(ulid) ?? Promise.resolve()).then(() => this.send(ulid));
    this.chains.set(ulid, next);
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
    const fields = this.pending.get(ulid);
    const store = this.stores.get(ulid);
    if (!fields || fields.size === 0 || !store) return;
    this.pending.delete(ulid);
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
      this.cache.applyWrite({ task: w.task, commit: w.commit });
      this.states.set(ulid, { kind: "saved", pendingPush: false });
    } catch (e) {
      // Keep the edits for the retry, unless the field was edited again.
      const again = this.pending.get(ulid) ?? new Map<PatchField, unknown>();
      for (const [k, v] of fields) if (!again.has(k)) again.set(k, v);
      this.pending.set(ulid, again);
      this.states.set(ulid, { kind: "error", message: errorMessage(e) });
    }
    this.sending.delete(ulid);
    this.notify();
  }
}
