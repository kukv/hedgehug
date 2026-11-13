import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort, project } from "../catalog/fixtures.test-util";
import { fakeItemActions } from "../items/actions.test-util";
import { itemDeleteFailedMessage } from "../items/delete";
import { flush, gate } from "../items/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import {
  createProjectDelete,
  projectDeleteDialog,
  projectDeletedMessage,
  projectDeleteNote,
} from "./delete";

const p1 = project("S1", "S1-W1", 1);
const ro = project("S2", "S2-W1", 1);

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const actions = fakeItemActions();
  actions.projectPreview = { items: 12, retentionDays: 14 };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const after: string[] = [];
  const del = createProjectDelete({
    actions,
    catalog,
    toasts,
    afterDelete: async (p) => void after.push(p.id),
  });
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return { del, actions, after, messages };
}

describe("project delete texts", () => {
  const preview = { items: 12, retentionDays: 14 };

  it("writes the explanation of the settings section", () => {
    expect(projectDeleteNote(preview)).toBe(
      "プロジェクトと、その下の 12 件のアイテムを削除します。設定の「削除済み」から戻せます。14 日を過ぎると完全に削除されます。",
    );
  });

  it("writes the title, the body and the note of the dialog", () => {
    const d = projectDeleteDialog(p1, preview);
    expect(d.title).toBe("プロジェクトを削除");
    expect(d.lines).toEqual([
      "「プロジェクト 1」と、その下の 12 件のアイテムを削除します。",
      "設定の「削除済み」から戻せます。14 日を過ぎると完全に削除されます。",
    ]);
    expect(d.ok).toBe("削除");
  });
});

describe("createProjectDelete", () => {
  it("loads the explanation for the project", async () => {
    const { del, actions } = await setup();
    await del.load(p1);
    expect(get(del.note)?.projectId).toBe(p1.id);
    expect(get(del.note)?.text).toContain("12 件");
    expect(actions.calls).toEqual([`projectDeletePreview ${p1.id}`]);
  });

  it("shows no explanation when it cannot be read, and drops an old answer", async () => {
    const { del, actions } = await setup();
    await del.load(p1);
    actions.missing.add(p1.id);
    await del.load(p1);
    expect(get(del.note)).toBeNull();
    actions.missing.clear();
    const g = gate();
    const read = actions.projectDeletePreview.bind(actions);
    actions.projectDeletePreview = async (s, id) => {
      const answer = await read(s, id);
      if (id === p1.id) await g.wait;
      return answer;
    };
    const slow = del.load(p1);
    await flush();
    await del.load({ ...p1, id: "S1-P2" });
    g.release();
    await slow;
    expect(get(del.note)?.projectId).toBe("S1-P2");
  });

  it("opens the dialog after reading the preview", async () => {
    const { del } = await setup();
    await del.open(p1);
    expect(get(del.state)?.lines[0]).toBe(
      "「プロジェクト 1」と、その下の 12 件のアイテムを削除します。",
    );
  });

  it("does nothing in a read-only Store", async () => {
    const { del, actions } = await setup();
    await del.open(ro);
    expect(get(del.state)).toBeNull();
    expect(actions.calls).toEqual([]);
  });

  it("deletes on confirm: closes, shows the toast, and leaves the project", async () => {
    const { del, actions, after, messages } = await setup();
    await del.open(p1);
    await del.confirm();
    expect(actions.calls).toEqual([`projectDeletePreview ${p1.id}`, `deleteProject ${p1.id}`]);
    expect(get(del.state)).toBeNull();
    expect(messages()).toEqual([projectDeletedMessage]);
    expect(messages()).toEqual(["プロジェクトを削除しました"]);
    expect(after).toEqual([p1.id]);
  });

  it("deletes nothing on cancel, and drops a preview that arrives after it", async () => {
    const { del, actions, after } = await setup();
    await del.open(p1);
    del.cancel();
    await del.confirm();
    expect(actions.calls).toEqual([`projectDeletePreview ${p1.id}`]);
    const g = gate();
    const read = actions.projectDeletePreview.bind(actions);
    actions.projectDeletePreview = async (s, id) => {
      const answer = await read(s, id);
      await g.wait;
      return answer;
    };
    const opening = del.open(p1);
    await flush();
    del.cancel();
    g.release();
    await opening;
    expect(get(del.state)).toBeNull();
    expect(after).toEqual([]);
  });

  it("sends one delete for a double confirm", async () => {
    const { del, actions } = await setup();
    await del.open(p1);
    const hold = actions.holdNext();
    const first = del.confirm();
    await flush();
    await del.confirm();
    hold.release();
    await first;
    expect(actions.calls.filter((c) => c.startsWith("deleteProject"))).toEqual([
      `deleteProject ${p1.id}`,
    ]);
  });

  it("shows a refusal as an error toast, closes, and does not leave", async () => {
    const { del, actions, after, messages } = await setup();
    await del.open(p1);
    actions.refuseNext("削除できません");
    await del.confirm();
    expect(messages()).toEqual(["!削除できません"]);
    expect(get(del.state)).toBeNull();
    expect(after).toEqual([]);
  });

  it("shows another failure with the shared failure words", async () => {
    const { del, actions, messages } = await setup();
    await del.open(p1);
    actions.failNext();
    await del.confirm();
    expect(messages()).toEqual([`!${itemDeleteFailedMessage}`]);
  });

  it("leaves without a toast when the project is already gone (confirm or preview)", async () => {
    const { del, actions, after, messages } = await setup();
    await del.open(p1);
    actions.missing.add(p1.id);
    await del.confirm();
    expect(messages()).toEqual([]);
    expect(after).toEqual([p1.id]);
    await del.open(p1);
    expect(get(del.state)).toBeNull();
    expect(after).toEqual([p1.id, p1.id]);
  });
});
