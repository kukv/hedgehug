import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { flush } from "./fixtures.test-util";
import { moveFailedMessage } from "./move";
import { epic, far, project, setup } from "./move.test-util";

describe("the status replacement dialog (B5a-20)", () => {
  const unused = { below: 2, statuses: [{ status: "st5", count: 1, default: "st1" }] };

  it("asks for replacements when the project changes and the destination does not use a status", async () => {
    const { move, actions, messages, task } = await setup();
    actions.movePreviewResult = unused;
    const moving = move.moveTo(task, far);
    await flush();
    expect(actions.calls).toEqual([`movePreview ${task.id} {"parent":"S1-I9"}`]);
    expect(get(move.moveConfirm)).toEqual({
      title: "移動",
      body: "HH-2 画面 と、その下の 2 件を プロジェクト 2 へ移動します。",
      rows: [
        {
          status: "st5",
          label: "Pending(1 件)",
          options: [
            { id: "st1", label: "Todo" },
            { id: "st2", label: "InProgress" },
            { id: "st3", label: "Done" },
            { id: "st4", label: "Cancel" },
          ],
          value: "st1",
        },
      ],
    });
    move.confirmMove({ st5: "st2" });
    expect(await moving).toBe(true);
    expect(get(move.moveConfirm)).toBeNull();
    expect(actions.calls.at(-1)).toBe(`move ${task.id} {"parent":"S1-I9"} {"st5":"st2"}`);
    expect(messages()).toEqual(["プロジェクト 2 に移動しました"]);
  });

  it("says only the item when nothing is below, and sends the project target for an Epic", async () => {
    const { move, actions, task } = await setup();
    actions.movePreviewResult = { below: 0, statuses: unused.statuses };
    void move.moveTo(task, project);
    await flush();
    expect(get(move.moveConfirm)?.body).toBe("HH-2 画面 を移動します。");
  });

  it("cancel returns false and does not move", async () => {
    const { move, actions, messages, refreshed, task } = await setup();
    actions.movePreviewResult = unused;
    const moving = move.moveTo(task, far);
    await flush();
    move.cancelMove();
    expect(await moving).toBe(false);
    expect(get(move.moveConfirm)).toBeNull();
    expect(actions.calls.some((c) => c.startsWith("move "))).toBe(false);
    expect(messages()).toEqual([]);
    expect(refreshed).toEqual([]);
  });

  it("refuses another move while the dialog is open", async () => {
    const { move, actions, task } = await setup();
    actions.movePreviewResult = unused;
    const first = move.moveTo(task, far);
    await flush();
    expect(await move.moveTo(task, epic)).toBe(false);
    move.cancelMove();
    await first;
  });

  it("moves without a dialog when the preview has no unused status", async () => {
    const { move, actions, task } = await setup();
    actions.movePreviewResult = { below: 3, statuses: [] };
    expect(await move.moveTo(task, far)).toBe(true);
    expect(get(move.moveConfirm)).toBeNull();
    expect(actions.calls).toEqual([
      `movePreview ${task.id} {"parent":"S1-I9"}`,
      `move ${task.id} {"parent":"S1-I9"}`,
    ]);
  });

  it("does not read the preview for a move inside the same project", async () => {
    const { move, actions, task } = await setup();
    actions.movePreviewResult = unused;
    expect(await move.moveTo(task, epic)).toBe(true);
    expect(actions.calls).toEqual([`move ${task.id} {"parent":"S1-I1"}`]);
  });

  it("moves through choose with the dialog too", async () => {
    const { move, actions, task } = await setup();
    actions.movePreviewResult = unused;
    await move.open(task, 0, 0);
    const choosing = move.choose(far);
    await flush();
    expect(get(move.state)).toBeNull();
    expect(get(move.moveConfirm)).not.toBeNull();
    move.confirmMove({ st5: "st1" });
    await choosing;
    expect(actions.calls.at(-1)).toBe(`move ${task.id} {"parent":"S1-I9"} {"st5":"st1"}`);
  });

  it("shows a failure of the preview and does not move", async () => {
    const { move, actions, messages, refreshed, task } = await setup();
    actions.missing.add(task.id);
    expect(await move.moveTo(task, far)).toBe(false);
    expect(get(move.moveConfirm)).toBeNull();
    expect(messages()).toEqual([`!${moveFailedMessage}`]);
    expect(refreshed).toEqual(["change"]);
  });
});
