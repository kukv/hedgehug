import { afterEach, describe, expect, it, vi } from "vitest";
import { TaskCache } from "./cache";
import { changed, FakeConn, fakeApi, problem, type Req, settle, showOf, tv } from "./testing";

afterEach(() => {
  vi.useRealTimers();
});

function setup(route: Parameters<typeof fakeApi>[0]) {
  const { api, calls } = fakeApi(route);
  const conn = new FakeConn();
  let t = 1_000_000;
  const cache = new TaskCache({ api, conn: conn.asConnection(), now: () => t });
  return { cache, conn, calls, advance: (ms: number) => (t += ms) };
}

const listCalls = (calls: Req[]) => calls.filter((c) => c.path === "/api/tasks");

describe("TaskCache lists", () => {
  it("fetches the query and keeps each task once by ULID", async () => {
    const { cache, calls } = setup(() => ({ tasks: [tv("A", "a"), tv("B", "b")] }));
    const w = cache.watchList({ q: "kind:task", tree: false, store: "shared" });
    expect(w.loading()).toBe(true);
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(w.ulids()).toEqual(["A", "B"]);
    expect(cache.task("A")?.title).toBe("a");
    expect(listCalls(calls)[0]?.query.get("q")).toBe("kind:task");
    expect(listCalls(calls)[0]?.query.get("store")).toBe("shared");
    w.dispose();
  });

  it("shares one fetch between watchers of the same query", async () => {
    const { cache, calls } = setup(() => ({ tasks: [tv("A")] }));
    const w1 = cache.watchList({ q: "", tree: false });
    const w2 = cache.watchList({ q: "", tree: false });
    await vi.waitFor(() => expect(w2.loading()).toBe(false));
    expect(listCalls(calls)).toHaveLength(1);
    w1.dispose();
    w2.dispose();
  });

  it("builds the tree from the children and holds the cycle", async () => {
    const root = tv("R", "root", {
      children: [tv("C1", "c1", { children: [tv("G", "g")] }), tv("C2")],
    });
    const { cache, calls } = setup(() => ({ roots: [root], cycle: [tv("X")] }));
    const w = cache.watchTree({ q: "workspace:main", tree: true });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(listCalls(calls)[0]?.query.get("tree")).toBe("true");
    expect(w.roots()).toEqual([
      {
        ulid: "R",
        children: [
          { ulid: "C1", children: [{ ulid: "G", children: [] }] },
          { ulid: "C2", children: [] },
        ],
      },
    ]);
    expect(w.cycle()).toEqual(["X"]);
    expect(cache.task("G")?.title).toBe("g");
    expect(cache.task("R")?.children).toBeUndefined();
    w.dispose();
  });

  it("marks the context ancestors on the tree node, not on the row", async () => {
    let ctx = true;
    const { cache, conn } = setup((req) => {
      if (req.query.get("tree") === "true") {
        return { roots: [tv("R", "r", { context: ctx, children: [tv("A", "a")] })] };
      }
      return { tasks: [tv("R", "r")] }; // another query finds R itself: no context
    });
    const w = cache.watchTree({ q: "x", tree: true });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(w.roots()).toEqual([
      { ulid: "R", context: true, children: [{ ulid: "A", children: [] }] },
    ]);
    expect(cache.task("R")).not.toHaveProperty("context");

    const l = cache.watchList({ q: "y", tree: false });
    await vi.waitFor(() => expect(l.loading()).toBe(false));
    expect(w.roots()[0]?.context).toBe(true);

    // the tree follows the answer when the mark itself changes
    ctx = false;
    conn.emit("resync");
    await settle();
    await settle();
    expect(w.roots()[0]).toEqual({ ulid: "R", children: [{ ulid: "A", children: [] }] });
    w.dispose();
    l.dispose();
  });

  it("keeps the objects of the tasks that did not change when it fetches again", async () => {
    let titleB = "b";
    const { cache, conn } = setup(() => ({
      roots: [tv("R", "r", { children: [tv("A", "a"), tv("B", titleB)] })],
    }));
    const w = cache.watchTree({ q: "", tree: true });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    const a = cache.task("A");
    const b = cache.task("B");
    const roots = w.roots();

    conn.emit("resync");
    await settle();
    await settle();
    expect(cache.task("A")).toBe(a);
    expect(cache.task("B")).toBe(b);
    expect(w.roots()).toBe(roots);

    titleB = "b2";
    conn.emit("tasks.changed", changed("shared", ["B"], "c1"));
    await vi.waitFor(() => expect(cache.task("B")?.title).toBe("b2"));
    expect(cache.task("A")).toBe(a);
    expect(cache.task("B")).not.toBe(b);
    expect(w.roots()).toBe(roots); // the shape is the same
    w.dispose();
  });

  it("notifies only when something changed", async () => {
    const { cache, conn } = setup(() => ({ tasks: [tv("A")] }));
    const w = cache.watchList({ q: "", tree: false });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    const fn = vi.fn();
    cache.subscribe(fn);
    conn.emit("resync");
    await settle();
    await settle();
    expect(fn).not.toHaveBeenCalled();
    w.dispose();
  });

  it("fetches the queries on screen again at tasks.changed, and no others", async () => {
    const { cache, conn, calls } = setup(() => ({ tasks: [] }));
    const w1 = cache.watchList({ q: "a", tree: false });
    const w2 = cache.watchList({ q: "b", tree: false });
    await vi.waitFor(() => expect(w2.loading()).toBe(false));
    w2.dispose();
    const before = listCalls(calls).length;
    conn.emit("tasks.changed", changed("shared", ["U"], "c1"));
    await vi.waitFor(() => expect(listCalls(calls).length).toBe(before + 1));
    await settle();
    expect(listCalls(calls).length).toBe(before + 1);
    w1.dispose();
  });

  it("holds the error of a query", async () => {
    const { cache } = setup(() => problem(400, "query.invalid"));
    const w = cache.watchList({ q: "(", tree: false });
    await vi.waitFor(() => expect(w.error()).not.toBeNull());
    expect(w.error()?.type).toBe("query.invalid");
    expect(w.loading()).toBe(false);
    w.dispose();
  });
});

