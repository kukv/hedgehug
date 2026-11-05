import { afterEach, describe, expect, it, vi } from "vitest";
import { createApi } from "./api/client";
import { Connection } from "./connection";

const enc = new TextEncoder();

function sseStream() {
  let c!: ReadableStreamDefaultController<Uint8Array>;
  const body = new ReadableStream<Uint8Array>({
    start(x) {
      c = x;
    },
  });
  return {
    response: new Response(body, { headers: { "Content-Type": "text/event-stream" } }),
    send: (s: string) => c.enqueue(enc.encode(s)),
    end: () => c.close(),
  };
}

const ev = (event: string, data: unknown) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
const sync = (store: string, unpushed = 0) => ({ store, remotes: [], unpushed });
const hello = (client: string, heads: Record<string, string>, unread = 0) =>
  ev("hello", {
    client,
    unread,
    stores: Object.entries(heads).map(([store, head]) => ({ store, head, sync: sync(store) })),
  });

type Streams = ReturnType<typeof sseStream>;

// A server for the tests: each call of /api/events takes the next entry of
// the script (a stream, or null for a refused connection).
function setup(script: (Streams | null)[]) {
  const calls: { url: string; auth: string | null }[] = [];
  const presence: { client: string; visible: boolean }[] = [];
  const f = (async (input: RequestInfo | URL, init?: RequestInit) => {
    if (typeof input === "string") {
      calls.push({ url: input, auth: new Headers(init?.headers).get("Authorization") });
      const next = script.shift();
      if (!next) throw new TypeError("Failed to fetch");
      return next.response;
    }
    const req = input as Request;
    presence.push(await req.json());
    return new Response(null, { status: 204 });
  }) as unknown as typeof fetch;
  const waits: { ms: number; resolve: () => void }[] = [];
  const slept: number[] = [];
  const sleep = (ms: number) =>
    new Promise<void>((resolve) => {
      slept.push(ms);
      waits.push({ ms, resolve });
    });
  const api = createApi({ token: () => "tok", fetch: f });
  const conn = new Connection({ token: () => "tok", fetch: f, api, sleep, now: () => 1000 });
  return {
    conn,
    calls,
    presence,
    slept,
    wake: () => waits.shift()?.resolve(),
    until: (fn: () => boolean) => vi.waitFor(() => expect(fn()).toBe(true)),
  };
}

afterEach(() => {
  Object.defineProperty(document, "visibilityState", { value: "visible", configurable: true });
});

