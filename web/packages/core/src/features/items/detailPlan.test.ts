import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakePlanningPort, overviewOf, releaseOf, sprintOf } from "../planning/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createDetail } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces } from "./settings.test-util";

// 詳細パネルのスプリント・リリース(02「スプリント・リリース」、F1-5・F1-15)。
async function setup(sprint = "", release = "") {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const task = { ...item("S1", "S1-P1", 2, "Task", null, "画面"), sprint, release };
  const planning = fakePlanningPort({
    "S1-W1": overviewOf(
      [sprintOf({ id: "SP1", state: "active" }), sprintOf({ id: "SP0", state: "closed" })],
      [releaseOf({ id: "R1" })],
    ),
  });
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const detail = createDetail({
    catalog,
    items: fakeItemsPort([task]),
    workspaces: fakeWorkspaces(),
    catalogPort: fakeCatalogPort(),
    planning,
    toasts,
    onChanged: () => void changed++,
  });
  await detail.open("S1", task.id);
  return { detail, planning, toasts, task, changed: () => changed };
}

describe("createDetail sprint and release", () => {
  it("loads the planning overview of the item's workspace with the item", async () => {
    const { detail, planning } = await setup();
    expect(get(detail.view)?.plan).toBe(planning.overviews["S1-W1"]);
  });

  it("saves the sprint and the release, sending the backlog and none as null", async () => {
    const { detail, planning, task, changed } = await setup("SP1", "R1");
    expect(await detail.setSprint("")).toBe(true);
    expect(await detail.setRelease("")).toBe(true);
    expect(await detail.setSprint("SP1")).toBe(true);
    expect(planning.calls.filter((c) => !c.startsWith("overview"))).toEqual([
      `sprint ${task.id} null`,
      `release ${task.id} null`,
    ]);
    expect(changed()).toBe(2);
  });

  it("shows a refusal as an error toast and tells the field to revert", async () => {
    const { detail, planning, toasts } = await setup();
    planning.refuseNext("完了したスプリントには入れられません");
    expect(await detail.setSprint("SP0")).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({
      text: "完了したスプリントには入れられません",
      error: true,
    });
  });

  it("shows other failures as the save-failed toast and tells the field to revert", async () => {
    const { detail, planning, toasts, changed } = await setup();
    planning.failNext(new Error("404"));
    expect(await detail.setSprint("SP1")).toBe(false);
    planning.failNext(new Error("network"));
    expect(await detail.setRelease("R1")).toBe(false);
    expect(get(toasts.list).map((t) => [t.text, t.error])).toEqual([
      ["保存できませんでした", true],
      ["保存できませんでした", true],
    ]);
    expect(changed()).toBe(0);
  });

  it("does not rewrite a deleted sprint shown as the backlog when the backlog is picked", async () => {
    const { detail, planning, changed } = await setup("gone", "gone");
    expect(await detail.setSprint("")).toBe(true);
    expect(await detail.setRelease("")).toBe(true);
    expect(planning.calls.filter((c) => !c.startsWith("overview"))).toEqual([]);
    expect(changed()).toBe(0);
  });

  it("reads the overview again on reload", async () => {
    const { detail, planning } = await setup();
    const next = overviewOf([sprintOf({ id: "SP2" })]);
    planning.overviews["S1-W1"] = next;
    await detail.reload();
    expect(get(detail.view)?.plan).toBe(next);
  });
});
