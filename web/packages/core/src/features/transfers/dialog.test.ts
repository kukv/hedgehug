import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { isItemDraft } from "../drafts/drafts";
import { flush, item } from "../items/fixtures.test-util";
import { setup, viewOf } from "./dialog.test-util";
import { noTargetsMessage } from "./dialogText";

describe("transfer dialog: opening and choosing (02「移す先の選び方」)", () => {
  it("shows a toast and opens nothing when there is no candidate", async () => {
    const { dialog, fake, task, toastTexts } = await setup();
    fake.targetList = [];
    await dialog.open(task);
    expect(get(dialog.state)).toBeNull();
    expect(toastTexts()).toEqual([noTargetsMessage]);
  });

  it("does nothing for an item in a read-only Store", async () => {
    const { dialog, fake } = await setup();
    await dialog.open(item("S2", "S2-P1", 3, "Task", null));
    expect(fake.calls).toEqual([]);
    expect(get(dialog.state)).toBeNull();
  });

  it("selects the first workspace, project and Epic, and reads the preview for them", async () => {
    const { dialog, fake, task } = await setup();
    fake.defaultPreview = { ...fake.defaultPreview, below: 0, deleted: 0, nextDisplay: "MB-2" };
    await dialog.open(task);
    expect(fake.calls).toEqual(["targets S1-I3", "preview S1-I3 S1/S1-W2/S1-W2-P1"]);
    const s = get(dialog.state);
    expect(s?.displayId).toBe("HH-3");
    expect(s?.workspaces).toEqual([
      { value: "S1/S1-W2", label: "work / Mobile" },
      { value: "S3/S3-W1", label: "team / Team" },
    ]);
    expect(s?.projects.map((o) => o.label)).toEqual(["アプリ", "空"]);
    expect(s?.parents).toEqual([{ value: "S1-W2-E1", label: "MB-1 通知" }]);
    expect(s?.form).toEqual({ workspace: "S1/S1-W2", project: "S1-W2-P1", parent: "S1-W2-E1" });
    expect(s?.intro).toBe("HH-3 決済 を移動します。");
    expect(s?.changes[0]).toBe("ID は MB-2 からの番号に振り直されます");
    expect(s?.noEpic).toBe("");
    expect(s?.canSubmit).toBe(true);
  });

  it("does not select a parent for an Epic and can submit in a project without Epics", async () => {
    const { dialog, epic } = await setup();
    await dialog.open(epic);
    expect(get(dialog.state)?.parents).toBeNull();
    await dialog.setProject("S1-W2-P2");
    expect(get(dialog.state)?.noEpic).toBe("");
    expect(get(dialog.state)?.canSubmit).toBe(true);
  });

  it("says the project has no Epic and cannot submit, for a Task", async () => {
    const { dialog, task } = await setup();
    await dialog.open(task);
    await dialog.setProject("S1-W2-P2");
    const s = get(dialog.state);
    expect(s?.noEpic).toBe("このプロジェクトには Epic がありません");
    expect(s?.parents).toEqual([]);
    expect(s?.form.parent).toBe("");
    expect(s?.canSubmit).toBe(false);
  });

  it("reads the preview again and rebuilds the status rows when the workspace or project changes", async () => {
    const { dialog, fake, task } = await setup();
    fake.previewOf["S1/S1-W2/S1-W2-P1"] = {
      ...fake.defaultPreview,
      replacements: [
        {
          status: "st9",
          name: "Review",
          count: 3,
          choices: [
            { id: "st1", name: "Todo" },
            { id: "st2", name: "Doing" },
          ],
          default: "st1",
        },
      ],
    };
    await dialog.open(task);
    expect(get(dialog.state)?.replacements).toEqual([
      {
        status: "st9",
        name: "Review",
        label: "Review(3 件)",
        options: [
          { value: "st1", label: "Todo" },
          { value: "st2", label: "Doing" },
        ],
        value: "st1",
      },
    ]);
    dialog.setReplacement("st9", "st2");
    expect(get(dialog.state)?.replacements[0]?.value).toBe("st2");
    await dialog.setWorkspace("S3/S3-W1");
    expect(fake.calls.at(-1)).toBe("preview S1-I3 S3/S3-W1/S3-W1-P1");
    expect(get(dialog.state)?.replacements).toEqual([]);
    expect(get(dialog.state)?.form).toEqual({
      workspace: "S3/S3-W1",
      project: "S3-W1-P1",
      parent: "S3-W1-E1",
    });
    await dialog.setWorkspace("S1/S1-W2");
    expect(get(dialog.state)?.replacements[0]?.value).toBe("st1");
    await dialog.setProject("S1-W2-P2");
    expect(fake.calls.at(-1)).toBe("preview S1-I3 S1/S1-W2/S1-W2-P2");
  });

  it("takes a status choice only for a row of the preview and one of its options", async () => {
    const { dialog, fake, task } = await setup();
    fake.previewOf["S1/S1-W2/S1-W2-P1"] = {
      ...fake.defaultPreview,
      replacements: [
        {
          status: "st9",
          name: "Review",
          count: 1,
          choices: [{ id: "st1", name: "Todo" }],
          default: "st1",
        },
      ],
    };
    await dialog.open(task);
    dialog.setReplacement("st9", "st7");
    dialog.setReplacement("st8", "st1");
    expect(get(dialog.state)?.replacements.map((r) => [r.status, r.value])).toEqual([
      ["st9", "st1"],
    ]);
    await dialog.submit();
    expect(fake.calls.at(-1)).toContain(
      JSON.stringify({ statusChoices: { st9: "st1" } }).slice(1, -1),
    );
  });

  it("uses only the later open when open is called again while the candidates load", async () => {
    const { dialog, fake, task, epic } = await setup();
    const first = dialog.open(task);
    const second = dialog.open(epic);
    await Promise.all([first, second]);
    expect(get(dialog.state)?.displayId).toBe("HH-1");
    expect(get(dialog.state)?.parents).toBeNull();
    expect(fake.calls).toEqual([
      "targets S1-I3",
      "targets S1-I1",
      "preview S1-I1 S1/S1-W2/S1-W2-P1",
    ]);
  });

  it("does not read the preview again when only the parent Epic changes", async () => {
    const { dialog, fake, task } = await setup();
    await dialog.open(task);
    const before = fake.calls.length;
    dialog.setParent("S1-W2-E1");
    expect(fake.calls.length).toBe(before);
    expect(get(dialog.state)?.form.parent).toBe("S1-W2-E1");
  });

  it("shows the loading state, with the button off, until the preview arrives", async () => {
    const { dialog, fake, task } = await setup();
    const hold = fake.holdNext();
    const opening = dialog.open(task);
    await flush();
    expect(get(dialog.state)).toMatchObject({
      loading: true,
      intro: "",
      changes: [],
      replacements: [],
      canSubmit: false,
    });
    hold.release();
    await opening;
    expect(get(dialog.state)?.loading).toBe(false);
  });

  it("drops a stale preview that arrives after the destination changed", async () => {
    const { dialog, fake, task } = await setup();
    fake.previewOf["S1/S1-W2/S1-W2-P1"] = { ...fake.defaultPreview, nextDisplay: "MB-9" };
    fake.previewOf["S3/S3-W1/S3-W1-P1"] = { ...fake.defaultPreview, nextDisplay: "TM-5" };
    const hold = fake.holdNext();
    const opening = dialog.open(task);
    await flush();
    await dialog.setWorkspace("S3/S3-W1");
    hold.release();
    await opening;
    expect(get(dialog.state)?.changes[0]).toBe("ID は TM-5 からの番号に振り直されます");
    expect(get(dialog.state)?.form.workspace).toBe("S3/S3-W1");
  });

  it("closes when cancelled, and ignores the preview that comes after", async () => {
    const { dialog, fake, task } = await setup();
    const hold = fake.holdNext();
    const opening = dialog.open(task);
    await flush();
    dialog.cancel();
    hold.release();
    await opening;
    expect(get(dialog.state)).toBeNull();
  });

  it("shows a toast when the candidates cannot be read, and reloads when the item is gone", async () => {
    const { dialog, fake, task, toasts, env } = await setup();
    fake.refuseNext("閲覧のみの Store です");
    await dialog.open(task);
    expect(get(dialog.state)).toBeNull();
    expect(toasts()).toEqual([{ text: "閲覧のみの Store です", error: true }]);
    fake.missing.add("S1-I3");
    await dialog.open(task);
    expect(get(dialog.state)).toBeNull();
    expect(toasts().at(-1)).toEqual({ text: "移動できませんでした", error: true });
    expect(env.refreshed).toBe(1);
  });

  it("closes with a toast when the preview cannot be read, and reloads when the item is gone", async () => {
    const { dialog, fake, task, toasts, env } = await setup();
    await dialog.open(task);
    fake.refuseNext("閲覧のみの Store です");
    await dialog.setWorkspace("S3/S3-W1");
    expect(fake.calls.at(-1)).toBe("preview S1-I3 S3/S3-W1/S3-W1-P1");
    expect(get(dialog.state)).toBeNull();
    expect(toasts()).toEqual([{ text: "閲覧のみの Store です", error: true }]);
    await dialog.open(task);
    fake.missing.add("S1-I3");
    await dialog.setProject("S1-W2-P2");
    expect(fake.calls.at(-1)).toBe("preview S1-I3 S1/S1-W2/S1-W2-P2");
    expect(get(dialog.state)).toBeNull();
    expect(toasts().at(-1)).toEqual({ text: "移動できませんでした", error: true });
    expect(env.refreshed).toBe(1);
  });

  it("asks to discard a draft first when the panel shows the item, and stops on cancel", async () => {
    const { dialog, fake, task, panel, confirms, env } = await setup();
    panel.set(viewOf(task));
    env.allow = false;
    await dialog.open(task);
    expect(confirms).toEqual([isItemDraft]);
    expect(fake.calls).toEqual([]);
    expect(get(dialog.state)).toBeNull();
  });

  it("asks when the panel shows an item below, but not for another item or Store", async () => {
    const { dialog, epic, task, panel, confirms } = await setup();
    const child = item("S1", "S1-P1", 4, "Task", epic.id, "子");
    panel.set(viewOf(child, [epic]));
    await dialog.open(epic);
    expect(confirms).toHaveLength(1);
    dialog.cancel();
    // 内部 ID が同じでも、別の Store のアイテムは別のもの。
    panel.set(viewOf({ ...task, storeId: "S2" }));
    await dialog.open(task);
    expect(confirms).toHaveLength(1);
    expect(get(dialog.state)).not.toBeNull();
  });
});
