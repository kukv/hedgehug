import { beforeEach, describe, expect, it } from "vitest";
import type { StoreNode, WorkspaceNode } from "../catalog/catalog";
import { type FakeTransfers, fakeTransfers } from "../transfers/transfers.test-util";
import { resolveRoute } from "./resolve";
import { itemsIn, stores } from "./resolve.test-util";

let transfers: FakeTransfers;
beforeEach(() => {
  transfers = fakeTransfers();
});

const pRoute = (project: string) =>
  ({ name: "project", store: "work", project, tab: "tree" }) as const;
const movedFrom = {
  old: "HH-P7",
  at: "2026-11-13T00:05:00.000Z",
  display: "MB-P2",
  current: "MB-P2",
};

// プロジェクトのアドレスは、一覧(いまの接頭辞・旧い接頭辞)で見つからないときだけ、サーバーの locateProject に聞く
// (A5b-17・A5b-24)。
describe("resolveRoute of a project not in the catalog", () => {
  it("does not ask locateProject for a project the catalog has, by the current or an old prefix", async () => {
    expect(await resolveRoute(pRoute("HH-P2"), await stores(), itemsIn("S1"), transfers)).toEqual({
      name: "project",
      storeId: "S1",
      projectId: "S1-P2",
      tab: "tree",
    });
    const old = [
      {
        id: "S1",
        name: "work",
        color: "teal",
        readOnly: false,
        workspaces: [
          {
            id: "W1",
            storeId: "S1",
            name: "W1",
            prefix: "WEB",
            oldPrefixes: ["HH"],
            projects: [{ id: "W1-P1", number: 1, displayId: "WEB-P1" }],
          } as WorkspaceNode,
        ],
      } as StoreNode,
    ];
    expect(await resolveRoute(pRoute("HH-P1"), old, itemsIn("S1"), transfers)).toMatchObject({
      projectId: "W1-P1",
    });
    expect(transfers.calls).toEqual([]);
  });

  it("asks locateProject with the Store name and the ID, and opens what it finds on the same tab with movedFrom", async () => {
    transfers.locatedProject = { kind: "found", storeId: "S2", projectId: "S2-P9", movedFrom };
    const r = await resolveRoute(pRoute("HH-P7"), await stores(), itemsIn("S1"), transfers);
    expect(transfers.calls).toEqual(["locateProject work HH-P7"]);
    expect(r).toEqual({
      name: "project",
      storeId: "S2",
      projectId: "S2-P9",
      tab: "tree",
      movedFrom,
    });
  });

  it("opens a found project without movedFrom when locateProject gives none (the current ID)", async () => {
    transfers.locatedProject = { kind: "found", storeId: "S1", projectId: "S1-P9" };
    expect(await resolveRoute(pRoute("MB-P2"), await stores(), itemsIn("S1"), transfers)).toEqual({
      name: "project",
      storeId: "S1",
      projectId: "S1-P9",
      tab: "tree",
    });
  });

  it("gives the Gone of a gone project with home", async () => {
    const gone = {
      reason: "deleted",
      old: "HH-P7",
      at: "2026-11-13T00:05:00.000Z",
      newDisplay: "MB-P2",
    } as const;
    transfers.locatedProject = { kind: "gone", ...gone };
    expect(await resolveRoute(pRoute("HH-P7"), await stores(), itemsIn("S1"), transfers)).toEqual({
      name: "home",
      gone,
    });
  });

  it("is plain home when locateProject has nothing (NotFound), and does not ask for an unreadable ID", async () => {
    transfers.missing.add("HH-P9");
    const s = await stores();
    expect(await resolveRoute(pRoute("HH-P9"), s, itemsIn("S1"), transfers)).toEqual({
      name: "home",
    });
    transfers.calls.length = 0;
    expect(await resolveRoute(pRoute("HH-9"), s, itemsIn("S1"), transfers)).toEqual({
      name: "home",
    });
    expect(transfers.calls).toEqual([]);
  });

  it("passes on errors of locateProject other than not found", async () => {
    transfers.failNext();
    await expect(
      resolveRoute(pRoute("HH-P7"), await stores(), itemsIn("S1"), transfers),
    ).rejects.toThrow("network");
  });

  // 出していたプロジェクトのアドレスを読み直すときは、サーバーに聞かない(A5b-19)。
  it("does not ask locateProject when told not to", async () => {
    transfers.locatedProject = { kind: "found", storeId: "S1", projectId: "S1-P2", movedFrom };
    expect(
      await resolveRoute(pRoute("HH-P7"), await stores(), itemsIn("S1"), transfers, false),
    ).toEqual({ name: "home" });
    expect(transfers.calls).toEqual([]);
  });
});