describe("TaskCache details", () => {
  it("fetches a task by ID and finds it by ULID", async () => {
    const t = tv("A", "a");
    const { cache, calls } = setup(() => showOf(t, { body: "hello" }));
    const w = cache.watchDetail("T-A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    expect(calls[0]?.path).toBe("/api/tasks/T-A");
    expect(cache.detail("A")?.body).toBe("hello");
    expect(cache.task("A")?.title).toBe("a");
    expect(cache.task("A")).not.toHaveProperty("body");
    w.dispose();
  });

  it("fetches a detail again only when tasks.changed names it", async () => {
    const { cache, conn, calls } = setup((r) => showOf(tv(r.path.endsWith("/A") ? "A" : "B")));
    const wa = cache.watchDetail("A");
    const wb = cache.watchDetail("B");
    await vi.waitFor(() => expect(wb.ulid()).toBe("B"));
    await vi.waitFor(() => expect(wa.ulid()).toBe("A"));
    const n = calls.length;
    conn.emit("tasks.changed", changed("shared", ["A"], "c1"));
    await vi.waitFor(() => expect(calls.length).toBe(n + 1));
    await settle();
    expect(calls.slice(n).map((c) => c.path)).toEqual(["/api/tasks/A"]);
    wa.dispose();
    wb.dispose();
  });

  it("forgets a purged task and shows the 404 of its detail", async () => {
    let gone = false;
    const { cache, conn } = setup(() => (gone ? problem(404, "task.not_found") : showOf(tv("A"))));
    const w = cache.watchDetail("A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    gone = true;
    conn.emit("tasks.changed", changed("shared", [], "c2", "purge", ["A"]));
    await vi.waitFor(() => expect(w.error()?.status).toBe(404));
    expect(w.ulid()).toBeNull();
    expect(cache.task("A")).toBeUndefined();
    expect(cache.detail("A")).toBeUndefined();
    w.dispose();
  });
});

describe("TaskCache marks", () => {
  it("marks a change from outside for ten seconds, and tells when it is gone", () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const { cache, conn, advance } = setup(() => ({}));
    const fn = vi.fn();
    cache.subscribe(fn);
    conn.emit("tasks.changed", changed("shared", ["A"], "c1", "set"));
    expect(cache.live("A")).toEqual({ label: "CLI で更新 · 今", until: 1_010_000 });
    expect(cache.live("B")).toBeUndefined();
    expect(fn).toHaveBeenCalledTimes(1);

    advance(10_000);
    vi.advanceTimersByTime(10_000);
    expect(cache.live("A")).toBeUndefined();
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it("says sync for merge and repair", () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const { cache, conn } = setup(() => ({}));
    conn.emit("tasks.changed", changed("shared", ["A"], "c1", "merge"));
    conn.emit("tasks.changed", changed("shared", ["B"], "c2", "repair"));
    conn.emit("tasks.changed", changed("shared", ["C"], "c3", "move"));
    expect(cache.live("A")?.label).toBe("同期で更新 · 今");
    expect(cache.live("B")?.label).toBe("同期で更新 · 今");
    expect(cache.live("C")?.label).toBe("CLI で更新 · 今");
  });

  it("does not mark the commit this tab made, whether the event or the answer comes first", () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const { cache, conn } = setup(() => ({}));
    cache.noteOwnCommit("own1");
    conn.emit("tasks.changed", changed("shared", ["A"], "own1"));
    expect(cache.live("A")).toBeUndefined();

    conn.emit("tasks.changed", changed("shared", ["B"], "own2"));
    expect(cache.live("B")).toBeDefined();
    cache.noteOwnCommit("own2");
    expect(cache.live("B")).toBeUndefined();
  });
});

describe("TaskCache writes", () => {
  it("takes in the task of a write, and puts it into the detail", async () => {
    const t = tv("A", "a");
    const { cache } = setup(() => showOf(t, { body: "text" }));
    const w = cache.watchDetail("A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    const fn = vi.fn();
    cache.subscribe(fn);

    cache.applyWrite({ task: tv("A", "renamed"), commit: "c9" });
    expect(cache.task("A")?.title).toBe("renamed");
    expect(cache.detail("A")?.title).toBe("renamed");
    expect(cache.detail("A")?.body).toBe("text");
    expect(fn).toHaveBeenCalled();
    w.dispose();
  });

  it("forgets the tasks purged", async () => {
    const { cache } = setup(() => ({ tasks: [tv("A"), tv("B")] }));
    const w = cache.watchList({ q: "", tree: false });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    cache.applyWrite({ ulids: ["A"], commit: "c9" });
    expect(cache.task("A")).toBeUndefined();
    expect(cache.task("B")).toBeDefined();
    w.dispose();
  });
});

describe("TaskCache fetch order", () => {
  it("does not let a list sent before a write undo the write", async () => {
    let open!: () => void;
    const gate = new Promise<void>((r) => {
      open = r;
    });
    const { cache } = setup(async () => {
      await gate;
      return { tasks: [tv("A", "old")] };
    });
    const w = cache.watchList({ q: "", tree: false });
    cache.applyWrite({ task: tv("A", "new"), commit: "c1" });
    open();
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(w.ulids()).toEqual(["A"]);
    expect(cache.task("A")?.title).toBe("new");
    w.dispose();
  });

  it("does not let a detail sent before a write undo the write", async () => {
    let n = 0;
    let open!: () => void;
    const gate = new Promise<void>((r) => {
      open = r;
    });
    const { cache, conn } = setup(async () => {
      const k = n++;
      if (k === 0) return showOf(tv("A", "a"), { body: "b" });
      if (k > 1) return showOf(tv("A", "new"), { body: "b" }); // fetched again after the write
      await gate;
      return showOf(tv("A", "old"), { body: "b" });
    });
    const w = cache.watchDetail("A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    conn.emit("resync"); // a second fetch is on its way when the write comes
    await settle();
    cache.applyWrite({ task: tv("A", "new"), commit: "c1" });
    open();
    await settle();
    await settle();
    expect(cache.task("A")?.title).toBe("new");
    expect(cache.detail("A")?.title).toBe("new");
    w.dispose();
  });

  it("fetches the detail again when an event came before the answer of the write", async () => {
    let n = 0;
    let repos = ["old/repo"];
    let open!: () => void;
    const gate = new Promise<void>((r) => {
      open = r;
    });
    const { cache, conn, calls } = setup(async () => {
      if (n++ === 1) {
        // the fetch the event started; the write is committed by now
        repos = ["new/repo"];
        await gate;
        return showOf(tv("A", "new"), { repos });
      }
      return showOf(tv("A", n === 1 ? "a" : "new"), { repos });
    });
    const w = cache.watchDetail("A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    expect(cache.detail("A")?.repos).toEqual(["old/repo"]);

    conn.emit("tasks.changed", changed("shared", ["A"], "c1"));
    await settle();
    cache.applyWrite({ task: tv("A", "new"), commit: "c1" });
    open();
    await vi.waitFor(() => expect(cache.detail("A")?.repos).toEqual(["new/repo"]));
    expect(calls.filter((c) => c.path === "/api/tasks/A")).toHaveLength(3);
    w.dispose();
  });

  it("does not bring back a task purged after the list was sent", async () => {
    let open!: () => void;
    const gate = new Promise<void>((r) => {
      open = r;
    });
    const { cache } = setup(async () => {
      await gate;
      return { tasks: [tv("A"), tv("B")] };
    });
    const w = cache.watchList({ q: "", tree: false });
    cache.applyWrite({ ulids: ["A"], commit: "c1" });
    open();
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(cache.task("A")).toBeUndefined();
    expect(cache.task("B")).toBeDefined();
    expect(w.ulids()).toEqual(["B"]);
    w.dispose();
  });

  it("does not bring back a task purged after the tree was sent", async () => {
    let open!: () => void;
    const gate = new Promise<void>((r) => {
      open = r;
    });
    const { cache } = setup(async () => {
      await gate;
      return { roots: [tv("R", "r", { children: [tv("A"), tv("B")] })] };
    });
    const w = cache.watchTree({ q: "", tree: true });
    cache.applyWrite({ ulids: ["A"], commit: "c1" });
    open();
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(w.roots()).toEqual([{ ulid: "R", children: [{ ulid: "B", children: [] }] }]);
    w.dispose();
  });

  it("does not notify when a detail fetched again did not change", async () => {
    const { cache, conn } = setup(() => showOf(tv("A", "a")));
    const w = cache.watchDetail("A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    const fn = vi.fn();
    cache.subscribe(fn);
    conn.emit("resync");
    await settle();
    await settle();
    expect(fn).not.toHaveBeenCalled();
    w.dispose();
  });

  it("ignores the answer of a list whose watch was released", async () => {
    const { cache, calls } = setup(() => ({ tasks: [tv("A")] }));
    const w = cache.watchList({ q: "", tree: false });
    w.dispose();
    await vi.waitFor(() => expect(calls).toHaveLength(1));
    await settle();
    await settle();
    expect(cache.task("A")).toBeUndefined();
  });
});

describe("TaskCache start-up and watchList", () => {
  it("picks up a change made between the first fetch and the first hello", async () => {
    let title = "old";
    const { api, calls } = fakeApi(() => ({ tasks: [tv("A", title)] }));
    const conn = new FakeConn();
    conn.lastOpenAt = null; // not open yet
    const cache = new TaskCache({ api, conn: conn.asConnection() });
    const w = cache.watchList({ q: "", tree: false });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(cache.task("A")?.title).toBe("old");
    title = "new"; // committed before the stream subscribed
    conn.emit("resync"); // the first hello
    await vi.waitFor(() => expect(cache.task("A")?.title).toBe("new"));
    expect(listCalls(calls)).toHaveLength(2);
    w.dispose();
  });

  it("lists in a flat list even when tree: true is passed", async () => {
    const { cache, calls } = setup(() => ({ tasks: [tv("A")], roots: [tv("R")] }));
    const w = cache.watchList({ q: "", tree: true });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(listCalls(calls)[0]?.query.get("tree")).toBe("false");
    expect(w.ulids()).toEqual(["A"]);
    w.dispose();
  });
});
