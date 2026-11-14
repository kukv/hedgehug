import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, flush } from "../items/fixtures.test-util";
import { fakeHistory } from "../router/history.test-util";
import { createNavigation } from "../router/navigation";
import { createToasts } from "../toasts/toasts";
import { fakeTransfers } from "../transfers/transfers.test-util";
import { createScreen } from "./screen";

// 開いているプロジェクトが、同期・ほかのタブ・ほかのプログラムで削除されたときの画面(A4b-22)。
// gone に入れたプロジェクトと、goneWorkspaces に入れたワークスペースは、次の一覧の読み直しから消える。
async function setup(start: string) {
  const { history, stack } = fakeHistory(start);
  const navigation = createNavigation(history);
  const port = fakeCatalogPort();
  const gone = new Set<string>();
  const goneWorkspaces = new Set<string>();
  const listProjects = port.listProjects.bind(port);
  const listWorkspaces = port.listWorkspaces.bind(port);
  port.listProjects = async (s, w) => (await listProjects(s, w)).filter((p) => !gone.has(p.id));
  port.listWorkspaces = async (s) =>
    (await listWorkspaces(s)).filter((w) => !goneWorkspaces.has(w.id));
  const catalog = createCatalog(port);
  await catalog.load();
  // HH-P9 は、どこにもないプロジェクト(サーバーの locateProject も NotFound)。
  const transfers = fakeTransfers();
  transfers.missing.add("HH-P9");
  const screen = createScreen({
    navigation,
    catalog,
    items: fakeItemsPort(),
    transfers,
    toasts: createToasts({ set: () => 0, clear: () => undefined }),
    now: () => new Date(),
    openItem: () => {},
  });
  screen.start();
  await flush();
  return { screen, navigation, catalog, stack, gone, goneWorkspaces };
}

describe("createScreen when the shown project is gone", () => {
  it("moves to the overview of its workspace, without pushing history", async () => {
    const { screen, catalog, stack, gone } = await setup("/s/work/p/HH-P2/settings");
    // 別の Store(ro)にも同じ HH-P2 がある。それに解決されず、概要へ移る。
    expect(catalog.project("S2", "S2-P2")).toBeDefined();
    expect(get(screen.current)).toMatchObject({ name: "project", projectId: "S1-P2" });
    gone.add("S1-P2");
    await catalog.load();
    await flush();
    expect(stack).toEqual(["/s/work/w/HH/overview"]);
    expect(get(screen.current)).toMatchObject({
      name: "workspace",
      storeId: "S1",
      workspaceId: "S1-W1",
      tab: "overview",
    });
  });

  it("goes home when the workspace is gone too", async () => {
    const { screen, catalog, stack, gone, goneWorkspaces } = await setup("/s/work/p/HH-P2/tree");
    gone.add("S1-P2");
    goneWorkspaces.add("S1-W1");
    await catalog.load();
    await flush();
    expect(get(screen.current)).toEqual({ name: "home" });
    expect(stack).toEqual(["/"]);
  });

  it("keeps showing the project when only the tab changes", async () => {
    const { screen, navigation, stack } = await setup("/s/work/p/HH-P2/overview");
    navigation.go({ name: "project", store: "work", project: "HH-P2", tab: "tree" });
    await flush();
    expect(stack.at(-1)).toBe("/s/work/p/HH-P2/tree");
    expect(get(screen.current)).toMatchObject({ name: "project", projectId: "S1-P2", tab: "tree" });
  });

  it("keeps showing a project that is still there", async () => {
    const { screen, catalog, stack, gone } = await setup("/s/work/p/HH-P2/overview");
    gone.add("S1-P1");
    await catalog.load();
    await flush();
    expect(stack).toEqual(["/s/work/p/HH-P2/overview"]);
    expect(get(screen.current)).toMatchObject({ name: "project", projectId: "S1-P2" });
  });

  it("still sends an address that never resolved home", async () => {
    const { screen, stack } = await setup("/s/work/p/HH-P9/overview");
    expect(get(screen.current)).toEqual({ name: "home" });
    expect(stack).toEqual(["/s/work/p/HH-P9/overview"]);
  });

  it("sends another missing project home, not to the workspace of the one left", async () => {
    const { screen, navigation, stack } = await setup("/s/work/p/HH-P2/overview");
    navigation.go({ name: "project", store: "work", project: "HH-P9", tab: "overview" });
    await flush();
    expect(get(screen.current)).toEqual({ name: "home" });
    expect(stack.at(-1)).toBe("/s/work/p/HH-P9/overview");
  });
});
