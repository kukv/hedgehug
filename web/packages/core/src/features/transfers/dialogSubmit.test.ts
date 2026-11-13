import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { flush, item } from "../items/fixtures.test-util";
import { setup, viewOf } from "./dialog.test-util";

const sent = (calls: string[]) => calls.filter((c) => c.startsWith("transfer"));

describe("transfer dialog: sending (02「保存」、A5a-12)", () => {
  it("sends the selected destination and the status choices, then toasts and calls afterMoved", async () => {
    const { dialog, fake, task, toastTexts, moved, shown } = await setup();
    fake.previewOf["S1/S1-W2/S1-W2-P1"] = {
      ...fake.defaultPreview,
      replacements: [
        {
          status: "st9",
          name: "Review",
          count: 1,
          choices: [{ id: "st2", name: "Doing" }],
          default: "st2",
        },
      ],
    };
    fake.result = { toStore: "S1", itemId: "S1-I3", display: "MB-2", emptied: [] };
    await dialog.open(task);
    await dialog.submit();
    expect(fake.calls.at(-1)).toBe(
      `transfer S1-I3 ${JSON.stringify({ toStore: "S1", toWorkspace: "S1-W2", toProject: "S1-W2-P1", toParent: "S1-W2-E1", statusChoices: { st9: "st2" } })}`,
    );
    expect(toastTexts()).toEqual(["HH-3 は MB-2 になりました"]);
    expect(moved).toEqual(["HH-3 → S1/S1-I3"]);
    expect(shown).toEqual([undefined]);
    expect(get(dialog.state)).toBeNull();
  });

  it("omits toParent for an Epic and lists the emptied items in the toast", async () => {
    const { dialog, fake, epic, toastTexts } = await setup();
    fake.result = {
      toStore: "S3",
      itemId: "N1",
      display: "TM-5",
      emptied: ["優先度", "関連システム"],
    };
    await dialog.open(epic);
    await dialog.setWorkspace("S3/S3-W1");
    await dialog.submit();
    expect(fake.calls.at(-1)).toBe(
      `transfer S1-I1 ${JSON.stringify({ toStore: "S3", toWorkspace: "S3-W1", toProject: "S3-W1-P1", statusChoices: {} })}`,
    );
    expect(toastTexts()).toEqual([
      "HH-1 は TM-5 になりました。合う値がなく空になった項目: 優先度・関連システム",
    ]);
  });

  it("tells afterMoved that the panel was showing the item when the send started", async () => {
    const { dialog, fake, task, panel, shown } = await setup();
    panel.set(viewOf(task));
    await dialog.open(task);
    const hold = fake.holdNext();
    const sending = dialog.submit();
    await flush();
    // 送っているあいだに読み直しでパネルが閉じても、押したときに出していたことを渡す。
    panel.set(null);
    hold.release();
    await sending;
    expect(shown).toEqual(["S1-I3"]);
  });

  it("tells afterMoved nothing when the panel shows the same internal ID in another Store", async () => {
    const { dialog, task, panel, shown } = await setup();
    panel.set(viewOf({ ...task, storeId: "S2" }));
    await dialog.open(task);
    await dialog.submit();
    expect(shown).toEqual([undefined]);
  });

  it("tells afterMoved the item below that the panel was showing (02「保存」)", async () => {
    const { dialog, epic, panel, shown } = await setup();
    const child = item("S1", "S1-P1", 4, "Task", epic.id, "子");
    panel.set(viewOf(child, [epic]));
    await dialog.open(epic);
    await dialog.submit();
    expect(shown).toEqual(["S1-I4"]);
  });

  it("shows a refusal as it is, keeps the dialog open, and does not call afterMoved", async () => {
    const { dialog, fake, task, toasts, moved } = await setup();
    await dialog.open(task);
    fake.refuseNext("team に送信できないため移動できません: remote に接続できません");
    await dialog.submit();
    expect(toasts()).toEqual([
      { text: "team に送信できないため移動できません: remote に接続できません", error: true },
    ]);
    expect(get(dialog.state)?.busy).toBe(false);
    expect(get(dialog.state)?.canSubmit).toBe(true);
    expect(moved).toEqual([]);
  });

  it("says 移動できませんでした for another failure", async () => {
    const { dialog, fake, task, toasts } = await setup();
    await dialog.open(task);
    fake.failNext();
    await dialog.submit();
    expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
    expect(get(dialog.state)).not.toBeNull();
  });

  it("refuses before sending when moving to another Store with a remote while offline", async () => {
    const { dialog, fake, task, toasts, env } = await setup();
    env.online = false;
    fake.previewOf["S3/S3-W1/S3-W1-P1"] = {
      ...fake.defaultPreview,
      crossStore: true,
      destinationHasRemote: true,
    };
    await dialog.open(task);
    await dialog.setWorkspace("S3/S3-W1");
    await dialog.submit();
    expect(sent(fake.calls)).toEqual([]);
    expect(toasts()).toEqual([{ text: "オフラインのため team へ移動できません", error: true }]);
    expect(get(dialog.state)).not.toBeNull();
  });

  it("sends offline when the move stays inside the Store, even if the Store has a remote", async () => {
    const { dialog, fake, task, env } = await setup();
    env.online = false;
    fake.previewOf["S1/S1-W2/S1-W2-P1"] = { ...fake.defaultPreview, destinationHasRemote: true };
    await dialog.open(task);
    await dialog.submit();
    expect(sent(fake.calls)).toHaveLength(1);
  });

  it("sends offline when the other Store has no remote", async () => {
    const { dialog, fake, task, env } = await setup();
    env.online = false;
    fake.previewOf["S3/S3-W1/S3-W1-P1"] = { ...fake.defaultPreview, crossStore: true };
    await dialog.open(task);
    await dialog.setWorkspace("S3/S3-W1");
    await dialog.submit();
    expect(sent(fake.calls)).toHaveLength(1);
  });

  it("closes and reloads the screen when the item is gone", async () => {
    const { dialog, fake, task, toasts, env, moved } = await setup();
    await dialog.open(task);
    fake.missing.add("S1-I3");
    await dialog.submit();
    expect(get(dialog.state)).toBeNull();
    expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
    expect(env.refreshed).toBe(1);
    expect(moved).toEqual([]);
  });

  it("sends only once while the first send is pending, and ignores destination changes meanwhile", async () => {
    const { dialog, fake, task } = await setup();
    await dialog.open(task);
    const hold = fake.holdNext();
    const first = dialog.submit();
    void dialog.submit();
    await flush();
    expect(get(dialog.state)).toMatchObject({ busy: true, canSubmit: false });
    await dialog.setWorkspace("S3/S3-W1");
    expect(get(dialog.state)?.form.workspace).toBe("S1/S1-W2");
    hold.release();
    await first;
    expect(sent(fake.calls)).toHaveLength(1);
  });

  it("does not send while the preview is loading or without a parent Epic", async () => {
    const { dialog, fake, task } = await setup();
    const hold = fake.holdNext();
    const opening = dialog.open(task);
    await flush();
    await dialog.submit();
    hold.release();
    await opening;
    await dialog.setProject("S1-W2-P2");
    await dialog.submit();
    expect(sent(fake.calls)).toEqual([]);
  });

  it("still toasts and calls afterMoved when cancelled while the send is pending", async () => {
    const { dialog, fake, task, toastTexts, moved } = await setup();
    await dialog.open(task);
    const hold = fake.holdNext();
    const sending = dialog.submit();
    await flush();
    dialog.cancel();
    hold.release();
    await sending;
    expect(get(dialog.state)).toBeNull();
    expect(toastTexts()).toEqual(["HH-3 は MB-2 になりました"]);
    expect(moved).toHaveLength(1);
  });
});
