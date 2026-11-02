import { describe, expect, it, vi } from "vitest";
import { matchPeople, PeopleCache } from "./people";
import { changed, FakeConn, fakeApi } from "./testing";

function setup() {
  const world: Record<string, string[]> = { shared: ["alice", "bob"] };
  const { api, calls } = fakeApi((r) => world[r.query.get("store") ?? ""] ?? []);
  const conn = new FakeConn();
  const people = new PeopleCache({ api, conn: conn.asConnection() });
  return { people, conn, calls, world };
}

describe("PeopleCache", () => {
  it("fetches the names of a store once", async () => {
    const { people, calls } = setup();
    expect(people.get("shared")).toBeUndefined();
    const [a, b] = await Promise.all([people.load("shared"), people.load("shared")]);
    expect(a).toEqual(["alice", "bob"]);
    expect(b).toBe(a);
    await people.load("shared");
    expect(calls).toHaveLength(1);
    expect(calls[0]?.path).toBe("/api/people");
    expect(calls[0]?.query.get("store")).toBe("shared");
    expect(people.get("shared")).toBe(a);
  });

  it("keeps the names of each store apart", async () => {
    const { people, world } = setup();
    world.other = ["zed"];
    expect(await people.load("shared")).toEqual(["alice", "bob"]);
    expect(await people.load("other")).toEqual(["zed"]);
  });

  it("fetches again after tasks.changed of that store, and only then", async () => {
    const { people, conn, calls, world } = setup();
    await people.load("shared");
    conn.emit("tasks.changed", changed("other", ["U"], "c1"));
    await people.load("shared");
    expect(calls).toHaveLength(1);

    world.shared = ["alice", "bob", "carol"];
    conn.emit("tasks.changed", changed("shared", ["U"], "c2"));
    expect(people.get("shared")).toEqual(["alice", "bob"]); // stale until the next load
    const fn = vi.fn();
    people.subscribe(fn);
    expect(await people.load("shared")).toEqual(["alice", "bob", "carol"]);
    expect(calls).toHaveLength(2);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("keeps the same array when the names did not change", async () => {
    const { people, conn } = setup();
    const first = await people.load("shared");
    conn.emit("tasks.changed", changed("shared", ["U"], "c1"));
    expect(await people.load("shared")).toBe(first);
  });

  it("makes everything stale at resync", async () => {
    const { people, conn, calls } = setup();
    await people.load("shared");
    conn.emit("resync");
    await people.load("shared");
    expect(calls).toHaveLength(2);
  });
});

describe("matchPeople", () => {
  const names = ["alice", "Bob", "malice", "carol"];
  it("puts the names that start with the text first, and ignores case", () => {
    expect(matchPeople(names, "al")).toEqual(["alice", "malice"]);
    expect(matchPeople(names, "b")).toEqual(["Bob"]);
    expect(matchPeople(names, "")).toEqual(names);
  });

  it("stops at the limit", () => {
    expect(matchPeople(names, "", 2)).toEqual(["alice", "Bob"]);
  });
});
