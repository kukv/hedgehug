import { describe, expect, it, vi } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { InboxPage } from "../../model/inbox";
import { get } from "../../reactive/store";
import { emptyOf, inboxTabs, pillOf } from "./inbox";
import { row, rows, setup } from "./inbox.test-util";

const ids = (s: { rows: { id: number }[] }) => s.rows.map((r) => r.id);

describe("createInbox (06「受信箱」)", () => {
  it("opens the 300 newest rows of すべて, and more appends the next ones", async () => {
    const t = setup(vi.fn, rows(301));
    await t.inbox.open();
    let s = get(t.inbox.state);
    expect(s).toMatchObject({ tab: "all", more: true, loading: false });
    expect(s.rows).toHaveLength(300);
    expect(t.port.list).toHaveBeenLastCalledWith("all");
    expect(get(t.inbox.unread).all).toBe(301);

    await t.inbox.more();
    s = get(t.inbox.state);
    expect(t.port.list).toHaveBeenLastCalledWith("all", { at: row(2).at, id: 2 });
    expect(s.rows).toHaveLength(301);
    expect(s.rows.at(-1)?.id).toBe(1);
    expect(s.more).toBe(false);
  });

  it("reads the list again when a tab is selected", async () => {
    const t = setup(vi.fn, [row(3, "mention"), row(2, "automation_stopped"), row(1, "automation")]);
    await t.inbox.open();
    await t.inbox.select("automation");
    expect(t.port.list).toHaveBeenLastCalledWith("automation");
    expect(get(t.inbox.state).tab).toBe("automation");
    expect(ids(get(t.inbox.state))).toEqual([2, 1]);
  });

  it("drops an answer for a tab that is no longer selected", async () => {
    const t = setup(vi.fn, [row(2, "mention"), row(1, "automation")]);
    let release: (p: InboxPage) => void = () => {};
    t.port.list.mockImplementationOnce(() => new Promise<InboxPage>((r) => (release = r)));
    const opening = t.inbox.open();
    await t.inbox.select("automation");
    release({ rows: [row(2, "mention")], more: false, unread: get(t.inbox.unread) });
    await opening;
    expect(ids(get(t.inbox.state))).toEqual([1]);
  });

  it("toggles read and unread, and takes the unread counts from the new list", async () => {
    const t = setup(vi.fn, [row(2), row(1)]);
    await t.inbox.open();
    const first = get(t.inbox.state).rows[0];
    if (!first) throw new Error("no row");
    await t.inbox.toggle(first);
    expect(t.port.setRead).toHaveBeenLastCalledWith(2, true);
    expect(get(t.inbox.state).rows[0]?.read).toBe(true);
    expect(get(t.inbox.unread).all).toBe(1);

    await t.inbox.toggle(get(t.inbox.state).rows[0] ?? first);
    expect(t.port.setRead).toHaveBeenLastCalledWith(2, false);
    expect(get(t.inbox.unread).all).toBe(2);
  });

  it("marks only an unread row as read", async () => {
    const t = setup(vi.fn, [row(2, "automation", true), row(1)]);
    await t.inbox.open();
    const [read, unread] = get(t.inbox.state).rows;
    if (!read || !unread) throw new Error("no rows");
    await t.inbox.markRead(read);
    expect(t.port.setRead).not.toHaveBeenCalled();
    await t.inbox.markRead(unread);
    expect(t.port.setRead).toHaveBeenLastCalledWith(1, true);
    expect(get(t.inbox.unread).all).toBe(0);
  });

  it("reads all of the shown tab only, and canReadAll follows its unread count", async () => {
    const t = setup(vi.fn, [row(2, "mention"), row(1, "automation")]);
    await t.inbox.open();
    await t.inbox.select("mention");
    expect(get(t.inbox.canReadAll)).toBe(true);
    await t.inbox.readAll();
    expect(t.port.readAll).toHaveBeenCalledWith("mention");
    expect(get(t.inbox.unread)).toMatchObject({ all: 1, mention: 0, automation: 1 });
    expect(get(t.inbox.canReadAll)).toBe(false);
    await t.inbox.select("automation");
    expect(get(t.inbox.canReadAll)).toBe(true);
  });

  it("shows 適用しました after apply and reads the rows again", async () => {
    const t = setup(vi.fn, [row(1, "automation_stopped")]);
    await t.inbox.open();
    const r = get(t.inbox.state).rows[0];
    if (!r) throw new Error("no row");
    await t.inbox.apply(r);
    expect(t.port.apply).toHaveBeenCalledWith(1);
    expect(get(t.inbox.state).rows[0]?.read).toBe(true);
    expect(get(t.toasts.list).map((x) => [x.text, x.error])).toEqual([["適用しました", false]]);
  });

  it("shows a refusal as an error toast and reads the rows again", async () => {
    const t = setup(vi.fn, [row(1, "automation_stopped")]);
    await t.inbox.open();
    const r = get(t.inbox.state).rows[0];
    if (!r) throw new Error("no row");
    const refusal = "HH-1 は先に変更されたため、適用しませんでした";
    t.port.apply.mockRejectedValueOnce(new Refused(refusal));
    t.change([{ ...r, read: true }]);
    await t.inbox.apply(r);
    expect(get(t.toasts.list).map((x) => [x.text, x.error])).toEqual([[refusal, true]]);
    expect(get(t.inbox.state).rows[0]?.read).toBe(true);
  });

  it("shows other failures as an error toast without throwing, and reads the rows again", async () => {
    const t = setup(vi.fn, [row(1)]);
    await t.inbox.open();
    const r = get(t.inbox.state).rows[0];
    if (!r) throw new Error("no row");
    t.port.setRead.mockRejectedValueOnce(new Error("500 Internal Server Error"));
    t.change([{ ...r, read: true }]);
    await expect(t.inbox.toggle(r)).resolves.toBeUndefined();
    expect(get(t.toasts.list).map((x) => [x.text, x.error])).toEqual([
      ["500 Internal Server Error", true],
    ]);
    expect(get(t.inbox.state).rows[0]?.read).toBe(true);
  });

  it("reads the rows again without a toast when the notice is gone", async () => {
    const t = setup(vi.fn, [row(2), row(1)]);
    await t.inbox.open();
    const r = get(t.inbox.state).rows[0];
    if (!r) throw new Error("no row");
    t.port.keep.mockRejectedValueOnce(new NotFound("not found"));
    t.change([row(1)]);
    await t.inbox.keep(r);
    expect(get(t.toasts.list)).toEqual([]);
    expect(ids(get(t.inbox.state))).toEqual([1]);
  });

  it("keeps without a toast and reads the rows again", async () => {
    const t = setup(vi.fn, [row(1, "automation_stopped")]);
    await t.inbox.open();
    const r = get(t.inbox.state).rows[0];
    if (!r) throw new Error("no row");
    await t.inbox.keep(r);
    expect(t.port.keep).toHaveBeenCalledWith(1);
    expect(get(t.inbox.state).rows[0]?.read).toBe(true);
    expect(get(t.toasts.list)).toEqual([]);
  });

  it("reloads as many rows as were shown after more", async () => {
    const t = setup(vi.fn, rows(650));
    await t.inbox.open();
    await t.inbox.more();
    expect(get(t.inbox.state).rows).toHaveLength(600);
    await t.inbox.reload();
    expect(get(t.inbox.state)).toMatchObject({ more: true, loading: false });
    expect(get(t.inbox.state).rows).toHaveLength(600);
  });

  it("shows a failure to read as an error toast without throwing", async () => {
    const t = setup(vi.fn, rows(301));
    t.port.list.mockRejectedValueOnce(new Error("500 Internal Server Error"));
    await expect(t.inbox.open()).resolves.toBeUndefined();
    expect(get(t.inbox.state).loading).toBe(false);
    await t.inbox.open();
    t.port.list.mockRejectedValueOnce(new Error("通信できません"));
    await expect(t.inbox.more()).resolves.toBeUndefined();
    t.port.list.mockRejectedValueOnce(new Error("通信できません"));
    await expect(t.inbox.select("mention")).resolves.toBeUndefined();
    expect(get(t.toasts.list).map((x) => [x.text, x.error])).toEqual([
      ["500 Internal Server Error", true],
      ["通信できません", true],
      ["通信できません", true],
    ]);
    expect(get(t.inbox.state)).toMatchObject({ tab: "mention", loading: false });
  });

  it("reloads the unread counts alone", async () => {
    const t = setup(vi.fn, [row(1)]);
    await t.inbox.reloadUnread();
    expect(get(t.inbox.unread).all).toBe(1);
    expect(t.port.list).not.toHaveBeenCalled();
  });
});

