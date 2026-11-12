import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog, type StoreNode } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import {
  childKindOf,
  parentCandidates,
  parentMissing,
  pickParent,
  startValues,
} from "./createDefaults";
import { item } from "./fixtures.test-util";

async function stores(): Promise<StoreNode[]> {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  return get(catalog.stores);
}

describe("startValues", () => {
  it("takes the given place, skipping a read-only store's workspace", async () => {
    const s = await stores();
    expect(startValues(s, { workspaceId: "S1-W1", projectId: "S1-P2", kind: "Epic" }, {})).toEqual({
      ok: true,
      workspace: s[0]?.workspaces[0],
      projectId: "S1-P2",
      kind: "Epic",
      parentId: null,
    });
    const r = startValues(s, {}, { workspaceId: "S2-W1", projectId: "S2-P2" });
    expect(r).toMatchObject({ ok: true, projectId: "S1-P1", kind: "Task" });
  });

  it("uses the open project when it is in the workspace", async () => {
    const s = await stores();
    expect(startValues(s, {}, { workspaceId: "S1-W1", projectId: "S1-P2" })).toMatchObject({
      projectId: "S1-P2",
    });
  });

  it("refuses with the spec's words when nothing can be written", async () => {
    expect(startValues([], {}, {})).toEqual({
      ok: false,
      message: "先にワークスペースを作ってください",
    });
    const onlyRo = (await stores()).filter((x) => x.readOnly);
    expect(startValues(onlyRo, {}, {})).toEqual({
      ok: false,
      message: "アイテムを作成できる Store がありません(閲覧のみの Store だけです)",
    });
    const s = await stores();
    const empty = s.map((x) => ({
      ...x,
      workspaces: x.workspaces.map((w) => ({ ...w, projects: [] })),
    }));
    expect(startValues(empty, {}, {})).toMatchObject({
      ok: false,
      message: "Web にはプロジェクトがありません。先にプロジェクトを作ってください",
    });
  });
});

describe("parent candidates", () => {
  const epic = item("S1", "P", 1, "Epic", null);
  const task = item("S1", "P", 2, "Task", epic.id);
  const step = item("S1", "P", 3, "Step", task.id);

  it("are Epics for work items, work items for Steps, none for Epics", () => {
    const all = [epic, task, step];
    expect(parentCandidates(all, "Bug")).toEqual([epic]);
    expect(parentCandidates(all, "Step")).toEqual([task]);
    expect(parentCandidates(all, "Epic")).toEqual([]);
  });

  it("keeps the wanted parent if it can be one, else takes the first", () => {
    expect(pickParent([epic], epic.id)).toBe(epic.id);
    expect(pickParent([epic], task.id)).toBe(epic.id);
    expect(pickParent([], null)).toBeNull();
  });

  it("says what is missing", () => {
    expect(parentMissing("Step")).toBe(
      "このプロジェクトには Story・Task・Bug がありません。先に作ってください",
    );
    expect(parentMissing("Task")).toBe(
      "このプロジェクトには Epic がありません。先に作ってください",
    );
  });
});

describe("childKindOf", () => {
  it("is Story under an Epic, Step under work items, and none under a Step", () => {
    expect(childKindOf("Epic")).toBe("Story");
    expect(childKindOf("Bug")).toBe("Step");
    expect(childKindOf("Step")).toBeNull();
  });
});
