import type { Item } from "../../model/item";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { item } from "./fixtures.test-util";
import { createItemMenuHost } from "./menuHost";

// アイテムのメニューの口(menuHost)のテストの準備。依存はどれも、呼ばれた順を log に残す偽物。

// HH-1(Epic)の下に、見えている順で HH-5・HH-3・HH-7 の 3 つの Task。HH-3 の下に Step の HH-8。
export const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
export const t5 = item("S1", "S1-P1", 5, "Task", epic.id, "画面");
export const t3 = item("S1", "S1-P1", 3, "Task", epic.id, "決済");
export const t7 = item("S1", "S1-P1", 7, "Task", epic.id, "検索");
export const step = item("S1", "S1-P1", 8, "Step", t3.id, "設計");
export const ro = item("S2", "S2-P1", 1, "Task", null, "閲覧のみ");
export const ctx = { rows: [epic, t5, t3, step, t7], children: [step], row: false };
export const rowCtx = { ...ctx, row: true };

export async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const log: string[] = [];
  let failure: unknown = null;
  let hold: Promise<void> | null = null;
  let holdRefresh: Promise<void> | null = null;
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
    refresh: {
      afterChange: async () => {
        log.push("change");
        if (holdRefresh) await holdRefresh;
      },
    },
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
    holdRefresh: (p: Promise<void> | null) => {
      holdRefresh = p;
    },
  };
}
