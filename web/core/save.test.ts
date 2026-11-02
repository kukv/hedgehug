import { describe, expect, it, vi } from "vitest";
import { TaskCache } from "./cache";
import { SaveController } from "./save";
import { FakeConn, fakeApi, problem, showOf, tv } from "./testing";

function setup(route: Parameters<typeof fakeApi>[0] = () => ({})) {
  const { api, calls } = fakeApi((r) => {
    if (r.method === "PATCH") {
      const out = route(r);
      if (out instanceof Response) return out;
      return { task: tv("A", "saved"), commit: "c1", ...(out as object) };
    }
    return route(r);
  });
  const conn = new FakeConn();
  const cache = new TaskCache({ api, conn: conn.asConnection() });
  const timers: { fn: () => void; ms: number; id: number }[] = [];
  let next = 1;
  const setTimer = ((fn: () => void, ms: number) => {
    const id = next++;
    timers.push({ fn, ms, id });
    return id;
  }) as unknown as typeof setTimeout;
  const clearTimer = ((id: number) => {
    const i = timers.findIndex((t) => t.id === id);
    if (i >= 0) timers.splice(i, 1);
  }) as unknown as typeof clearTimeout;
  const save = new SaveController({ api, cache, conn: conn.asConnection(), setTimer, clearTimer });
  const patches = () => calls.filter((c) => c.method === "PATCH");
  return { save, cache, conn, calls, patches, timers };
}

