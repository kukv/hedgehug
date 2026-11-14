import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, flush } from "../items/fixtures.test-util";
import { fakeHistory } from "../router/history.test-util";
import { createNavigation } from "../router/navigation";
import { createToasts } from "../toasts/toasts";
import { createLeaveMoved } from "../transfers/leaveMoved";
import { fakeTransfers } from "../transfers/transfers.test-util";
import { createScreen } from "./screen";

// プロジェクトの旧 ID のアドレスと、移したあとの画面(A5b-2・A5b-19・A5b-24)。
// renumber に入れたプロジェクトは、次の一覧の読み直しから、その番号になる(同じ Store の中で移したときと同じく、
// 内部 ID は変わらずに表示 ID が変わる)。
async function setup(start: string) {
  const { history, stack } = fakeHistory(start);
  const navigation = createNavigation(history);
  const port = fakeCatalogPort();
  const renumber = new Map<string, number>();
  const listProjects = port.listProjects.bind(port);
  port.listProjects = async (s, w) =>
    (await listProjects(s, w)).map((p) => ({ ...p, number: renumber.get(p.id) ?? p.number }));
  const catalog = createCatalog(port);
  await catalog.load();
  const transfers = fakeTransfers();
  const screen = createScreen({
    navigation,
    catalog,
    items: fakeItemsPort(),
    transfers,
    toasts: createToasts({ set: () => 0, clear: () => undefined }),
    now: () => new Date(),
    openItem: () => {},
  });
  return { screen, navigation, catalog, stack, renumber, transfers };
}

const moved = {
  old: "HH-P7",
  at: "2026-11-13T00:05:00.000Z",
  display: "HH-P2",
  current: "HH-P2",
};

describe("createScreen with a moved project", () => {
  it("opens the moved project from its old ID on the same tab, keeps the line, and clears it on another tab", async () => {
    const { screen, navigation, stack, transfers } = await setup("/s/work/p/HH-P7/tree");
    transfers.locatedProject = {
      kind: "found",
      storeId: "S1",
      projectId: "S1-P2",
      movedFrom: moved,
    };
    screen.start();
    await flush();
    expect(stack).toEqual(["/s/work/p/HH-P2/tree"]);
    expect(get(screen.current)).toEqual({
      name: "project",
      storeId: "S1",
      projectId: "S1-P2",
      tab: "tree",
    });
    expect(get(screen.projectMoved)).toEqual({
      storeId: "S1",
      projectId: "S1-P2",
      tab: "tree",
      moved,
    });
    navigation.go({ name: "project", store: "work", project: "HH-P2", tab: "overview" });
    await flush();
    expect(get(screen.current)).toMatchObject({ projectId: "S1-P2", tab: "overview" });
    expect(get(screen.projectMoved)).toBeNull();
  });

  it("clears the line when another project opens", async () => {
    const { screen, navigation, transfers } = await setup("/s/work/p/HH-P7/tree");
    transfers.locatedProject = {
      kind: "found",
      storeId: "S1",
      projectId: "S1-P2",
      movedFrom: moved,
    };
    screen.start();
    await flush();
    navigation.go({ name: "project", store: "work", project: "HH-P1", tab: "tree" });
    await flush();
    expect(get(screen.projectMoved)).toBeNull();
  });

  // ほかの書き手が移したときは、いまの扱い(A4b-22)のまま、ワークスペースの概要へ移る。サーバーには聞かない。
  it("moves to the workspace overview without asking locateProject when another writer moved the shown project", async () => {
    const { screen, catalog, stack, renumber, transfers } = await setup("/s/work/p/HH-P2/settings");
    screen.start();
    await flush();
    transfers.locatedProject = {
      kind: "found",
      storeId: "S1",
      projectId: "S1-P2",
      movedFrom: { ...moved, old: "HH-P2", display: "HH-P5", current: "HH-P5" },
    };
    // 別の Store(ro)の HH-P2 も番号を変え、アドレスがどの Store の一覧でも見つからないようにする。
    renumber.set("S1-P2", 5);
    renumber.set("S2-P2", 6);
    await catalog.load();
    await flush();
    expect(stack).toEqual(["/s/work/w/HH/overview"]);
    expect(transfers.calls.filter((c) => c.startsWith("locateProject"))).toEqual([]);
    expect(get(screen.projectMoved)).toBeNull();
  });

  // 自分で移したあと(A5b-19 の順: 新しいアドレスへ替えてから一覧を読み直す)は、新しいアドレスをサーバーの
  // いまの ID で解決し、1 行は出さず、ワークスペースの概要へも移らない。
  it("stays on the moved project without the line after its own move", async () => {
    const { screen, navigation, catalog, stack, renumber, transfers } = await setup(
      "/s/work/p/HH-P2/settings",
    );
    screen.start();
    await flush();
    const leave = createLeaveMoved({ navigation, catalog });
    const done = leave();
    transfers.locatedProject = { kind: "found", storeId: "S1", projectId: "S1-P2" };
    renumber.set("S1-P2", 5);
    await done({ toStore: "S1", projectId: "S1-P2", display: "HH-P5", destination: "Web" });
    await flush();
    expect(transfers.calls).toEqual(["locateProject work HH-P5"]);
    expect(stack).toEqual(["/s/work/p/HH-P5/settings"]);
    expect(get(screen.current)).toEqual({
      name: "project",
      storeId: "S1",
      projectId: "S1-P2",
      tab: "settings",
    });
    expect(get(screen.projectMoved)).toBeNull();
  });
});
