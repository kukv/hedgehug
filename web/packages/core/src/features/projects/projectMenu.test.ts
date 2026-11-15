import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import type { Project } from "../../model/project";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort, project } from "../catalog/fixtures.test-util";
import { readOnlyNote } from "../catalog/readonly";
import { createToasts } from "../toasts/toasts";
import { canMoveProject, createProjectMover, projectMenu } from "./projectMenu";
import { createRenameProjectDialog } from "./renameDialog";

async function setup(count = 3) {
  const catalog = createCatalog({
    ...fakeCatalogPort(),
    listProjects: async (storeId, workspaceId) =>
      Array.from({ length: count }, (_, i) => project(storeId, workspaceId, i + 1)),
  });
  await catalog.load();
  return catalog;
}

function nodes(catalog: Awaited<ReturnType<typeof setup>>, storeId: string) {
  const store = catalog.store(storeId);
  const workspace = store?.workspaces[0];
  if (!store || !workspace) throw new Error("fixture");
  return { store, workspace };
}

describe("projectMenu", () => {
  it("lists the items in order with their labels", async () => {
    const { store, workspace } = nodes(await setup(), "S1");
    const items = projectMenu(store, workspace, "S1-P2");
    expect(items.map((i) => [i.id, i.label])).toEqual([
      ["open", "開く"],
      ["add-epic", "Epic を追加"],
      ["up", "上へ移動"],
      ["down", "下へ移動"],
      ["rename", "名前を変更…"],
      ["delete", "削除…"],
    ]);
    expect(items.every((i) => !i.disabled && i.hint === undefined)).toBe(true);
  });

  it("disables everything but 開く in a read-only store, with the note", async () => {
    const { store, workspace } = nodes(await setup(), "S2");
    for (const i of projectMenu(store, workspace, "S2-P2")) {
      if (i.id === "open") {
        expect(i.disabled).toBe(false);
        expect(i.hint).toBeUndefined();
      } else {
        expect(i.disabled).toBe(true);
        expect(i.hint).toBe(readOnlyNote);
      }
    }
  });

  it("disables 上へ at the top and 下へ at the bottom, without a hint", async () => {
    const { store, workspace } = nodes(await setup(), "S1");
    const state = (id: string) =>
      Object.fromEntries(projectMenu(store, workspace, id).map((i) => [i.id, i]));
    expect(state("S1-P1").up).toMatchObject({ disabled: true });
    expect(state("S1-P1").up?.hint).toBeUndefined();
    expect(state("S1-P1").down).toMatchObject({ disabled: false });
    expect(state("S1-P3").down).toMatchObject({ disabled: true });
    expect(state("S1-P3").up).toMatchObject({ disabled: false });
  });

  it("disables both moves for a single project", async () => {
    const { store, workspace } = nodes(await setup(1), "S1");
    expect(canMoveProject(workspace, "S1-P1", "up")).toBe(false);
    expect(canMoveProject(workspace, "S1-P1", "down")).toBe(false);
    expect(canMoveProject(workspace, "nope", "down")).toBe(false);
    const disabled = projectMenu(store, workspace, "S1-P1")
      .filter((i) => i.disabled)
      .map((i) => i.id);
    expect(disabled).toEqual(["up", "down"]);
  });
});

describe("createProjectMover", () => {
  function mover(catalog: Awaited<ReturnType<typeof setup>>, fail?: string) {
    const calls: string[] = [];
    const toasts = createToasts({ set: () => 0, clear: () => undefined });
    let loads = 0;
    const m = createProjectMover({
      catalog: {
        ...catalog,
        load: async () => {
          loads += 1;
        },
      },
      port: {
        move: async (s, p, d) => {
          calls.push(`move ${s} ${p} ${d}`);
          if (fail) throw new Refused(fail);
        },
      },
      toasts,
    });
    return { m, calls, toasts, loads: () => loads };
  }

  it("moves without a toast and reloads the catalog", async () => {
    const t = mover(await setup());
    await t.m.move("S1", "S1-P1", "down");
    expect(t.calls).toEqual(["move S1 S1-P1 down"]);
    expect(get(t.toasts.list)).toEqual([]);
    expect(t.loads()).toBe(1);
  });

  it("shows a refusal as an error toast and still reloads", async () => {
    const t = mover(await setup(), "並びを変えられませんでした");
    await t.m.move("S1", "S1-P1", "down");
    expect(get(t.toasts.list).map((x) => [x.text, x.error])).toEqual([
      ["並びを変えられませんでした", true],
    ]);
    expect(t.loads()).toBe(1);
  });
});

describe("createRenameProjectDialog", () => {
  const p: Project = project("S1", "S1-W1", 1);

  it("closes on success and keeps open with the refusal text otherwise", async () => {
    let answer: string | null = "タイトルを入力してください";
    const asked: string[] = [];
    const d = createRenameProjectDialog({
      rename: async (_p, title) => {
        asked.push(title);
        return answer;
      },
    });
    expect(get(d.state)).toBeNull();
    d.open(p);
    expect(get(d.state)).toEqual({ project: p, error: null });
    await d.save("");
    expect(get(d.state)).toEqual({ project: p, error: "タイトルを入力してください" });
    answer = null;
    await d.save("新しい名前");
    expect(get(d.state)).toBeNull();
    expect(asked).toEqual(["", "新しい名前"]);
  });

  it("ignores a second save while the first is waiting, so a refusal cannot reopen it", async () => {
    let release: (m: string | null) => void = () => {};
    let calls = 0;
    const d = createRenameProjectDialog({
      rename: (_p, _t) => {
        calls += 1;
        // 1 回目は返事を止める。2 回目(あれば)は、同じ値は送り済みとして null で終わる。
        if (calls > 1) return Promise.resolve(null);
        return new Promise<string | null>((r) => {
          release = r;
        });
      },
    });
    d.open(p);
    const first = d.save("x");
    await d.save("x");
    expect(calls).toBe(1);
    expect(get(d.state)).not.toBeNull();
    release("断られました");
    await first;
    expect(get(d.state)).toEqual({ project: p, error: "断られました" });
  });

  it("does not change the state with a reply that arrives after close or reopen", async () => {
    const pending: ((m: string | null) => void)[] = [];
    const d = createRenameProjectDialog({
      rename: () => new Promise<string | null>((r) => pending.push(r)),
    });
    d.open(p);
    const closed = d.save("a");
    d.close();
    pending[0]?.("断られました");
    await closed;
    expect(get(d.state)).toBeNull();

    d.open(p);
    const reopened = d.save("b");
    d.open(p);
    pending[1]?.("断られました");
    await reopened;
    expect(get(d.state)).toEqual({ project: p, error: null });
  });

  it("close closes", () => {
    const d = createRenameProjectDialog({ rename: async () => "x" });
    d.open(p);
    d.close();
    expect(get(d.state)).toBeNull();
  });
});