describe("SaveController", () => {
  it("waits two seconds, and starts the wait again at each edit", () => {
    const { save, timers, patches } = setup();
    save.edit("A", "shared", "title", "a");
    expect(timers).toHaveLength(1);
    expect(timers[0]?.ms).toBe(2000);
    save.edit("A", "shared", "title", "ab");
    expect(timers).toHaveLength(1); // the first wait was cleared
    expect(patches()).toHaveLength(0);
  });

  it("sends the fields edited in one PATCH, with the props together", async () => {
    const { save, timers, patches } = setup();
    save.edit("A", "shared", "title", "new title");
    save.edit("A", "shared", "title", "newer title");
    save.edit("A", "shared", "props.points", 3);
    save.edit("A", "shared", "props.due", null);
    save.edit("A", "shared", "assignees", ["alice"]);
    timers[0]?.fn();
    await vi.waitFor(() => expect(save.state("A").kind).toBe("saved"));
    expect(patches()).toHaveLength(1);
    const p = patches()[0];
    expect(p?.path).toBe("/api/tasks/A");
    expect(p?.query.get("store")).toBe("shared");
    expect(p?.body).toEqual({
      title: "newer title",
      assignees: ["alice"],
      props: { points: 3, due: null },
    });
  });

  it("sends at once at flush, and the answer goes to the cache", async () => {
    const { save, cache, timers, patches } = setup();
    save.edit("A", "shared", "title", "x");
    expect(save.editing("A", "title")).toBe(true);
    await save.flush("A");
    expect(timers).toHaveLength(0);
    expect(patches()).toHaveLength(1);
    expect(save.editing("A", "title")).toBe(false);
    expect(cache.task("A")?.title).toBe("saved");
    expect(save.state("A")).toEqual({ kind: "saved", pendingPush: false });
  });

  it("goes through saving, and says push is pending when the store has unpushed commits", async () => {
    const { save, conn } = setup();
    const kinds: string[] = [];
    save.subscribe(() => kinds.push(save.state("A").kind));
    save.edit("A", "shared", "title", "x");
    await save.flush("A");
    expect(kinds).toContain("saving");
    expect(save.state("A")).toEqual({ kind: "saved", pendingPush: false });

    conn.changeSync({ store: "shared", remotes: ["origin"], unpushed: 2 });
    expect(save.state("A")).toEqual({ kind: "saved", pendingPush: true });
    conn.changeSync({ store: "shared", remotes: ["origin"], unpushed: 0 });
    expect(save.state("A")).toEqual({ kind: "saved", pendingPush: false });
  });

  it("shows the words of the error, and sends the same edits again at the retry", async () => {
    let fail = true;
    const { save, patches } = setup(() => (fail ? problem(422, "schema.violation") : {}));
    save.edit("A", "shared", "status", "bogus");
    await save.flush("A");
    const s = save.state("A");
    expect(s.kind).toBe("error");
    expect(s.kind === "error" && s.message).toContain("スキーマに合わない変更です");
    expect(save.editing("A", "status")).toBe(true);

    fail = false;
    await save.flush("A");
    expect(patches()).toHaveLength(2);
    expect(patches()[1]?.body).toEqual({ status: "bogus" });
    expect(save.state("A").kind).toBe("saved");
  });

  it("sends one PATCH of a task at a time, in the order of the edits", async () => {
    const { save, patches } = setup();
    let second: Promise<void> | undefined;
    let fired = false;
    save.subscribe(() => {
      // An edit that comes while the first PATCH is on its way.
      if (fired || save.state("A").kind !== "saving") return;
      fired = true;
      save.edit("A", "shared", "title", "two");
      expect(save.editing("A", "title")).toBe(true);
      second = save.flush("A");
    });
    save.edit("A", "shared", "title", "one");
    await save.flush("A");
    await second;
    expect(patches().map((p) => (p.body as { title: string }).title)).toEqual(["one", "two"]);
    expect(save.editing("A", "title")).toBe(false);
  });

  it("keeps editing true until the answer is in the cache, and after a failure", async () => {
    let fail = true;
    const { save, cache } = setup(() => (fail ? problem(422, "schema.violation") : {}));
    const seen: { editing: boolean; title?: string }[] = [];
    save.subscribe(() =>
      seen.push({ editing: save.editing("A", "body"), title: cache.task("A")?.title }),
    );
    save.edit("A", "shared", "body", "text");
    expect(save.editing("A", "body")).toBe(true); // not sent
    await save.flush("A");
    expect(save.editing("A", "body")).toBe(true); // failed: the text is still the user's
    fail = false;
    seen.length = 0;
    await save.flush("A");
    expect(save.editing("A", "body")).toBe(false);
    // false is never seen before the cache has the answer
    for (const s of seen) if (!s.editing) expect(s.title).toBe("saved");
    expect(seen.some((s) => s.editing)).toBe(true); // while it was on its way
  });

  it("does nothing at flush when nothing was edited", async () => {
    const { save, patches } = setup();
    await save.flush("A");
    expect(patches()).toHaveLength(0);
    expect(save.state("A")).toEqual({ kind: "idle" });
  });

  it("puts the sent body into the detail before editing turns false", async () => {
    const { save, cache } = setup((r) =>
      r.method === "GET" ? showOf(tv("A", "a"), { body: "old" }) : {},
    );
    const w = cache.watchDetail("A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    const seen: { editing: boolean; body?: string }[] = [];
    save.subscribe(() =>
      seen.push({ editing: save.editing("A", "body"), body: cache.detail("A")?.body }),
    );
    save.edit("A", "shared", "body", "new text");
    await save.flush("A");
    expect(cache.detail("A")?.body).toBe("new text");
    expect(save.editing("A", "body")).toBe(false);
    for (const s of seen) if (!s.editing) expect(s.body).toBe("new text");
    w.dispose();
  });

  it("is not saved while another edit is waiting", async () => {
    const { save, timers } = setup();
    let fired = false;
    save.subscribe(() => {
      if (fired || save.state("A").kind !== "saving") return;
      fired = true;
      save.edit("A", "shared", "title", "again");
    });
    save.edit("A", "shared", "title", "one");
    await save.flush("A");
    expect(save.state("A").kind).toBe("saving");
    expect(timers).toHaveLength(1);
    await save.flush("A");
    expect(save.state("A").kind).toBe("saved");
  });

  it("drops the chain of a task when its last PATCH is over", async () => {
    const { save } = setup();
    save.edit("A", "shared", "title", "x");
    await save.flush("A");
    await Promise.resolve();
    expect((save as unknown as { chains: Map<string, unknown> }).chains.size).toBe(0);
  });

  it("does not send a field the server refused with the next edit of another field", async () => {
    let status: number | null = 422;
    const { save, patches } = setup((r) => {
      const b = r.body as { status?: string };
      return status && b.status ? problem(status, "schema.violation") : {};
    });
    save.edit("A", "shared", "status", "bogus");
    await save.flush("A");
    save.edit("A", "shared", "title", "t");
    await save.flush("A");
    expect(patches().map((p) => p.body)).toEqual([{ status: "bogus" }, { title: "t" }]);
    const s = save.state("A");
    expect(s.kind === "error" && s.message).toContain("スキーマに合わない変更です");
    expect(save.editing("A", "status")).toBe(true);

    // Nothing else waits: a retry sends it again.
    status = null;
    await save.flush("A");
    expect(patches()[2]?.body).toEqual({ status: "bogus" });
    expect(save.state("A").kind).toBe("saved");
  });

  it("sends a refused field again when it is edited again", async () => {
    let refuse = true;
    const { save, patches } = setup((r) =>
      refuse && (r.body as { status?: string }).status ? problem(422, "schema.violation") : {},
    );
    save.edit("A", "shared", "status", "bogus");
    await save.flush("A");
    refuse = false;
    save.edit("A", "shared", "title", "t");
    save.edit("A", "shared", "status", "done");
    await save.flush("A");
    expect(patches()[1]?.body).toEqual({ title: "t", status: "done" });
    expect(save.state("A").kind).toBe("saved");
  });

  it("merges the fields of a failure that may pass (5xx) into the next save", async () => {
    let fail = true;
    const { save, patches } = setup(() => (fail ? problem(500, "internal") : {}));
    save.edit("A", "shared", "status", "done");
    await save.flush("A");
    fail = false;
    save.edit("A", "shared", "title", "t");
    await save.flush("A");
    expect(patches()[1]?.body).toEqual({ status: "done", title: "t" });
    expect(save.state("A").kind).toBe("saved");
  });

  it("saves the next edit after a subscriber threw inside a save", async () => {
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    const { save, patches } = setup();
    let boom = true;
    save.subscribe(() => {
      if (boom && save.state("A").kind === "saved") {
        boom = false;
        throw new Error("subscriber");
      }
    });
    save.edit("A", "shared", "title", "one");
    await save.flush("A"); // does not reject
    expect(save.editing("A", "title")).toBe(false);
    save.edit("A", "shared", "title", "two");
    await save.flush("A");
    expect(patches().map((p) => (p.body as { title: string }).title)).toEqual(["one", "two"]);
    expect(save.state("A").kind).toBe("saved");
    err.mockRestore();
  });

  it("saves the next edit after the cache threw on an answer", async () => {
    const { save, cache, patches } = setup();
    const apply = vi.spyOn(cache, "applyWrite").mockImplementationOnce(() => {
      throw new Error("cache");
    });
    save.edit("A", "shared", "title", "one");
    await save.flush("A");
    expect(save.state("A").kind).toBe("error");
    expect(save.editing("A", "title")).toBe(true);
    apply.mockRestore();
    save.edit("A", "shared", "title", "two");
    await save.flush("A");
    expect(patches()).toHaveLength(2);
    expect(save.state("A").kind).toBe("saved");
  });

  it("sends the PATCH and ends saved when a subscriber throws on every change", async () => {
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    const { save, patches } = setup();
    save.subscribe(() => {
      throw new Error("subscriber");
    });
    save.edit("A", "shared", "title", "one");
    await save.flush("A");
    expect(patches()).toHaveLength(1);
    expect(save.state("A").kind).toBe("saved");
    expect(save.editing("A", "title")).toBe(false);
    err.mockRestore();
  });
});
