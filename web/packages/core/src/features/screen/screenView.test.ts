import { describe, expect, it } from "vitest";
import { get, writable } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, flush } from "../items/fixtures.test-util";
import { fakeHistory } from "../router/history.test-util";
import { createNavigation } from "../router/navigation";
import { createToasts } from "../toasts/toasts";
import { fakeTransfers } from "../transfers/transfers.test-util";
import type { SavedView } from "../views/views";
import { createScreen } from "./screen";

const id = "01jabcdefghjkmnpqrstvwxyz0";
const bug: SavedView = {
  id,
  name: "バグ",
  content: { query: "", layout: "table", table: { columns: [], group: "none", sort: null } },
};

// ビューの画面のアドレス(E3b-2・E3b-17)。views は、ビューの一覧(テストが入れ替える)。
async function setup(start: string, list: SavedView[]) {
  const { history, stack } = fakeHistory(start);
  const navigation = createNavigation(history);
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const views = writable(list);
  const toasts = createToasts({ set: () => 0, clear: () => undefined });
  const screen = createScreen({
    navigation,
    catalog,
    views,
    items: fakeItemsPort(),
    transfers: fakeTransfers(),
    toasts,
    now: () => new Date(),
    openItem: () => {},
  });
  screen.start();
  await flush();
  return { screen, stack, views, toasts };
}

describe("createScreen for a saved view", () => {
  it("resolves the address of a view in the list", async () => {
    const { screen, stack } = await setup(`/views/${id}`, [bug]);
    expect(get(screen.current)).toEqual({ name: "view", viewId: id });
    expect(stack).toEqual([`/views/${id}`]);
  });

  it("replaces an id that is not in the list with my tasks, without a toast", async () => {
    const { screen, stack, toasts } = await setup(`/views/${id}`, []);
    expect(get(screen.current)).toEqual({ name: "mytasks" });
    expect(stack).toEqual(["/my-tasks"]);
    expect(get(toasts.list)).toEqual([]);
  });

  it("replaces with my tasks when the open view leaves the list", async () => {
    const { screen, stack, views, toasts } = await setup(`/views/${id}`, [bug]);
    views.set([]);
    await flush();
    expect(get(screen.current)).toEqual({ name: "mytasks" });
    expect(stack).toEqual(["/my-tasks"]);
    expect(get(toasts.list)).toEqual([]);
  });
});
