import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeLinks, row } from "./fixtures.test-util";
import { createLinkList } from "./links";
import {
  checksLabel,
  linkText,
  linkUrl,
  markerOf,
  reviewLabel,
  showsProgress,
  stateLabel,
} from "./text";

// flush は、マイクロタスクを回す(core は setTimeout の型を持たない)。
const flush = async () => {
  for (let i = 0; i < 50; i++) await Promise.resolve();
};

function setup(accounts: unknown[] = [{ id: "A1" }]) {
  const links = fakeLinks();
  const toasts: { text: string; error: boolean }[] = [];
  let changed = 0;
  const list = createLinkList({
    links: links.port,
    accounts: { list: async () => accounts as never },
    toasts: {
      show: (text: string, o?: { error?: boolean }) => toasts.push({ text, error: !!o?.error }),
    } as never,
    onChanged: () => {
      changed += 1;
    },
  });
  return { links, list, toasts, changed: () => changed };
}

describe("text", () => {
  it("writes the row words of 07", () => {
    expect(linkText(row())).toBe("org/web#41 — Add login form");
    expect(linkUrl(row())).toBe("https://github.com/org/web/pull/41");
    expect(linkUrl(row({ kind: "issue", number: 3 }))).toBe("https://github.com/org/web/issues/3");
    expect(stateLabel.merged).toBe("Merged");
    expect([
      reviewLabel("none"),
      reviewLabel("pending"),
      reviewLabel("approved"),
      reviewLabel("changes_requested"),
    ]).toEqual(["", "レビュー待ち", "承認済み", "修正依頼"]);
    expect([
      checksLabel(""),
      checksLabel("success"),
      checksLabel("failure"),
      checksLabel("pending"),
    ]).toEqual(["", "CI 成功", "CI 失敗", "CI 実行中"]);
    expect([
      showsProgress(row()),
      showsProgress(row({ state: "draft" })),
      showsProgress(row({ state: "merged" })),
      showsProgress(row({ kind: "issue" })),
    ]).toEqual([true, true, false, false]);
    expect(markerOf("01ABC")).toBe("hedgehug:01ABC");
  });
});

describe("createLinkList", () => {
  it("shows the stored rows first, then the refreshed ones", async () => {
    const t = setup();
    t.links.refreshed = [row({ state: "merged", error: "" })];
    await t.list.load("S1", "I1");
    expect(t.links.calls).toEqual(["list S1 I1", "refresh S1 I1"]);
    expect(get(t.list.state)).toMatchObject({
      loaded: true,
      hasAccount: true,
      rows: [row({ state: "merged" })],
    });
  });

  it("keeps the refresh errors when the list is read again", async () => {
    const t = setup();
    t.links.refreshed = [row({ error: "状態を更新できませんでした: x" })];
    await t.list.load("S1", "I1");
    await t.list.reload();
    expect(get(t.list.state).rows[0]?.error).toBe("状態を更新できませんでした: x");
  });

  it("knows when this machine has no account", async () => {
    const t = setup([]);
    await t.list.load("S1", "I1");
    expect(get(t.list.state).hasAccount).toBe(false);
  });

  it("removes with a toast and reads again", async () => {
    const t = setup();
    await t.list.load("S1", "I1");
    await t.list.remove(row());
    expect(t.links.calls).toContain("remove S1 I1 pr:org/web#41");
    expect(t.toasts).toEqual([{ text: "リンクを解除しました", error: false }]);
    expect(t.changed()).toBe(1);
  });

  it("shows a refusal, keeps quiet when the item is gone, and shows the fallback otherwise", async () => {
    const t = setup();
    await t.list.load("S1", "I1");
    t.links.fail = new Refused("断り");
    await t.list.remove(row());
    t.links.fail = new NotFound();
    await t.list.remove(row());
    t.links.fail = new Error("x");
    await t.list.remove(row());
    expect(t.toasts).toEqual([
      { text: "断り", error: true },
      { text: "リンクを解除できませんでした", error: true },
    ]);
  });

  it("keeps the refreshed rows when the list is read during the refresh", async () => {
    const t = setup();
    let release: () => void = () => undefined;
    t.links.port.refresh = () =>
      new Promise((resolve) => {
        release = () => resolve([row({ state: "merged", error: "状態を更新できませんでした: x" })]);
      });
    const loading = t.list.load("S1", "I1");
    await flush(); // 一覧を出し、取り直しを待つところまで進める
    await t.list.reload(); // 取り直しの途中の画面への反映
    release();
    await loading;
    expect(get(t.list.state).rows[0]).toMatchObject({
      state: "merged",
      error: "状態を更新できませんでした: x",
    });
  });

  it("forgets a row's refresh error when it is removed and linked again", async () => {
    const t = setup();
    t.links.refreshed = [row({ error: "状態を更新できませんでした: x" })];
    await t.list.load("S1", "I1");
    await t.list.remove(row()); // 解除
    await t.list.reload(); // 付け直したあとの読み直し(偽の一覧は同じ行を返す)
    expect(get(t.list.state).rows[0]?.error).toBe("");
  });

  it("forgets every refresh error with forgetAll", async () => {
    const t = setup();
    t.links.refreshed = [row({ error: "状態を更新できませんでした: x" })];
    await t.list.load("S1", "I1");
    t.list.forgetAll();
    await t.list.reload();
    expect(get(t.list.state).rows[0]?.error).toBe("");
  });

  it("shows no rows when the item is gone on refresh or reload", async () => {
    const t = setup();
    t.links.port.refresh = async () => {
      throw new NotFound();
    };
    await t.list.load("S1", "I1");
    expect(get(t.list.state)).toMatchObject({ loaded: true, rows: [] });
    t.links.rows = [row()];
    await t.list.reload();
    expect(get(t.list.state).rows).toHaveLength(1);
    t.links.fail = new NotFound();
    await t.list.reload();
    expect(get(t.list.state)).toMatchObject({ loaded: true, rows: [] });
  });

  it("assumes an account again after reset", async () => {
    const t = setup([]);
    await t.list.load("S1", "I1");
    t.list.reset();
    expect(get(t.list.state).hasAccount).toBe(true);
  });

  it("drops answers for an item that is no longer open", async () => {
    const t = setup();
    const first = t.list.load("S1", "I1");
    t.list.reset();
    await first;
    expect(get(t.list.state)).toMatchObject({ loaded: false, rows: [] });
  });
});
