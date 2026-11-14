import { describe, expect, it } from "vitest";
import { writable } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import type { Route } from "../router/route";
import { createLeaveMoved } from "./leaveMoved";

// 自分でプロジェクトを移したあとの移り先(A5b-19)。新しいアドレスの「設定」タブへ replace し、そのあとで一覧を
// 読み直す。新しいアドレスは、読み直す前の一覧にないので、移動の結果(Store の内部 ID と新しい ID)から作る。
// 「移動」を押したあと、送っているあいだに画面を移っていたら、移らずに読み直すだけにする(A5a-18 と同じ)。
async function setup(loadFails = false) {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const events: string[] = [];
  const routes: Route[] = [];
  const load = catalog.load;
  const route = writable<Route>({
    name: "project",
    store: "work",
    project: "HH-P1",
    tab: "settings",
  });
  const deps = {
    navigation: {
      route,
      replace: (r: Route) => {
        events.push("replace");
        routes.push(r);
      },
    },
    catalog: {
      stores: catalog.stores,
      load: async () => {
        events.push("load");
        if (loadFails) throw new Error("network");
        await load();
      },
    },
  };
  return { leave: createLeaveMoved(deps), route, events, routes };
}

const result = { toStore: "S1", projectId: "S1-P9", display: "TM-P3", destination: "work / Tm" };

describe("createLeaveMoved", () => {
  it("replaces to the settings tab of the new address first, then reloads the list", async () => {
    const { leave, events, routes } = await setup();
    await leave()(result);
    expect(events).toEqual(["replace", "load"]);
    expect(routes).toEqual([{ name: "project", store: "work", project: "TM-P3", tab: "settings" }]);
  });

  it("only reloads when the screen changed while sending", async () => {
    const { leave, route, events } = await setup();
    const done = leave();
    route.set({ name: "workspace", store: "work", prefix: "HH", tab: "overview" });
    await done(result);
    expect(events).toEqual(["load"]);
  });

  it("does not throw when the reload fails", async () => {
    const { leave, events } = await setup(true);
    await expect(leave()(result)).resolves.toBeUndefined();
    expect(events).toEqual(["replace", "load"]);
  });

  it("goes home when the destination Store is not in the list", async () => {
    const { leave, routes } = await setup();
    await leave()({ ...result, toStore: "S9" });
    expect(routes).toEqual([{ name: "home" }]);
  });
});
