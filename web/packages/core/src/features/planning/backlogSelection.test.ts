import { describe, expect, it } from "vitest";
import type { BacklogFrame } from "../../model/planning";
import { get } from "../../reactive/store";
import { fakeItemsPort, item } from "../items/fixtures.test-util";
import { fakeWorkspaces } from "../items/settings.test-util";
import { createRowSelection, type SelectedRow } from "../selection/rowSelection";
import { createToasts } from "../toasts/toasts";
import { createBacklog } from "./backlog";
import { goneText } from "./backlogDrop";
import { fakePlanningPort, frameOf, rowOf, sprintOf } from "./fixtures.test-util";

// バックログの行の選択のつなぎ(F2b1-3・F2b1-14)。1 はスプリントの枠、2・3 はバックログの枠。
const sp1 = sprintOf({ id: "SP1", state: "active" });
const row = (n: number) => rowOf({ id: `S1-I${n}`, display: `HH-${n}`, status: "st1" });
const sel = (n: number, readOnly = false): SelectedRow => ({
  storeId: "S1",
  itemId: `S1-I${n}`,
  display: `HH-${n}`,
  readOnly,
});
const framesOf = (): BacklogFrame[] => [frameOf(sp1, [row(1)]), frameOf(null, [row(2), row(3)])];
const ids = (rows: SelectedRow[]) => rows.map((r) => r.itemId);

async function setup(readOnly = false) {
  const planning = fakePlanningPort();
  planning.backlogs["S1-W1"] = framesOf();
  planning.backlogs["S1-W2"] = [];
  const items = fakeItemsPort([1, 2, 3].map((n) => item("S1", "S1-P1", n, "Task", null)));
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const selection = createRowSelection();
  const backlog = createBacklog({
    port: planning,
    items,
    workspaces: fakeWorkspaces(),
    listMembers: async () => [],
    toasts,
    openDetail: () => {},
    selection,
    isReadOnly: () => readOnly,
  });
  await backlog.open("S1", "S1-W1");
  const shown = () => get(toasts.list).map((t) => t.text);
  return { backlog, planning, items, selection, shown };
}

describe("createBacklog selection", () => {
  it("exposes the selection of its deps", async () => {
    const { backlog, selection } = await setup();
    expect(backlog.selection).toBe(selection);
  });

  it("prunes the selection on reload, top to bottom", async () => {
    const { backlog, planning, selection } = await setup();
    for (const n of [3, 2, 1]) selection.toggle(sel(n), true);
    planning.backlogs["S1-W1"] = [frameOf(sp1, [row(1)]), frameOf(null, [row(2)])];
    await backlog.reload();
    expect(ids(get(selection.rows))).toEqual(["S1-I1", "S1-I2"]);
    planning.backlogs["S1-W1"] = [];
    await backlog.reload();
    expect(get(selection.rows)).toEqual([]);
  });

  it("keeps the selection when a reload fails", async () => {
    const { backlog, planning, selection } = await setup();
    selection.toggle(sel(2), true);
    planning.backlog = async () => {
      throw new Error("down");
    };
    await backlog.reload();
    expect(ids(get(selection.rows))).toEqual(["S1-I2"]);
  });

  it("clears on close and on another workspace, but not on the same one", async () => {
    const { backlog, selection } = await setup();
    selection.toggle(sel(2), true);
    await backlog.open("S1", "S1-W1");
    expect(ids(get(selection.rows))).toEqual(["S1-I2"]);
    await backlog.open("S1", "S1-W2");
    expect(get(selection.rows)).toEqual([]);
    await backlog.open("S1", "S1-W1");
    selection.toggle(sel(2), true);
    backlog.close();
    expect(get(selection.rows)).toEqual([]);
  });

  it("carries read-only of the store into the rows", async () => {
    const { backlog, selection } = await setup(true);
    selection.toggle(sel(2), true);
    await backlog.reload();
    expect(get(selection.rows)).toEqual([sel(2, true)]);
  });
});

describe("createBacklog selectedOne", () => {
  it("is null for none or several rows", async () => {
    const { backlog, selection } = await setup();
    expect(backlog.selectedOne()).toBeNull();
    selection.toggle(sel(1), true);
    selection.toggle(sel(2), true);
    expect(backlog.selectedOne()).toBeNull();
  });

  it("gives the one row, whose setStatus saves and toasts like the status menu", async () => {
    const { backlog, items, selection, shown } = await setup();
    selection.toggle(sel(2), true);
    const one = backlog.selectedOne();
    expect(one).toMatchObject({ storeId: "S1", itemId: "S1-I2", source: "backlog" });
    await one?.setStatus("st3");
    expect((await items.get("S1", "S1-I2")).status).toBe("st3");
    expect(shown()).toEqual([goneText("HH-2", "done")]);
  });

  it("is null when the one row is not shown", async () => {
    const { backlog, selection } = await setup();
    selection.toggle(sel(9), true);
    expect(backlog.selectedOne()).toBeNull();
    selection.clear();
    selection.toggle({ ...sel(2), storeId: "S2" }, true);
    expect(backlog.selectedOne()).toBeNull();
  });
});
