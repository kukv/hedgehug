import { describe, expect, it, vi } from "vitest";
import type { ScreenStatesPort } from "../../port/search";
import { get, writable } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, flush } from "../items/fixtures.test-util";
import { fakeHistory } from "../router/history.test-util";
import { createNavigation } from "../router/navigation";
import type { Resolved } from "../router/resolve";
import { createScreenStates } from "../screenstate/screenStates";
import { createToasts } from "../toasts/toasts";
import { fakeTransfers } from "../transfers/transfers.test-util";
import { createLastScreen, lastScreenKey } from "./lastScreen";
import { createScreen } from "./screen";

// 最後に開いていた画面を覚えて、画面を指さないアドレス(/)で開く(E4a)。
function setup(start: string, saved: string | null | Promise<string | null> = null) {
  const { history, stack } = fakeHistory(start);
  const navigation = createNavigation(history);
  const port: ScreenStatesPort = {
    get: vi.fn(async () => saved),
    put: vi.fn(async () => {}),
  };
  const current = writable<Resolved | null>(null);
  const replace = vi.spyOn(navigation, "replace");
  const last = createLastScreen({ navigation, current, screenStates: createScreenStates(port) });
  return { navigation, current, port, replace, stack, last };
}

describe("createLastScreen", () => {
  it("writes the shown screen", () => {
    const { current, port, last } = setup("/s/e2e/w/HH/sprints");
    last.start();
    expect(port.put).not.toHaveBeenCalled();
    current.set({ name: "workspace", storeId: "S1", workspaceId: "S1-W1", tab: "sprints" });
    expect(port.put).toHaveBeenCalledWith(lastScreenKey, '{"path":"/s/e2e/w/HH/sprints"}');
    expect(lastScreenKey).toBe("last-screen");
  });

  // 解決できないアドレスを打ったときは、アドレスを覚えず、マイタスクを覚える(壊れたアドレスを開き直さない)。
  it("writes my tasks for an address that did not resolve", () => {
    const { current, port, last } = setup("/s/work/w/XX/overview");
    last.start();
    current.set({ name: "mytasks" });
    expect(port.put).toHaveBeenCalledWith(lastScreenKey, '{"path":"/my-tasks"}');
  });

  it("does not write home or item", () => {
    const { current, port, last } = setup("/inbox");
    last.start();
    current.set({ name: "home" });
    current.set({ name: "item", storeId: "S1", projectId: "S1-P1", itemId: "S1-I1" });
    expect(port.put).not.toHaveBeenCalled();
  });

  it("restore goes to the last screen", async () => {
    const { replace, stack, last } = setup("/", '{"path":"/inbox"}');
    last.start();
    await flush();
    expect(replace.mock.calls).toEqual([[{ name: "inbox" }]]);
    expect(stack).toEqual(["/inbox"]);
  });

  it.each([null, "null", "{broken", '{"path":5}', '{"path":"/"}'])(
    "restore goes to my tasks when empty (%s)",
    async (saved) => {
      const { replace, stack, last } = setup("/", saved);
      last.start();
      await flush();
      expect(replace.mock.calls).toEqual([[{ name: "mytasks" }]]);
      expect(stack).toEqual(["/my-tasks"]);
    },
  );

  it("restore does not override a later move", async () => {
    let done: (body: string | null) => void = () => {};
    const { navigation, replace, stack, last } = setup(
      "/",
      new Promise<string | null>((r) => {
        done = r;
      }),
    );
    last.start();
    await flush();
    await navigation.go({ name: "inbox" });
    done('{"path":"/search"}');
    await flush();
    expect(replace).not.toHaveBeenCalled();
    expect(get(navigation.route)).toEqual({ name: "inbox" });
    expect(stack).toEqual(["/", "/inbox"]);
  });

  // Review Focus 4: 最後の画面が削除済みのワークスペースのとき、トーストは 1 回だけで、マイタスクへ移る。
  it("restore goes to my tasks when gone", async () => {
    const { history, stack } = fakeHistory("/");
    const navigation = createNavigation(history);
    const catalog = createCatalog(fakeCatalogPort());
    await catalog.load();
    const transfers = fakeTransfers();
    transfers.locatedWorkspace = {
      kind: "gone",
      reason: "deleted",
      old: "XX",
      at: new Date(2026, 10, 13, 9, 5).toISOString(),
      newDisplay: "team",
    };
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
    const port: ScreenStatesPort = {
      get: vi.fn(async () => '{"path":"/s/work/w/XX/overview"}'),
      put: vi.fn(async () => {}),
    };
    const last = createLastScreen({
      navigation,
      current: screen.current,
      screenStates: createScreenStates(port),
    });
    screen.start();
    last.start();
    await flush();
    await catalog.load();
    await flush();
    expect(get(toasts.list).map((t) => t.text)).toEqual([
      "このワークスペースは 11/13 09:05 に team へ移動しましたが、削除されています。設定の「削除済み」から復元できます",
    ]);
    expect(get(navigation.route)).toEqual({ name: "mytasks" });
    expect(get(screen.current)).toEqual({ name: "mytasks" });
    expect(stack).toEqual(["/my-tasks"]);
  });

  it("stops following when stopped", () => {
    const { current, port, last } = setup("/inbox");
    const stop = last.start();
    stop();
    current.set({ name: "inbox" });
    expect(port.put).not.toHaveBeenCalled();
  });
});
