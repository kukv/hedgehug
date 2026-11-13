import { describe, expect, it } from "vitest";
import type { HistoryPort } from "../../port/history";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, flush, item } from "../items/fixtures.test-util";
import { createNavigation } from "../router/navigation";
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
  const opened: string[] = [];
  const screen = createScreen({
    navigation,
    catalog,
    items,
    openItem: (s, i) => void opened.push(`${s}/${i}`),
  });
  if (begin) {
    screen.start();
    await flush();
  }
  return { screen, navigation, history, opened, catalog, items };
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
    expect(opened).toEqual(["S1/S1-I12"]);
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
    expect(opened).toEqual(["S1/S1-I12"]);
  });

  it("shows home for an address it cannot resolve", async () => {
    const { screen } = await setup("/s/work/p/HH-P9/overview");
    expect(get(screen.current)).toEqual({ name: "home" });
  });
});
