import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { item } from "./fixtures.test-util";
import { menuAt } from "./menuHost";
import { ctx, epic, ro, rowCtx, setup, step, t3, t5, t7 } from "./menuHost.test-util";

describe("menuAt", () => {
  it("puts the menu under the button", () => {
    expect(menuAt({ left: 10, bottom: 20 })).toEqual({ x: 10, y: 24 });
  });
});

describe("createItemMenuHost", () => {
  it("opens with the entries of the item and its place among the visible rows", async () => {
    const { host } = await setup();
    host.open(t3, ctx, 5, 6);
    const s = get(host.state);
    expect([s?.x, s?.y]).toEqual([5, 6]);
    expect(s?.entries.map((e) => [e.id, e.disabled])).toEqual([
      ["move-up", false],
      ["move-down", false],
      ["move", false],
      ["transfer", false],
      ["change-kind", false],
      ["duplicate", false],
      ["save-as-template", false],
      ["copy-id", false],
      ["delete", false],
    ]);
    // 見えている兄弟の先頭は「上へ移動」を、末尾は「下へ移動」を無効にする。
    host.open(t5, ctx, 0, 0);
    expect(get(host.state)?.entries[0]).toMatchObject({ id: "move-up", disabled: true });
    host.open(t7, ctx, 0, 0);
    expect(get(host.state)?.entries[1]).toMatchObject({ id: "move-down", disabled: true });
  });

  it("puts open and add-child at the head of a tree-row menu", async () => {
    const { host } = await setup();
    host.open(t3, rowCtx, 0, 0);
    expect(
      get(host.state)
        ?.entries.slice(0, 3)
        .map((e) => [e.id, e.label, e.disabled]),
    ).toEqual([
      ["open", "詳細を開く", false],
      ["add-child", "Step を追加", false],
      ["move-up", "上へ移動", false],
    ]);
    // Step の行には「{種類} を追加」を出さない。
    host.open(step, rowCtx, 0, 0);
    expect(
      get(host.state)
        ?.entries.slice(0, 2)
        .map((e) => e.id),
    ).toEqual(["open", "move-up"]);
    // 閲覧のみの Store の行では「{種類} を追加」は無効、「詳細を開く」は使える。
    host.open(ro, { rows: [ro], children: [], row: true }, 0, 0);
    const entries = get(host.state)?.entries ?? [];
    expect(entries.filter((e) => !e.disabled).map((e) => e.id)).toEqual(["open", "copy-id"]);
    expect(entries.find((e) => e.id === "add-child")).toMatchObject({
      disabled: true,
      hint: "閲覧のみの Store です",
    });
  });

  it("opens the detail panel and the create dialog from a tree-row menu", async () => {
    const { host, log } = await setup();
    host.open(t3, rowCtx, 0, 0);
    await host.select("open");
    expect(get(host.state)).toBeNull();
    host.open(epic, rowCtx, 0, 0);
    await host.select("add-child");
    expect(log).toEqual([
      `open S1/${t3.id}`,
      `add ${JSON.stringify({ workspaceId: "S1-W1", projectId: "S1-P1", kind: "Story", parentId: epic.id })}`,
    ]);
  });

  it("has no open or add-child in the detail-panel menu", async () => {
    const { host, log } = await setup();
    host.open(t3, ctx, 0, 0);
    await host.select("open");
    await host.select("add-child");
    expect(log).toEqual([]);
    expect(get(host.state)).not.toBeNull();
  });

  it("disables the writes in a read-only Store, and the Epic items", async () => {
    const { host } = await setup();
    host.open(ro, { rows: [ro], children: [], row: false }, 0, 0);
    const entries = get(host.state)?.entries ?? [];
    expect(entries.filter((e) => !e.disabled).map((e) => e.id)).toEqual(["copy-id"]);
    expect(entries.find((e) => e.id === "delete")?.hint).toBe("閲覧のみの Store です");
    host.open(epic, ctx, 0, 0);
    expect(get(host.state)?.entries.find((e) => e.id === "change-kind")).toMatchObject({
      disabled: true,
      hint: "Epic の種類は変更できません",
    });
  });

  it("does nothing for a disabled entry or an entry that is not in the menu", async () => {
    const { host, log } = await setup();
    host.open(t5, ctx, 0, 0);
    await host.select("move-up");
    expect(log).toEqual([]);
    expect(get(host.state)).not.toBeNull();
    host.close();
    await host.select("delete");
    expect(log).toEqual([]);
  });

  it("does not open for an item outside the catalog", async () => {
    const { host } = await setup();
    host.open(item("S9", "S9-P1", 1, "Task", null), ctx, 0, 0);
    expect(get(host.state)).toBeNull();
  });

  it("opens the transfer dialog for the item when 'transfer' is selected", async () => {
    const { host, log } = await setup();
    host.open(t3, ctx, 10, 20);
    await host.select("transfer");
    expect(log).toEqual(["transfer S1-I3"]);
  });

  it("does not run 'transfer' in a read-only Store", async () => {
    const { host, log } = await setup();
    host.open(ro, ctx, 10, 20);
    await host.select("transfer");
    expect(log).toEqual([]);
  });

  it("hands move, change kind, duplicate, copy ID and delete to their controllers", async () => {
    const { host, log } = await setup();
    host.open(t3, ctx, 7, 9);
    await host.select("move");
    host.open(t3, ctx, 7, 9);
    await host.select("change-kind");
    host.open(t3, ctx, 7, 9);
    await host.select("duplicate");
    host.open(t3, ctx, 7, 9);
    await host.select("copy-id");
    host.open(t3, ctx, 7, 9);
    await host.select("delete");
    expect(log).toEqual([
      `move ${t3.id} 7,9`,
      `kind ${t3.id} 7,9`,
      `duplicate ${t3.id}`,
      "copy HH-3",
      `delete ${t3.id}`,
    ]);
  });

  it("opens the template dialog for the item, its Store and its children", async () => {
    const { host } = await setup();
    host.open(t3, ctx, 0, 0);
    await host.select("save-as-template");
    const t = get(host.template);
    expect(t?.item).toBe(t3);
    expect(t?.store.id).toBe("S1");
    expect(t?.children).toEqual([step]);
    host.closeTemplate();
    expect(get(host.template)).toBeNull();
  });

  it("copies the ID in a read-only Store too", async () => {
    const { host, log } = await setup();
    host.open(ro, { rows: [ro], children: [], row: false }, 0, 0);
    await host.select("copy-id");
    expect(log).toEqual(["copy HH-1"]);
  });
});
