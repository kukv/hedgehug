import { beforeEach, describe, expect, it } from "vitest";
import type { StoreNode, WorkspaceNode } from "../catalog/catalog";
import { type FakeTransfers, fakeTransfers } from "../transfers/transfers.test-util";
import { resolveRoute } from "./resolve";
import { itemsIn, stores } from "./resolve.test-util";

let transfers: FakeTransfers;
beforeEach(() => {
  transfers = fakeTransfers();
});

// いまの ID で見つからないときだけ、サーバーの locate(旧 ID・旧い接頭辞・転送記録。A5a-15)に聞く。
describe("resolveRoute with old IDs", () => {
  it("does not call locate when the current ID is found", async () => {
    const r = await resolveRoute(
      { name: "item", store: "work", item: "HH-12" },
      await stores(),
      itemsIn("S1"),
      transfers,
    );
    expect(r).toMatchObject({ name: "item", itemId: "S1-I12" });
    expect(transfers.calls).toEqual([]);
  });

  it("asks locate with the Store name and the ID when the ID is not found, and opens what it finds with the moved-from note", async () => {
    const movedFrom = {
      old: "HH-5",
      at: "2026-11-13T00:05:00.000Z",
      display: "TM-4",
      current: "AR-7",
    };
    transfers.located = {
      kind: "found",
      storeId: "S3",
      projectId: "S3-P1",
      itemId: "S3-I9",
      movedFrom,
    };
    const r = await resolveRoute(
      { name: "item", store: "work", item: "HH-5" },
      await stores(),
      itemsIn("S1"),
      transfers,
    );
    expect(transfers.calls).toEqual(["locate work HH-5"]);
    expect(r).toEqual({
      name: "item",
      storeId: "S3",
      projectId: "S3-P1",
      itemId: "S3-I9",
      movedFrom,
    });
  });

  it("opens a found item without movedFrom when locate gives none (an old prefix)", async () => {
    transfers.located = { kind: "found", storeId: "S1", projectId: "S1-P1", itemId: "S1-I2" };
    const r = await resolveRoute(
      { name: "item", store: "work", item: "OLD-2" },
      await stores(),
      itemsIn("S1"),
      transfers,
    );
    expect(r).toEqual({ name: "item", storeId: "S1", projectId: "S1-P1", itemId: "S1-I2" });
  });

  it("gives the Gone of a gone item with home", async () => {
    const gone = {
      reason: "purged",
      old: "HH-5",
      at: "2026-11-13T00:05:00.000Z",
      newDisplay: "TM-4",
    } as const;
    transfers.located = { kind: "gone", ...gone };
    expect(
      await resolveRoute(
        { name: "item", store: "work", item: "HH-5" },
        await stores(),
        itemsIn("S1"),
        transfers,
      ),
    ).toEqual({ name: "home", gone });
  });

  it("is plain home when locate has nothing (NotFound), and does not ask for an unreadable ID", async () => {
    transfers.missing.add("HH-99");
    const s = await stores();
    expect(
      await resolveRoute(
        { name: "item", store: "work", item: "HH-99" },
        s,
        itemsIn("S1"),
        transfers,
      ),
    ).toEqual({ name: "home" });
    transfers.calls.length = 0;
    expect(
      await resolveRoute({ name: "item", store: "work", item: "abc" }, s, itemsIn("S1"), transfers),
    ).toEqual({ name: "home" });
    expect(transfers.calls).toEqual([]);
  });

  it("passes on errors of locate other than not found", async () => {
    transfers.failNext();
    await expect(
      resolveRoute(
        { name: "item", store: "work", item: "HH-5" },
        await stores(),
        itemsIn("S1"),
        transfers,
      ),
    ).rejects.toThrow("network");
  });
});

// 旧い接頭辞でのワークスペース・プロジェクトのアドレス(A5a-21)。いまの接頭辞が優先。
// ws は 1 つのワークスペース。プロジェクトは各ワークスペースに 1 つ(番号 1)。
const ws = (storeId: string, id: string, prefix: string, oldPrefixes: string[]) =>
  ({
    id,
    storeId,
    name: id,
    prefix,
    oldPrefixes,
    projects: [{ id: `${id}-P1`, number: 1, displayId: `${prefix}-P1` }],
  }) as WorkspaceNode;
const storeOf = (id: string, name: string, ...list: WorkspaceNode[]) =>
  ({ id, name, color: "teal", readOnly: false, workspaces: list }) as StoreNode;
const one = (...list: WorkspaceNode[]) => [storeOf("S1", "work", ...list)];
const wsRoute = (prefix: string) =>
  ({ name: "workspace", store: "work", prefix, tab: "overview" }) as const;
const pRoute = (project: string) =>
  ({ name: "project", store: "work", project, tab: "tree" }) as const;

describe("resolveRoute with old prefixes", () => {
  it("opens the workspace of an old prefix, and the project by its old display ID", async () => {
    const nodes = one(ws("S1", "W1", "WEB", ["HH"]));
    expect(await resolveRoute(wsRoute("HH"), nodes, itemsIn("S1"), transfers)).toEqual({
      name: "workspace",
      storeId: "S1",
      workspaceId: "W1",
      tab: "overview",
    });
    expect(await resolveRoute(pRoute("HH-P1"), nodes, itemsIn("S1"), transfers)).toEqual({
      name: "project",
      storeId: "S1",
      projectId: "W1-P1",
      tab: "tree",
    });
  });

  it("prefers the workspace that uses the prefix now over the one that used it before", async () => {
    const nodes = one(ws("S1", "W1", "WEB", ["HH"]), ws("S1", "W2", "HH", []));
    expect(await resolveRoute(wsRoute("HH"), nodes, itemsIn("S1"), transfers)).toEqual({
      name: "workspace",
      storeId: "S1",
      workspaceId: "W2",
      tab: "overview",
    });
    expect(await resolveRoute(pRoute("HH-P1"), nodes, itemsIn("S1"), transfers)).toEqual({
      name: "project",
      storeId: "S1",
      projectId: "W2-P1",
      tab: "tree",
    });
  });

  it("prefers the current prefix in another Store over an old prefix in the named Store", async () => {
    const nodes = [
      storeOf("S1", "work", ws("S1", "W1", "WEB", ["HH"])),
      storeOf("S2", "other", ws("S2", "W2", "HH", [])),
    ];
    expect(await resolveRoute(wsRoute("HH"), nodes, itemsIn("S1"), transfers)).toMatchObject({
      storeId: "S2",
      workspaceId: "W2",
    });
    expect(await resolveRoute(pRoute("HH-P1"), nodes, itemsIn("S1"), transfers)).toMatchObject({
      storeId: "S2",
      projectId: "W2-P1",
    });
  });

  it("is home for a prefix that no workspace has used, and for an old-prefix project number that does not exist", async () => {
    const nodes = one(ws("S1", "W1", "WEB", ["HH"]));
    expect(await resolveRoute(wsRoute("ZZ"), nodes, itemsIn("S1"), transfers)).toEqual({
      name: "home",
    });
    expect(await resolveRoute(pRoute("HH-P9"), nodes, itemsIn("S1"), transfers)).toEqual({
      name: "home",
    });
  });
});
