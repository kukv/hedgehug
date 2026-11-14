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

// ワークスペースの旧アドレスと、見出しの下の 1 行(B5c-2・B5c-13)。プロジェクトの screenMoved.test.ts と同じ形。
// goneWorkspaces に入れたワークスペースは、次の一覧の読み直しから消える。
async function setup(start: string) {
  const { history, stack } = fakeHistory(start);
  const navigation = createNavigation(history);
  const port = fakeCatalogPort();
  const goneWorkspaces = new Set<string>();
  const listWorkspaces = port.listWorkspaces.bind(port);
  port.listWorkspaces = async (s) =>
    (await listWorkspaces(s)).filter((w) => !goneWorkspaces.has(w.id));
  const catalog = createCatalog(port);
  await catalog.load();
  const transfers = fakeTransfers();
  const toasts = createToasts({ set: () => 0, clear: () => undefined });
  const screen = createScreen({
    navigation,
    catalog,
    items: fakeItemsPort(),
    transfers,
    toasts,
    now: () => new Date(2026, 10, 14, 12, 0),
    openItem: () => {},
  });
  return { screen, navigation, catalog, stack, transfers, toasts, goneWorkspaces };
}

const moved = {
  old: "XX",
  at: new Date(2026, 10, 13, 9, 5).toISOString(),
  display: "work",
  current: "work",
};

describe("createScreen with a moved workspace", () => {
  it("opens the moved workspace from its old address on the same tab, keeps the line, and clears it on another tab", async () => {
    const { screen, navigation, stack, transfers } = await setup("/s/gone/w/XX/sprints");
    transfers.locatedWorkspace = {
      kind: "found",
      storeId: "S1",
      workspaceId: "S1-W1",
      movedFrom: moved,
    };
    screen.start();
    await flush();
    // アドレスは、見つけた Store の名前の形(履歴に積まない)。いまの接頭辞は一覧から引く。
    expect(stack).toEqual(["/s/work/w/HH/sprints"]);
    // アドレスを替えたあとの解決は一覧で見つかるので、current に movedFrom は残らず、1 行だけが残る。
    expect(get(screen.current)).toEqual({
      name: "workspace",
      storeId: "S1",
      workspaceId: "S1-W1",
      tab: "sprints",
    });
    expect(get(screen.workspaceMoved)).toEqual({
      storeId: "S1",
      workspaceId: "S1-W1",
      tab: "sprints",
      // 文は、core の now() の時刻で作る。
      text: "このワークスペースは 11/13 09:05 に work へ移動しました",
    });
    navigation.go({ name: "workspace", store: "work", prefix: "HH", tab: "releases" });
    await flush();
    expect(get(screen.workspaceMoved)).toBeNull();
  });

  // 移した直後で一覧にまだないときは、アドレスを替えない(古いアドレスの接頭辞で替えると、移動先の別のワークスペースの
  // いまの接頭辞と同じことがある)。一覧を読み直したら、サーバーに聞き直し、一覧の接頭辞で替える。
  it("does not rewrite the address while the catalog lacks the moved workspace, and rewrites it with the catalog prefix after a reload", async () => {
    const { screen, catalog, stack, transfers, goneWorkspaces } = await setup("/s/ro/w/XX/sprints");
    goneWorkspaces.add("S1-W1");
    await catalog.load();
    transfers.locatedWorkspace = {
      kind: "found",
      storeId: "S1",
      workspaceId: "S1-W1",
      movedFrom: moved,
    };
    screen.start();
    await flush();
    expect(stack).toEqual(["/s/ro/w/XX/sprints"]);
    expect(get(screen.current)).toMatchObject({ name: "workspace", workspaceId: "S1-W1" });
    expect(get(screen.workspaceMoved)).toMatchObject({ workspaceId: "S1-W1", tab: "sprints" });
    goneWorkspaces.delete("S1-W1");
    await catalog.load();
    await flush();
    expect(stack).toEqual(["/s/work/w/HH/sprints"]);
    expect(get(screen.workspaceMoved)).not.toBeNull();
  });

  it("clears the line when another screen opens", async () => {
    const { screen, navigation, transfers } = await setup("/s/gone/w/XX/overview");
    transfers.locatedWorkspace = {
      kind: "found",
      storeId: "S1",
      workspaceId: "S1-W1",
      movedFrom: moved,
    };
    screen.start();
    await flush();
    expect(get(screen.workspaceMoved)).not.toBeNull();
    navigation.go({ name: "home" });
    await flush();
    expect(get(screen.workspaceMoved)).toBeNull();
  });

  it("opens by the current address without the line", async () => {
    const { screen } = await setup("/s/work/w/HH/overview");
    screen.start();
    await flush();
    expect(get(screen.workspaceMoved)).toBeNull();
  });

  // サーバー経由でほかの Store のワークスペースが見つかったとき(Store の名前を変えたあとのブックマークなど)は、
  // アドレスを見つけた Store の名前の形に替える。替えないと、次の一覧の読み直しで消えたと見られる。
  it("replaces the address with the Store name of the workspace found without movedFrom, and keeps it after a reload", async () => {
    const { screen, catalog, stack, transfers } = await setup("/s/nope/w/HH/overview");
    transfers.locatedWorkspace = { kind: "found", storeId: "S1", workspaceId: "S1-W1" };
    screen.start();
    await flush();
    await catalog.load();
    await flush();
    expect(stack).toEqual(["/s/work/w/HH/overview"]);
    expect(get(screen.current)).toMatchObject({ name: "workspace", workspaceId: "S1-W1" });
    expect(get(screen.workspaceMoved)).toBeNull();
  });

  it("goes home with the workspace gone toast", async () => {
    const { screen, stack, transfers, toasts } = await setup("/s/work/w/XX/overview");
    transfers.locatedWorkspace = {
      kind: "gone",
      reason: "deleted",
      old: "XX",
      at: moved.at,
      newDisplay: "team",
    };
    screen.start();
    await flush();
    expect(get(toasts.list).map((t) => t.text)).toEqual([
      "このワークスペースは 11/13 09:05 に team へ移動しましたが、削除されています。設定の「削除済み」から復元できます",
    ]);
    expect(stack).toEqual(["/"]);
  });

  // 出していたワークスペースのアドレスを読み直すときは、サーバーに聞かない(A5b-19 と同じ)。ほかの書き手が移した
  // ときも、移った先を開かずにホームへ移る(B5c-17)。アドレスもホームに替えるので、次の一覧の読み直しでもサーバーに
  // 聞かず、移った先へひとりでに移らない。
  it("goes home without asking locateWorkspace when reloads no longer find the shown workspace", async () => {
    const { screen, catalog, stack, transfers, goneWorkspaces } = await setup(
      "/s/work/w/HH/overview",
    );
    screen.start();
    await flush();
    transfers.locatedWorkspace = {
      kind: "found",
      storeId: "S2",
      workspaceId: "S2-W1",
      movedFrom: moved,
    };
    goneWorkspaces.add("S1-W1");
    await catalog.load();
    await flush();
    expect(transfers.calls).toEqual([]);
    expect(get(screen.current)).toEqual({ name: "home" });
    expect(stack).toEqual(["/"]);
    await catalog.load();
    await flush();
    expect(transfers.calls).toEqual([]);
    expect(stack).toEqual(["/"]);
    expect(get(screen.workspaceMoved)).toBeNull();
  });
});
