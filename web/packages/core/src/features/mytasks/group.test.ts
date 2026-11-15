import { describe, expect, it } from "vitest";
import type { StoreNode } from "../catalog/catalog";
import { result } from "../search/fixtures.test-util";
import type { SearchRow } from "../search/types";
import { groupMyTasks } from "./group";

const base = result("x").rows[0] as SearchRow;

interface Over {
  id: string;
  end?: string;
  storeId?: string;
  projectId?: string;
  projectTitle?: string;
  workspaceId?: string;
}
const row = (o: Over): SearchRow => ({
  ...base,
  itemId: o.id,
  end: o.end ?? "",
  storeId: o.storeId ?? "e2e",
  projectId: o.projectId ?? "p1",
  workspaceId: o.workspaceId ?? "w1",
  project: { display: "HH-P1", title: o.projectTitle ?? "p" },
});

const ws = (id: string, name: string) =>
  ({ id, name, projects: [], oldPrefixes: [] }) as unknown as StoreNode["workspaces"][number];
const stores: StoreNode[] = [
  {
    id: "e2e",
    name: "e2e",
    color: "blue",
    readOnly: false,
    workspaces: [ws("w1", "WEB"), ws("w2", "Mobile")],
  },
  { id: "team", name: "team", color: "green", readOnly: false, workspaces: [ws("w1", "運営")] },
];
const ids = (rows: SearchRow[] | undefined) => (rows ?? []).map((r) => r.itemId);

describe("groupMyTasks", () => {
  it("due groups on sunday", () => {
    const rows = [
      row({ id: "a", end: "2026-11-14" }),
      row({ id: "b", end: "2026-11-15" }),
      row({ id: "c", end: "2026-11-16" }),
      row({ id: "d" }),
    ];
    const g = groupMyTasks(rows, "due", "2026-11-15", stores);
    expect(g.map((x) => x.label)).toEqual(["期限切れ", "今日", "来週以降", "期限なし"]);
    expect(g.map((x) => x.warn)).toEqual([true, true, false, false]);
    expect(g.every((x) => x.store === null)).toBe(true);
  });

  it("due groups on monday", () => {
    const rows = [row({ id: "a", end: "2026-11-23" }), row({ id: "b", end: "2026-11-22" })];
    const g = groupMyTasks(rows, "due", "2026-11-16", stores);
    expect(g.map((x) => x.label)).toEqual(["今週", "来週以降"]);
    expect(ids(g[0]?.rows)).toEqual(["b"]);
    expect(ids(g[1]?.rows)).toEqual(["a"]);
  });

  it("due keeps server order on ties", () => {
    const rows = [
      row({ id: "a", end: "2026-11-20" }),
      row({ id: "b", end: "2026-11-18" }),
      row({ id: "c", end: "2026-11-20" }),
      row({ id: "d", end: "2026-11-20" }),
    ];
    const g = groupMyTasks(rows, "due", "2026-11-16", stores);
    expect(ids(g[0]?.rows)).toEqual(["b", "a", "c", "d"]);
  });

  it("project groups by title", () => {
    const rows = [
      row({ id: "1", projectId: "pb", projectTitle: "b", end: "2026-11-20" }),
      row({ id: "2", projectId: "pa", projectTitle: "a" }),
      row({ id: "3", projectId: "pb", projectTitle: "b" }),
      row({ id: "4", projectId: "pb", projectTitle: "b", end: "2026-11-18" }),
    ];
    const g = groupMyTasks(rows, "project", "2026-11-16", stores);
    expect(g.map((x) => x.label)).toEqual(["a", "b"]);
    expect(ids(g[1]?.rows)).toEqual(["4", "1", "3"]);
  });

  it("same project name stays apart", () => {
    const rows = [
      row({ id: "1", storeId: "team", projectId: "p9", projectTitle: "運営" }),
      row({ id: "2", storeId: "e2e", projectId: "p9", projectTitle: "運営" }),
    ];
    const g = groupMyTasks(rows, "project", "2026-11-16", stores);
    expect(g.map((x) => x.label)).toEqual(["運営", "運営"]);
    expect(ids(g[0]?.rows)).toEqual(["2"]);
  });

  it("workspace groups carry the store", () => {
    const rows = [
      row({ id: "1", storeId: "team", workspaceId: "w1" }),
      row({ id: "2", storeId: "e2e", workspaceId: "w2" }),
      row({ id: "3", storeId: "e2e", workspaceId: "w1" }),
    ];
    const g = groupMyTasks(rows, "workspace", "2026-11-16", stores);
    expect(g.map((x) => x.label)).toEqual(["Mobile", "WEB", "運営"]);
    expect(g[0]?.store).toEqual({ name: "e2e", color: "blue" });
    expect(g[2]?.store).toEqual({ name: "team", color: "green" });
  });

  it("same workspace name follows store order", () => {
    const two: StoreNode[] = [
      { ...(stores[0] as StoreNode), workspaces: [ws("w1", "同名")] },
      { ...(stores[1] as StoreNode), workspaces: [ws("w1", "同名")] },
    ];
    const rows = [row({ id: "1", storeId: "team" }), row({ id: "2", storeId: "e2e" })];
    const g = groupMyTasks(rows, "workspace", "2026-11-16", two);
    expect(g.map((x) => x.store?.name)).toEqual(["e2e", "team"]);
  });
});