describe("inbox text", () => {
  it("lists the tabs in the order of 06", () => {
    expect(inboxTabs.map((x) => x.label)).toEqual([
      "すべて",
      "メンション",
      "割り当て",
      "ウォッチ",
      "上書き",
      "自動化",
      "同期の修復",
    ]);
    expect(inboxTabs.map((x) => x.id)).toEqual([
      "all",
      "mention",
      "assign",
      "watch",
      "overwrite",
      "automation",
      "repair",
    ]);
  });

  it("names the pill of each kind", () => {
    expect(pillOf("automation_stopped")).toBe("自動化");
    expect(pillOf("automation")).toBe("自動化");
    expect(pillOf("mention")).toBe("メンション");
    expect(pillOf("assign")).toBe("割り当て");
    expect(pillOf("watch")).toBe("ウォッチ");
    expect(pillOf("overwrite")).toBe("上書き");
    expect(pillOf("repair")).toBe("同期の修復");
  });

  it("explains an empty tab", () => {
    expect(emptyOf("all")).toEqual({ title: "お知らせはありません" });
    expect(emptyOf("overwrite")).toEqual({
      title: "お知らせはありません",
      note: "このマシンで変えた値が、同期であとから届いた変更に負けると、ここに出ます。試すには、値を変えてからサイドバーの同期ボタンを押してください。",
    });
    expect(emptyOf("repair")).toEqual({
      title: "お知らせはありません",
      note: "同期のときに直したもの(番号の重なり、削除された親、親の循環、移動した親に合わせて移した子など)がここに出ます。",
    });
  });
});
