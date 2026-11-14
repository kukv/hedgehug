import { beforeEach, describe, expect, it } from "vitest";
import { type FakeTransfers, fakeTransfers } from "../transfers/transfers.test-util";
import { resolveRoute } from "./resolve";
import { itemsIn, stores } from "./resolve.test-util";

let transfers: FakeTransfers;
beforeEach(() => {
  transfers = fakeTransfers();
});

const wRoute = (prefix: string, store = "work") =>
  ({ name: "workspace", store, prefix, tab: "sprints" }) as const;
const movedFrom = {
  old: "XX",
  at: "2026-11-13T00:05:00.000Z",
  display: "team",
  current: "team",
};

// ワークスペースのアドレスは、一覧(アドレスの Store のいまの接頭辞・旧い接頭辞)で見つからないときだけ、
// サーバーの locateWorkspace に聞く(B5c-13。プロジェクトの A5b-24 と同じ)。
describe("resolveRoute of a workspace not in the catalog", () => {
  it("does not ask locateWorkspace for a workspace the named Store has", async () => {
    expect(await resolveRoute(wRoute("HH"), await stores(), itemsIn("S1"), transfers)).toEqual({
      name: "workspace",
      storeId: "S1",
      workspaceId: "S1-W1",
      tab: "sprints",
    });
    expect(transfers.calls).toEqual([]);
  });

  it("asks locateWorkspace with the Store name and the prefix, and opens what it finds on the same tab with movedFrom", async () => {
    transfers.locatedWorkspace = {
      kind: "found",
      storeId: "S2",
      workspaceId: "S2-W9",
      movedFrom,
    };
    const r = await resolveRoute(wRoute("XX"), await stores(), itemsIn("S1"), transfers);
    expect(transfers.calls).toEqual(["locateWorkspace work XX"]);
    expect(r).toEqual({
      name: "workspace",
      storeId: "S2",
      workspaceId: "S2-W9",
      tab: "sprints",
      movedFrom,
    });
  });

  it("does not open another Store's workspace of the same prefix from the catalog", async () => {
    transfers.missing.add("HH");
    expect(
      await resolveRoute(wRoute("HH", "nope"), await stores(), itemsIn("S1"), transfers),
    ).toEqual({ name: "home" });
    expect(transfers.calls).toEqual(["locateWorkspace nope HH"]);
  });

  it("is home with the gone reason when the server says it is gone", async () => {
    transfers.locatedWorkspace = {
      kind: "gone",
      reason: "deleted",
      old: "XX",
      at: movedFrom.at,
      newDisplay: "team",
    };
    expect(await resolveRoute(wRoute("XX"), await stores(), itemsIn("S1"), transfers)).toEqual({
      name: "home",
      gone: { reason: "deleted", old: "XX", at: movedFrom.at, newDisplay: "team" },
    });
  });

  it("is home when the server does not know it, and does not ask when askServer is false", async () => {
    transfers.missing.add("XX");
    expect(await resolveRoute(wRoute("XX"), await stores(), itemsIn("S1"), transfers)).toEqual({
      name: "home",
    });
    transfers.calls.length = 0;
    expect(
      await resolveRoute(wRoute("XX"), await stores(), itemsIn("S1"), transfers, false),
    ).toEqual({ name: "home" });
    expect(transfers.calls).toEqual([]);
  });
});
