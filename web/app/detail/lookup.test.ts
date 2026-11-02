import { describe, expect, it, vi } from "vitest";
import { ancestorChain, type Crumb, collapseCrumbs, createLookup, crumbsSig } from "./lookup";
import { makeCore, ok, showView, taskView } from "./testkit";

describe("createLookup", () => {
  it("answers from the cache without a request", async () => {
    const k = makeCore({ tasks: [taskView({ ulid: "U1", id: "REL-1", title: "one" })] });
    const lookup = createLookup({ api: k.api as never, cache: k.cache as never, store: "shared" });
    expect((await lookup("U1"))?.id).toBe("REL-1");
    expect(k.api.GET).not.toHaveBeenCalled();
  });

  it("fetches a task the cache lacks once for concurrent calls, and gives undefined on an error", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() => ok(showView({ ulid: "U9", id: "REL-9", title: "nine" })));
    const lookup = createLookup({ api: k.api as never, cache: k.cache as never, store: "shared" });
    const [a, b] = await Promise.all([lookup("REL-9"), lookup("REL-9")]);
    expect(a?.ulid).toBe("U9");
    expect(b).toBe(a);
    expect(k.api.GET).toHaveBeenCalledTimes(1);
    k.api.GET.mockImplementation(() => Promise.reject(new Error("down")));
    expect(await lookup("REL-10")).toBeUndefined();
  });

  it("puts what it fetched in the cache, so that the row follows the task", async () => {
    const k = makeCore();
    const show = showView({ ulid: "U9", id: "REL-9" });
    k.api.GET.mockImplementation(() => ok(show));
    k.cache.epochNow.mockReturnValue(7);
    const lookup = createLookup({ api: k.api as never, cache: k.cache as never, store: "shared" });
    await lookup("REL-9");
    expect(k.cache.prime).toHaveBeenCalledWith([show], 7);
  });
});

describe("ancestorChain", () => {
  const crumb = (ulid: string, parent: string | null): Crumb => ({
    ulid,
    id: ulid,
    title: ulid,
    kind: "task",
    status: "todo",
    parent,
    store: "shared",
  });
  const all: Record<string, Crumb> = {
    C: crumb("C", "B"),
    B: crumb("B", "A"),
    A: crumb("A", null),
  };

  it("returns the ancestors with the root first", async () => {
    const lookup = vi.fn(async (u: string) => all[u]);
    expect((await ancestorChain("C", lookup)).map((c) => c.ulid)).toEqual(["A", "B", "C"]);
    expect(await ancestorChain(null, lookup)).toEqual([]);
  });

  it("stops at a task it cannot read and at the limit", async () => {
    expect(
      (await ancestorChain("C", async (u) => (u === "A" ? undefined : all[u]))).map((c) => c.ulid),
    ).toEqual(["B", "C"]);
    const loop: Record<string, Crumb> = { X: crumb("X", "X") };
    expect(await ancestorChain("X", async (u) => loop[u], 3)).toHaveLength(3);
  });
});

describe("collapseCrumbs", () => {
  it("keeps three or fewer and collapses the middle of longer chains", () => {
    expect(collapseCrumbs([1, 2, 3])).toEqual([1, 2, 3]);
    expect(collapseCrumbs([1, 2, 3, 4, 5])).toEqual([1, null, 4, 5]);
  });
});

describe("crumbsSig", () => {
  it("changes with the title or the status of a cached task, and not with the rest", () => {
    const rows = [taskView({ ulid: "U1", title: "one" })];
    const cache = { task: (u: string) => rows.find((t) => t.ulid === u) };
    const a = crumbsSig(cache, ["U1", "U2"]);
    rows[0] = taskView({ ulid: "U1", title: "one", rank: "b0" });
    expect(crumbsSig(cache, ["U1", "U2"])).toBe(a);
    rows[0] = taskView({ ulid: "U1", title: "uno" });
    expect(crumbsSig(cache, ["U1", "U2"])).not.toBe(a);
    rows[0] = taskView({ ulid: "U1", title: "one", status: "done" });
    expect(crumbsSig(cache, ["U1", "U2"])).not.toBe(a);
  });
});
