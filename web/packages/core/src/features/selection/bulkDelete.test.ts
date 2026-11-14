import { describe, expect, it } from "vitest";
import { get, writable } from "../../reactive/store";
import { fakeItemActions } from "../items/actions.test-util";
import { itemDeleteFailedMessage } from "../items/delete";
import type { DetailView } from "../items/detail";
import { flush, gate } from "../items/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { bulkDeleteDialog, bulkDeletedText, createBulkDelete } from "./bulkDelete";
import { createRowSelection, type SelectedRow } from "./rowSelection";

const row = (storeId: string, itemId: string, display: string, readOnly = false): SelectedRow => ({
  storeId,
  itemId,
  display,
  readOnly,
});
const p = { stores: 1, retentionDays: 30, readOnly: 0 };

describe("bulkDeleteDialog", () => {
  it("dialog text", () => {
    const d = bulkDeleteDialog([row("S1", "a", "HH-1"), row("S1", "b", "HH-2")], p);
    expect(d.title).toBe("一括削除");
    expect(d.lines).toEqual([
      "HH-1、HH-2 と、その下にあるものを削除します。設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。",
    ]);
    expect(d.ok).toBe("2 件を削除");
  });

  it("dialog lists ten ids", () => {
    const rows = Array.from({ length: 12 }, (_, i) => row("S1", `i${i}`, `HH-${i + 1}`));
    const d = bulkDeleteDialog(rows, p);
    expect(d.lines[0]).toBe(
      "HH-1、HH-2、HH-3、HH-4、HH-5、HH-6、HH-7、HH-8、HH-9、HH-10 ほか 2 件 と、その下にあるものを削除します。設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。",
    );
    expect(d.ok).toBe("12 件を削除");
  });

  it("dialog for several stores and read-only rows", () => {
    const d = bulkDeleteDialog(
      [row("S1", "a", "HH-1"), row("S2", "b", "RO-1", true), row("S2", "c", "HH-2")],
      { stores: 2, retentionDays: 0, readOnly: 1 },
    );
    expect(d.lines).toEqual([
      "HH-1、HH-2 と、その下にあるものを削除します。設定の「削除済み」から戻せます。保持日数を過ぎると完全に削除されます。",
      "閲覧のみの Store の 1 件は削除しません。",
    ]);
    expect(d.ok).toBe("2 件を削除");
  });
});

describe("bulkDeletedText", () => {
  it("deleted text", () => {
    expect(bulkDeletedText({ deleted: 4, readOnly: 0 })).toBe("4 件を削除しました");
    expect(bulkDeletedText({ deleted: 4, readOnly: 1 })).toBe(
      "4 件を削除しました。1 件は閲覧のみの Store のためそのままです",
    );
  });
});

function setup(allow = true) {
  const actions = fakeItemActions();
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const panel = writable<DetailView | null>(null);
  const calls: string[] = [];
  const confirms: unknown[] = [];
  const bd = createBulkDelete({
    actions,
    toasts,
    detail: {
      view: panel,
      close: () => calls.push("close"),
      reload: async () => {
        calls.push("reload");
      },
    },
    drafts: {
      confirmLeave: async (only) => {
        confirms.push(only);
        return allow;
      },
    },
  });
  const selection = createRowSelection();
  selection.toggle(row("S1", "a", "HH-1"), true);
  selection.toggle(row("S2", "b", "RO-1", true), true);
  const after = async () => {
    calls.push("after");
  };
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  const show = (store: string, id: string, ancestors: string[] = []) =>
    panel.set({
      item: { storeId: store, id },
      ancestors: ancestors.map((a) => ({ storeId: store, id: a })),
    } as unknown as DetailView);
  return { bd, actions, selection, after, calls, messages, show, confirms };
}

describe("createBulkDelete", () => {
  it("confirm deletes, clears and reloads", async () => {
    const t = setup();
    t.actions.bulkResult = { deleted: 3, readOnly: 0 };
    await t.bd.open(t.selection, t.after);
    expect(t.actions.calls).toEqual(["bulkDeletePreview S1/a,S2/b"]);
    expect(get(t.bd.state)?.title).toBe("一括削除");
    await t.bd.confirm();
    expect(t.actions.calls[1]).toBe("bulkDelete S1/a,S2/b");
    expect(t.messages()).toEqual(["3 件を削除しました"]);
    expect(get(t.selection.rows)).toEqual([]);
    expect(get(t.bd.state)).toBeNull();
    expect(t.calls.filter((c) => c === "after")).toHaveLength(1);
    expect(t.calls.filter((c) => c === "reload")).toHaveLength(1);
  });

  it("confirm closes the open panel only for the same store", async () => {
    const a = setup();
    a.actions.bulkResult = { deleted: 1, readOnly: 0 };
    a.show("S1", "a");
    await a.bd.open(a.selection, a.after);
    await a.bd.confirm();
    expect(a.calls).toContain("close");
    const b = setup();
    b.actions.bulkResult = { deleted: 1, readOnly: 0 };
    b.show("S2", "a");
    await b.bd.open(b.selection, b.after);
    await b.bd.confirm();
    expect(b.calls).not.toContain("close");
  });

  it("nothing left to delete", async () => {
    const t = setup();
    await t.bd.open(t.selection, t.after);
    await t.bd.confirm();
    expect(t.messages()).toEqual([]);
    expect(get(t.selection.rows)).toEqual([]);
    expect(t.calls).toContain("after");
    expect(t.calls).toContain("reload");
  });

  it("failure", async () => {
    const t = setup();
    await t.bd.open(t.selection, t.after);
    t.actions.failNext();
    await t.bd.confirm();
    expect(t.messages()).toEqual([`!${itemDeleteFailedMessage}`]);
    expect(get(t.bd.state)).toBeNull();
    expect(get(t.selection.rows)).toHaveLength(2);
    expect(t.calls).toEqual(["after"]);
  });

  it("failure with a refusal shows its message", async () => {
    const t = setup();
    await t.bd.open(t.selection, t.after);
    t.actions.refuseNext("だめです");
    await t.bd.confirm();
    expect(t.messages()).toEqual(["!だめです"]);
  });

  it("asks about drafts first", async () => {
    const t = setup(false);
    t.show("S1", "x", ["a"]);
    await t.bd.open(t.selection, t.after);
    expect(t.confirms).toHaveLength(1);
    expect(t.actions.calls).toEqual([]);
    expect(get(t.bd.state)).toBeNull();
  });

  it("does not ask when the panel shows something else", async () => {
    const t = setup(false);
    t.show("S2", "a");
    await t.bd.open(t.selection, t.after);
    expect(t.confirms).toHaveLength(0);
    expect(get(t.bd.state)).not.toBeNull();
  });

  it("fixed while open", async () => {
    const t = setup();
    await t.bd.open(t.selection, t.after);
    t.selection.toggle(row("S1", "c", "HH-3"), true);
    await t.bd.confirm();
    expect(t.actions.calls[1]).toBe("bulkDelete S1/a,S2/b");
  });

  it("cancel", async () => {
    const t = setup();
    const g = gate();
    const orig = t.actions.bulkDeletePreview;
    t.actions.bulkDeletePreview = async (i) => {
      await g.wait;
      return orig(i);
    };
    const opening = t.bd.open(t.selection, t.after);
    await flush();
    t.bd.cancel();
    g.release();
    await opening;
    expect(get(t.bd.state)).toBeNull();
  });
});