describe("Connection", () => {
  it("opens at hello, keeps sync, unread and the clock, and asks for a resync", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const resync = vi.fn();
    t.conn.on("resync", resync);
    t.conn.start();
    expect(t.conn.state).toBe("connecting");

    s1.send(hello("c1", { shared: "h1" }, 3));
    await t.until(() => t.conn.state === "open");
    expect(t.calls[0]).toEqual({ url: "/api/events", auth: "Bearer tok" });
    expect(t.conn.unread).toBe(3);
    expect(t.conn.sync.get("shared")?.store).toBe("shared");
    expect(t.conn.lastOpenAt).toBe(1000);
    expect(resync).toHaveBeenCalledTimes(1);
    await t.until(() => t.presence.length === 1);
    expect(t.presence[0]).toEqual({ client: "c1", visible: true });
    t.conn.stop();
  });

  it("applies sync.state and notifications.unread and tells the subscribers", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const changed = vi.fn();
    t.conn.subscribe(changed);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");
    changed.mockClear();

    s1.send(ev("sync.state", sync("shared", 2)));
    await t.until(() => t.conn.sync.get("shared")?.unpushed === 2);
    s1.send(ev("notifications.unread", { count: 5 }));
    await t.until(() => t.conn.unread === 5);
    expect(changed).toHaveBeenCalledTimes(2);
    t.conn.stop();
  });

  it("carries push_at of the sync state as it is", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");
    s1.send(ev("sync.state", { ...sync("shared", 1), push_at: "2026-11-02T10:00:30Z" }));
    await t.until(() => t.conn.sync.get("shared")?.push_at === "2026-11-02T10:00:30Z");
    t.conn.stop();
  });

  it("passes tasks.changed, schema.changed, views.changed and automations.changed to the listeners", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const tasks = vi.fn();
    const schema = vi.fn();
    const views = vi.fn();
    const automations = vi.fn();
    t.conn.on("tasks.changed", tasks);
    t.conn.on("schema.changed", schema);
    t.conn.on("views.changed", views);
    t.conn.on("automations.changed", automations);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    s1.send(
      ev("tasks.changed", {
        store: "shared",
        ulids: ["U1"],
        removed: null,
        commit: "h2",
        op: "set",
        author: "a",
      }),
    );
    s1.send(ev("schema.changed", { store: "shared" }));
    s1.send(ev("views.changed", { store: "shared" }));
    s1.send(ev("automations.changed", { store: "shared" }));
    await t.until(() => automations.mock.calls.length === 1);
    expect(tasks).toHaveBeenCalledWith({
      store: "shared",
      ulids: ["U1"],
      removed: [],
      commit: "h2",
      op: "set",
      author: "a",
    });
    expect(schema).toHaveBeenCalledWith({ store: "shared" });
    t.conn.stop();
  });

  it("passes stores.changed to the listeners", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const changed = vi.fn();
    t.conn.on("stores.changed", changed);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    s1.send(ev("stores.changed", {}));
    await t.until(() => changed.mock.calls.length === 1);
    t.conn.stop();
  });

  it("retainSync drops the sync of a store that is gone, and tells the subscribers only then", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1", gone: "h2" }));
    await t.until(() => t.conn.sync.size === 2);
    const kept = t.conn.sync.get("shared");
    const notified = vi.fn();
    t.conn.subscribe(notified);

    t.conn.retainSync(["shared", "gone"]);
    expect(notified).not.toHaveBeenCalled();

    t.conn.retainSync(["shared"]);
    expect([...t.conn.sync.keys()]).toEqual(["shared"]);
    expect(t.conn.sync.get("shared")).toBe(kept);
    expect(notified).toHaveBeenCalledTimes(1);

    t.conn.retainSync(["shared"]);
    expect(notified).toHaveBeenCalledTimes(1);
    t.conn.stop();
  });

  it("passes auth.device to the listeners", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const device = vi.fn();
    t.conn.on("auth.device", device);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    s1.send(ev("auth.device", { id: "d1", state: "done", account: "bob" }));
    s1.send(ev("auth.device", { id: "d2", state: "error", code: "auth.canceled", message: "x" }));
    await t.until(() => device.mock.calls.length === 2);
    expect(device).toHaveBeenNthCalledWith(1, { id: "d1", state: "done", account: "bob" });
    expect(device).toHaveBeenNthCalledWith(2, {
      id: "d2",
      state: "error",
      code: "auth.canceled",
      message: "x",
    });
    t.conn.stop();
  });

  it("keeps the connection when a listener throws, and still calls the others", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    const second = vi.fn();
    t.conn.on("tasks.changed", () => {
      throw new Error("listener failed");
    });
    t.conn.on("tasks.changed", second);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");
    s1.send(
      ev("tasks.changed", {
        store: "shared",
        ulids: [],
        removed: [],
        commit: "h2",
        op: "",
        author: "",
      }),
    );
    await t.until(() => second.mock.calls.length === 1);
    expect(errors).toHaveBeenCalledTimes(1);
    expect(t.calls.length).toBe(1);
    expect(t.conn.state).toBe("open");
    errors.mockRestore();
    t.conn.stop();
  });

  it("reconnects after 1, 2, 4, 5 and 5 seconds and shows the state meanwhile", async () => {
    const s1 = sseStream();
    const s2 = sseStream();
    const t = setup([s1, null, null, null, null, s2]);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");

    s1.end();
    await t.until(() => t.slept.length === 1);
    expect(t.conn.state).toBe("reconnecting");
    expect(t.slept).toEqual([1000]);

    t.wake(); // the first try fails
    await t.until(() => t.slept.length === 2);
    expect(t.conn.state).toBe("down");
    t.wake();
    await t.until(() => t.slept.length === 3);
    t.wake();
    await t.until(() => t.slept.length === 4);
    t.wake();
    await t.until(() => t.slept.length === 5);
    expect(t.slept).toEqual([1000, 2000, 4000, 5000, 5000]);

    t.wake(); // the sixth try gets through
    s2.send(hello("c2", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");
    t.conn.stop();
  });

  it("does not resync after a reconnect when no HEAD moved", async () => {
    const s1 = sseStream();
    const s2 = sseStream();
    const t = setup([s1, s2]);
    const resync = vi.fn();
    t.conn.on("resync", resync);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => resync.mock.calls.length === 1);
    s1.send(
      ev("tasks.changed", {
        store: "shared",
        ulids: [],
        removed: [],
        commit: "h2",
        op: "",
        author: "",
      }),
    );
    s1.end();
    await t.until(() => t.slept.length === 1);
    t.wake();
    s2.send(hello("c2", { shared: "h2" })); // the head the events already told
    await t.until(() => t.conn.state === "open");
    expect(resync).toHaveBeenCalledTimes(1);
    t.conn.stop();
  });

  it("resyncs after a reconnect when a HEAD moved", async () => {
    const s1 = sseStream();
    const s2 = sseStream();
    const t = setup([s1, s2]);
    const resync = vi.fn();
    t.conn.on("resync", resync);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => resync.mock.calls.length === 1);
    s1.end();
    await t.until(() => t.slept.length === 1);
    t.wake();
    s2.send(hello("c2", { shared: "h9" }));
    await t.until(() => resync.mock.calls.length === 2);
    t.conn.stop();
  });

  it("sends presence when the page is hidden or shown", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.presence.length === 1);

    Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
    await t.until(() => t.presence.length === 2);
    expect(t.presence[1]).toEqual({ client: "c1", visible: false });
    t.conn.stop();
  });

  it("goes down when the first connection is refused", async () => {
    const t = setup([null]);
    t.conn.start();
    await t.until(() => t.slept.length === 1);
    expect(t.conn.state).toBe("down");
    expect(t.conn.lastOpenAt).toBeNull();
    t.conn.stop();
  });

  it("calls onUnauthorized for a 401 auth.token of /api/events, and for nothing else", async () => {
    const calls: string[] = [];
    let res: () => Response = () =>
      new Response(JSON.stringify({ type: "auth.token" }), { status: 401 });
    const f = (async (input: RequestInfo | URL) => {
      if (typeof input === "string") {
        calls.push(input);
        return res();
      }
      return new Response(null, { status: 204 });
    }) as unknown as typeof fetch;
    const onUnauthorized = vi.fn();
    const sleep = () => new Promise<void>(() => {});
    const api = createApi({ token: () => "tok", fetch: f });
    const conn = new Connection({ token: () => "tok", fetch: f, api, sleep, onUnauthorized });
    conn.start();
    await vi.waitFor(() => expect(onUnauthorized).toHaveBeenCalledTimes(1));
    conn.stop();

    onUnauthorized.mockClear();
    res = () => new Response(JSON.stringify({ type: "other" }), { status: 401 });
    const conn2 = new Connection({ token: () => "tok", fetch: f, api, sleep, onUnauthorized });
    conn2.start();
    await vi.waitFor(() => expect(conn2.state).toBe("down"));
    expect(onUnauthorized).not.toHaveBeenCalled();
    conn2.stop();
    expect(calls).toHaveLength(2);
  });
});
