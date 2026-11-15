// ツリーの書き込み(treeWrites.ts)のテストの材料。
import { vi } from "vitest";
import type { Item } from "../../model/item";
import type { MoveCandidate } from "../../model/itemAction";
import { item } from "./fixtures.test-util";
import { settings } from "./settings.test-util";
import { visibleNeighbors } from "./siblings";
import { createTreeWrites, type TreeWritesDeps } from "./treeWrites";

// 木: HH-1(Epic) ─ HH-2・HH-3・HH-4・HH-5(Task) / HH-6(Epic)。並べ替えのたびに読み直した並びを rows に入れる。
export const e1 = item("S1", "P1", 1, "Epic", null);
export const e6 = item("S1", "P1", 6, "Epic", null);
export const kids: Item[] = [2, 3, 4, 5].map((n) => item("S1", "P1", n, "Task", e1.id));
export const [a, b, c, d] = kids as [Item, Item, Item, Item];

export const candidate = (it: Item): MoveCandidate => ({
  kind: "item",
  id: it.id,
  display: `HH-${it.number}`,
  title: it.title,
  projectId: it.projectId,
  projectTitle: "ログイン",
});

// setup の reorder は、itemMenuHost.reorder と同じ約束の偽物: 見えている兄弟を基準に 1 件だけ動かし、読み直して、
// 動かせたら true(基準の兄弟がない・閲覧のみは false)。
export function setup(over: Partial<TreeWritesDeps> = {}, readOnly = false) {
  let order = [...kids];
  let rows: Item[] = [e1, ...order, e6];
  const log: string[] = [];
  const toasts = { show: vi.fn() };
  const deps: TreeWritesDeps = {
    screen: () => ({ storeId: "S1", readOnly, settings, rows }),
    reorder: async (it, seen, dir) => {
      const { prev, next } = visibleNeighbors(seen, it.id);
      const anchor = dir === "up" ? prev : next;
      if (!anchor) return false;
      log.push(`reorder ${it.id} ${anchor.id} ${dir === "down"}`);
      order = order.filter((i) => i.id !== it.id);
      order.splice(order.findIndex((i) => i.id === anchor.id) + (dir === "down" ? 1 : 0), 0, it);
      rows = [e1, ...order, e6];
      log.push("refresh");
      return true;
    },
    moveCandidates: async (_s, id) => {
      log.push(`candidates ${id}`);
      return [candidate(e6)];
    },
    moveTo: async (it, cand) => {
      log.push(`moveTo ${it.id} ${cand.id}`);
      return true;
    },
    setStatus: async (it, status) => {
      log.push(`status ${it.id} ${status}`);
    },
    afterChange: async () => {
      log.push("refresh");
    },
    settled: async () => {},
    expand: (id) => {
      log.push(`expand ${id}`);
    },
    toasts,
    ...over,
  };
  return { writes: createTreeWrites(deps), log, toasts, order: () => order.map((i) => i.id) };
}
