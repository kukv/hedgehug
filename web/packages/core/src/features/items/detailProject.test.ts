import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createDetail } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces, settings } from "./settings.test-util";

// 詳細パネルの行は、アイテムのプロジェクトの使うステータスだけを選択肢にする(03「使うステータス」)。
describe("detail with a project's rules", () => {
  it("narrows the status row to the statuses the item's project uses", async () => {
    const catalogPort = fakeCatalogPort();
    const catalog = createCatalog(catalogPort);
    await catalog.load();
    const task = item("S1", "S1-P1", 2, "Task", null, "画面");
    const ws = {
      ...settings,
      categoryDefaults: { todo: "st1", done: "st3", canceled: "st4" },
      projectRules: {
        "S1-P1": {
          hiddenStatuses: ["st2"],
          workflow: {},
          templates: [],
          templateDefaults: {},
          disabledRules: [],
        },
      },
    };
    const detail = createDetail({
      catalog,
      items: fakeItemsPort([task]),
      toasts: createToasts({ set: () => 0, clear: () => {} }),
      workspaces: fakeWorkspaces({ "S1-W1": ws }),
      catalogPort,
      onChanged: () => {},
    });
    await detail.open("S1", task.id);
    const row = get(detail.view)?.rows.find((r) => r.property.id === "status");
    expect(row?.options.map((o) => o.id)).toEqual(["st1", "st3", "st4"]);
  });
});
