import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { gate, item } from "./fixtures.test-util";
import { createItemMenuHost, menuAt, reorderFailedMessage } from "./menuHost";

// HH-1(Epic)の下に、見えている順で HH-5・HH-3・HH-7 の 3 つの Task。HH-3 の下に Step の HH-8。
const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
const t5 = item("S1", "S1-P1", 5, "Task", epic.id, "画面");
const t3 = item("S1", "S1-P1", 3, "Task", epic.id, "決済");
const t7 = item("S1", "S1-P1", 7, "Task", epic.id, "検索");
const step = item("S1", "S1-P1", 8, "Step", t3.id, "設計");
const ro = item("S2", "S2-P1", 1, "Task", null, "閲覧のみ");
const ctx = { rows: [epic, t5, t3, step, t7], children: [step], row: false };
const rowCtx = { ...ctx, row: true };

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const log: string[] = [];
  let failure: unknown = null;
  let hold: Promise<void> | null = null;
  const host = createItemMenuHost({
    items: {
      reorder: async (_s, id, anchor, after) => {
        log.push(`reorder ${id} ${after ? "after" : "before"} ${anchor}`);
        if (hold) await hold;
        if (failure) throw failure;
      },
    },
    catalog,
    toasts,
    refresh: { afterChange: async () => void log.push("change") },
    detail: { open: async (store, id) => void log.push(`open ${store}/${id}`) },
    dialog: { open: async (c) => void log.push(`add ${JSON.stringify(c)}`) },
    remove: { open: async (it: Item) => void log.push(`delete ${it.id}`) },
    move: { open: async (it: Item, x, y) => void log.push(`move ${it.id} ${x},${y}`) },
    kind: { open: (it: Item, x, y) => void log.push(`kind ${it.id} ${x},${y}`) },
    duplicate: { run: async (it: Item) => void log.push(`duplicate ${it.id}`) },
    copyId: async (id) => void log.push(`copy ${id}`),
  });
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return {
    host,
    log,
    messages,
    fail: (e: unknown) => {
      failure = e;
    },
    hold: (p: Promise<void> | null) => {
      hold = p;
    },
  };
}

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

  it("moves up before the visible previous sibling, skipping rows in between", async () => {
    const { host, log, messages } = await setup();
    host.open(t7, ctx, 0, 0);
    await host.select("move-up");
    expect(log).toEqual([`reorder ${t7.id} before ${t3.id}`, "change"]);
    expect(get(host.state)).toBeNull();
    expect(messages()).toEqual([]);
  });

  it("moves down after the visible next sibling", async () => {
    const { host, log } = await setup();
    host.open(t5, ctx, 0, 0);
    await host.select("move-down");
    expect(log).toEqual([`reorder ${t5.id} after ${t3.id}`, "change"]);
  });

  it("reorders without the menu, saying whether it moved (the Alt+arrow keys)", async () => {
    const { host, log, fail } = await setup();
    expect(await host.reorder(t3, ctx.rows, "up")).toBe(true);
    expect(await host.reorder(t3, ctx.rows, "down")).toBe(true);
    expect(log).toEqual([
      `reorder ${t3.id} before ${t5.id}`,
      "change",
      `reorder ${t3.id} after ${t7.id}`,
      "change",
    ]);
    // 見えている兄弟の端と閲覧のみの Store は、何も呼ばずに false。断りも false で、読み直さない。
    expect(await host.reorder(t5, ctx.rows, "up")).toBe(false);
    expect(await host.reorder(ro, [ro, item("S2", "S2-P1", 2, "Task", null)], "down")).toBe(false);
    expect(log).toHaveLength(4);
    fail(new Refused("動かせません"));
    expect(await host.reorder(t3, ctx.rows, "up")).toBe(false);
    expect(log).toHaveLength(5);
    expect(log.at(-1)).toBe(`reorder ${t3.id} before ${t5.id}`);
  });

  it("does not start a second reorder while one is running", async () => {
    const { host, log, hold } = await setup();
    const g = gate();
    hold(g.wait);
    const first = host.reorder(t3, ctx.rows, "up");
    expect(await host.reorder(t7, ctx.rows, "up")).toBe(false);
    g.release();
    expect(await first).toBe(true);
    expect(log).toEqual([`reorder ${t3.id} before ${t5.id}`, "change"]);
  });

  it("keeps a menu opened while a reorder is running", async () => {
    const { host, hold } = await setup();
    const g = gate();
    hold(g.wait);
    host.open(t7, ctx, 0, 0);
    const running = host.select("move-up");
    host.open(t5, ctx, 3, 4);
    g.release();
    await running;
    expect(get(host.state)).toMatchObject({ item: t5, x: 3, y: 4 });
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

  it("shows a refusal of the reorder as an error toast and does not reload", async () => {
    const { host, log, messages, fail } = await setup();
    fail(new Refused("動かせません"));
    host.open(t7, ctx, 0, 0);
    await host.select("move-up");
    expect(messages()).toEqual(["!動かせません"]);
    expect(log).toEqual([`reorder ${t7.id} before ${t3.id}`]);
  });

  it("shows another failure of the reorder with its own words, and reloads when the item is gone", async () => {
    const { host, log, messages, fail } = await setup();
    fail(new Error("network"));
    host.open(t7, ctx, 0, 0);
    await host.select("move-up");
    expect(messages()).toEqual([`!${reorderFailedMessage}`]);
    expect(reorderFailedMessage).toBe("順番を変更できませんでした");
    fail(new NotFound("gone"));
    host.open(t7, ctx, 0, 0);
    await host.select("move-up");
    expect(messages()).toHaveLength(1);
    expect(log.at(-1)).toBe("change");
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
