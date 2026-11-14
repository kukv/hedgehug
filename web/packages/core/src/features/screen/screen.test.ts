import { describe, expect, it } from "vitest";
import type { HistoryPort } from "../../port/history";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, flush, item } from "../items/fixtures.test-util";
import { createNavigation } from "../router/navigation";
import { formatRoute } from "../router/route";
import { createToasts } from "../toasts/toasts";
import { fakeTransfers } from "../transfers/transfers.test-util";
import { createScreen } from "./screen";

function memoryHistory(start: string): HistoryPort & { paths: string[] } {
  const paths = [start];
  return {
    paths,
    path: () => paths[paths.length - 1] ?? "/",
    push: (p) => void paths.push(p),
    replace: (p) => {
      paths[paths.length - 1] = p;
    },
    onPop: () => () => {},
  };
}

async function setup(start: string, begin = true) {
  const history = memoryHistory(start);
  const navigation = createNavigation(history);
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const items = fakeItemsPort([item("S1", "S1-P1", 12, "Task", null)]);
  const opened: unknown[][] = [];
  const transfers = fakeTransfers();
  const toasts = createToasts({ set: () => 0, clear: () => undefined });
  const screen = createScreen({
    navigation,
    catalog,
    items,
    transfers,
    toasts,
    now: () => new Date(2026, 10, 14, 12, 0),
    openItem: (...a) => void opened.push(a),
  });
  if (begin) {
    screen.start();
    await flush();
  }
  return { screen, navigation, history, opened, catalog, items, transfers, toasts };
}

describe("createScreen", () => {
  it("is null before the first resolution and then follows the address", async () => {
    const { screen, navigation } = await setup("/s/work/p/HH-P2/overview", false);
    expect(get(screen.current)).toBeNull();
    screen.start();
    await flush();
    expect(get(screen.current)).toEqual({
      name: "project",
      storeId: "S1",
      projectId: "S1-P2",
      tab: "overview",
    });
    navigation.go({ name: "workspace", store: "work", prefix: "HH", tab: "overview" });
    await flush();
    expect(get(screen.current)).toEqual({
      name: "workspace",
      storeId: "S1",
      workspaceId: "S1-W1",
      tab: "overview",
    });
  });

  it("does not emit current again for the same resolution", async () => {
    const { screen, catalog } = await setup("/s/work/p/HH-P2/overview");
    const seen: unknown[] = [];
    screen.current.subscribe((r) => void seen.push(r));
    await catalog.load();
    await flush();
    expect(seen).toHaveLength(1);
  });

  it("opens an item entry as its project's tree with the panel, without pushing history", async () => {
    const { screen, history, opened } = await setup("/s/work/i/HH-12");
    await flush();
    expect(history.paths).toEqual(["/s/work/p/HH-P1/tree"]);
    expect(get(screen.current)).toMatchObject({ name: "project", projectId: "S1-P1", tab: "tree" });
    expect(opened).toEqual([["S1", "S1-I12", undefined]]);
  });

  // 一覧の読み直し(画面への反映)が、アイテムを探すより短い間隔で続いても、アイテムの入口は開く。
  // 新しい解決を始めるたびに前の結果を捨てると、いつまでも開かない。
  it("opens an item entry even while the catalog keeps reloading during the lookup", async () => {
    const { screen, history, opened, catalog, items } = await setup("/s/work/i/HH-12", false);
    const find = items.findByNumber.bind(items);
    const pending: (() => void)[] = [];
    items.findByNumber = (...a) =>
      new Promise((resolve) => {
        pending.push(() => void find(...a).then(resolve));
      });
    screen.start();
    await flush();
    for (let i = 0; i < 3; i++) {
      await catalog.load();
      await flush();
      pending.shift()?.();
      await flush();
    }
    expect(history.paths).toEqual(["/s/work/p/HH-P1/tree"]);
    expect(opened).toEqual([["S1", "S1-I12", undefined]]);
  });

  it("shows home for an address it cannot resolve", async () => {
    const { screen, transfers } = await setup("/s/work/p/HH-P9/overview", false);
    transfers.missing.add("HH-P9");
    screen.start();
    await flush();
    expect(get(screen.current)).toEqual({ name: "home" });
  });

  it("opens the found item with its moved-from note after replacing the address with the project tree", async () => {
    const { navigation, opened, transfers } = await setup("/");
    const movedFrom = {
      old: "HH-5",
      at: "2026-11-13T00:05:00.000Z",
      display: "TM-4",
      current: "TM-4",
    };
    transfers.located = {
      kind: "found",
      storeId: "S1",
      projectId: "S1-P1",
      itemId: "S1-I9",
      movedFrom,
    };
    navigation.replace({ name: "item", store: "work", item: "HH-5" });
    await flush();
    expect(opened).toEqual([["S1", "S1-I9", movedFrom]]);
    expect(formatRoute(get(navigation.route))).toBe("/s/work/p/HH-P1/tree");
  });

  it("toasts the sentence of a gone item once, as a plain toast, and replaces the address with home", async () => {
    const { screen, navigation, transfers, toasts, catalog, opened } = await setup(
      "/s/work/i/HH-5",
      false,
    );
    transfers.located = {
      kind: "gone",
      reason: "deleted",
      old: "HH-5",
      at: new Date(2026, 10, 13, 9, 5).toISOString(),
      newDisplay: "TM-4",
    };
    // start は、アドレスと一覧の購読で 2 回解決する。どちらも locate に聞くが、トーストは 1 回だけ。
    screen.start();
    await flush();
    await catalog.load();
    await flush();
    expect(get(toasts.list).map((t) => [t.text, t.error])).toEqual([
      [
        "HH-5 は 11/13 09:05 に TM-4 へ移動しましたが、削除されています。設定の「削除済み」から復元できます",
        false,
      ],
    ]);
    expect(formatRoute(get(navigation.route))).toBe("/");
    expect(get(screen.current)).toEqual({ name: "home" });
    expect(opened).toEqual([]);
  });
});
