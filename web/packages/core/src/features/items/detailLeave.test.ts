import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakePlanningPort } from "../planning/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createDetail } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces } from "./settings.test-util";

// 入力中のものがあるとき、別のアイテムを開く・パネルを閉じるは、確認が「破棄」のときだけ進む(02「説明」)。
async function setup(answer: boolean) {
  const asked = { n: 0 };
  const catalogPort = fakeCatalogPort();
  const catalog = createCatalog(catalogPort);
  await catalog.load();
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
  const detail = createDetail({
    planning: fakePlanningPort(),
    catalog,
    items: fakeItemsPort([epic, task]),
    toasts: createToasts({ set: () => 0, clear: () => {} }),
    workspaces: fakeWorkspaces(),
    catalogPort,
    onChanged: () => {},
    leave: async () => {
      asked.n += 1;
      return answer;
    },
  });
  return { detail, asked, epic, task };
}

describe("detail with unsaved input", () => {
  it("asks before another item opens, and keeps the open one when canceled", async () => {
    const { detail, asked, epic, task } = await setup(false);
    await detail.open("S1", task.id);
    await detail.open("S1", epic.id);
    expect(asked.n).toBe(1);
    expect(get(detail.view)?.item.id).toBe(task.id);
  });

  it("opens the other item once the input is discarded", async () => {
    const { detail, epic, task } = await setup(true);
    await detail.open("S1", task.id);
    await detail.open("S1", epic.id);
    expect(get(detail.view)?.item.id).toBe(epic.id);
  });

  it("does not ask for the first item or for the same item again", async () => {
    const { detail, asked, task } = await setup(false);
    await detail.open("S1", task.id);
    await detail.open("S1", task.id);
    expect(asked.n).toBe(0);
    expect(get(detail.view)?.item.id).toBe(task.id);
  });

  it("requestClose closes on yes and stays open on no; close never asks", async () => {
    const no = await setup(false);
    await no.detail.open("S1", no.task.id);
    await no.detail.requestClose();
    expect(get(no.detail.view)).not.toBeNull();
    no.detail.close();
    expect(get(no.detail.view)).toBeNull();
    expect(no.asked.n).toBe(1);

    const yes = await setup(true);
    await yes.detail.open("S1", yes.task.id);
    await yes.detail.requestClose();
    expect(get(yes.detail.view)).toBeNull();
  });
});
