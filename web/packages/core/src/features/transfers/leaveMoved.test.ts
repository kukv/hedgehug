import { describe, expect, it } from "vitest";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import type { Route } from "../router/route";
import { leaveMovedProject } from "./leaveMoved";

// 自分でプロジェクトを移したあとの移り先(A5b-19)。新しいアドレスの「設定」タブへ replace し、そのあとで一覧を
// 読み直す。新しいアドレスは、読み直す前の一覧にないので、移動の結果(Store の内部 ID と新しい ID)から作る。
async function setup(loadFails = false) {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const events: string[] = [];
  const routes: Route[] = [];
  const load = catalog.load;
  const deps = {
    navigation: {
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
  return { deps, events, routes };
}

const result = { toStore: "S1", projectId: "S1-P9", display: "TM-P3", destination: "work / Tm" };

describe("leaveMovedProject", () => {
  it("replaces to the settings tab of the new address first, then reloads the list", async () => {
    const { deps, events, routes } = await setup();
    await leaveMovedProject(deps, result);
    expect(events).toEqual(["replace", "load"]);
    expect(routes).toEqual([{ name: "project", store: "work", project: "TM-P3", tab: "settings" }]);
  });

  it("does not throw when the reload fails", async () => {
    const { deps, events } = await setup(true);
    await expect(leaveMovedProject(deps, result)).resolves.toBeUndefined();
    expect(events).toEqual(["replace", "load"]);
  });

  it("goes home when the destination Store is not in the list", async () => {
    const { deps, routes } = await setup();
    await leaveMovedProject(deps, { ...result, toStore: "S9" });
    expect(routes).toEqual([{ name: "home" }]);
  });
});
