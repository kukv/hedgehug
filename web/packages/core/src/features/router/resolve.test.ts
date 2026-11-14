import { beforeEach, describe, expect, it } from "vitest";
import { type FakeTransfers, fakeTransfers } from "../transfers/transfers.test-util";
import { resolveRoute, routeOfProject, routeOfWorkspace } from "./resolve";
import { itemsIn, stores } from "./resolve.test-util";

let transfers: FakeTransfers;
beforeEach(() => {
  transfers = fakeTransfers();
});

describe("resolveRoute", () => {
  it("resolves a project by its display ID in the named store", async () => {
    const r = await resolveRoute(
      { name: "project", store: "ro", project: "HH-P2", tab: "tree" },
      await stores(),
      itemsIn("S1"),
      transfers,
    );
    expect(r).toEqual({ name: "project", storeId: "S2", projectId: "S2-P2", tab: "tree" });
  });

  // アドレスの Store にその ID がなければ、ほかの Store のいまの ID より先に、アドレスの Store の旧 ID を見る
  // (Store 名の Store を優先。01「画面のアドレス」)。どちらもサーバーの locate が決める。
  it("asks locate instead of the other stores when the named store has no such ID", async () => {
    transfers.located = { kind: "found", storeId: "S2", projectId: "S2-P1", itemId: "S2-I12" };
    const r = await resolveRoute(
      { name: "item", store: "work", item: "HH-12" },
      await stores(),
      itemsIn("S2"),
      transfers,
    );
    expect(transfers.calls).toEqual(["locate work HH-12"]);
    expect(r).toEqual({ name: "item", storeId: "S2", projectId: "S2-P1", itemId: "S2-I12" });
  });

  it("does not open a current ID of another store when locate finds an old ID of the named store", async () => {
    transfers.located = { kind: "found", storeId: "S1", projectId: "S1-P3", itemId: "S1-I30" };
    const r = await resolveRoute(
      { name: "item", store: "work", item: "HH-12" },
      await stores(),
      itemsIn("S2"),
      transfers,
    );
    expect(r).toEqual({ name: "item", storeId: "S1", projectId: "S1-P3", itemId: "S1-I30" });
  });

  it("resolves a workspace by its prefix", async () => {
    const r = await resolveRoute(
      { name: "workspace", store: "work", prefix: "HH", tab: "overview" },
      await stores(),
      itemsIn("S1"),
      transfers,
    );
    expect(r).toEqual({ name: "workspace", storeId: "S1", workspaceId: "S1-W1", tab: "overview" });
  });

  it.each(["sprints", "releases"] as const)("keeps the %s tab of a workspace", async (tab) => {
    const r = await resolveRoute(
      { name: "workspace", store: "work", prefix: "HH", tab },
      await stores(),
      itemsIn("S1"),
      transfers,
    );
    expect(r).toEqual({ name: "workspace", storeId: "S1", workspaceId: "S1-W1", tab });
  });

  it.each([
    { name: "project", store: "work", project: "HH-P9", tab: "overview" },
    { name: "project", store: "work", project: "HH-9", tab: "overview" },
    { name: "item", store: "work", item: "HH-13" },
    { name: "item", store: "work", item: "XX-12" },
    { name: "workspace", store: "work", prefix: "XX", tab: "overview" },
  ] as const)("goes home when %o is not found", async (route) => {
    transfers.missing.add("HH-P9");
    transfers.missing.add("HH-13");
    transfers.missing.add("XX-12");
    transfers.missing.add("XX");
    expect(await resolveRoute(route, await stores(), itemsIn("S1"), transfers)).toEqual({
      name: "home",
    });
  });

  it("passes on errors other than not found", async () => {
    const items = itemsIn("S1");
    items.findByNumber = async () => {
      throw new Error("offline");
    };
    await expect(
      resolveRoute(
        { name: "item", store: "work", item: "HH-12" },
        await stores(),
        items,
        transfers,
      ),
    ).rejects.toThrow("offline");
  });
});

describe("routes of internal IDs", () => {
  it("writes the addresses of a project and a workspace", async () => {
    const s = await stores();
    expect(routeOfProject(s, "S2", "S2-P1", "overview")).toEqual({
      name: "project",
      store: "ro",
      project: "HH-P1",
      tab: "overview",
    });
    expect(routeOfWorkspace(s, "S1", "S1-W1")).toEqual({
      name: "workspace",
      store: "work",
      prefix: "HH",
      tab: "overview",
    });
    expect(routeOfWorkspace(s, "S1", "S1-W1", "settings")).toMatchObject({ tab: "settings" });
    expect(routeOfWorkspace(s, "S1", "S1-W1", "sprints")).toMatchObject({ tab: "sprints" });
    expect(routeOfProject(s, "S1", "nope", "tree")).toEqual({ name: "home" });
    expect(routeOfWorkspace(s, "S9", "S1-W1")).toEqual({ name: "home" });
  });
});

describe("resolveRoute settings", () => {
  it("resolves the settings without any store", async () => {
    expect(
      await resolveRoute({ name: "settings", tab: "accounts" }, [], itemsIn("none"), transfers),
    ).toEqual({
      name: "settings",
      tab: "accounts",
    });
  });
});
