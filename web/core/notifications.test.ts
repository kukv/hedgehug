import { describe, expect, it } from "vitest";
import {
  filterByTab,
  formatWhen,
  type Notification,
  notificationText,
  sortNotifications,
  tabCounts,
  tabsOf,
} from "./notifications";

function n(over: Partial<Notification> & { ulid: string }): Notification {
  return {
    author: "",
    commit: "c",
    kind: "overwritten",
    read: false,
    store: "shared",
    task: { ulid: "T1", id: "REL-1", title: "title" },
    time: "2026-11-04T10:00:00Z",
    to: "alice",
    ...over,
  };
}

describe("tabsOf", () => {
  it.each([
    ["overwritten", ["all", "overwritten"]],
    ["mention", ["all", "mention"]],
    ["repaired", ["all", "repair"]],
    ["revived", ["all", "repair"]],
    ["renumbered", ["all", "repair"]],
    ["prefix_duplicate", ["all", "repair"]],
    ["rewrite_applied", ["all", "repair"]],
    ["purged_change", ["all", "repair"]],
    ["assigned", ["all", "assigned"]],
    ["watch", ["all", "watch"]],
    ["automation", ["all", "automation"]],
    ["automation_rejected", ["all", "automation"]],
    ["issue_overwritten", ["all", "automation"]],
    ["something_new", ["all"]],
  ])("%s", (kind, tabs) => {
    expect(tabsOf(kind)).toEqual(tabs);
  });
});

describe("sortNotifications", () => {
  it("puts the unread first, then the newest, then the larger ULID", () => {
    const items = [
      n({ ulid: "A", read: true, time: "2026-11-04T12:00:00Z" }),
      n({ ulid: "B", time: "2026-11-04T09:00:00Z" }),
      n({ ulid: "C", time: "2026-11-04T11:00:00Z" }),
      n({ ulid: "D", time: "2026-11-04T11:00:00Z" }),
    ];
    expect(sortNotifications(items).map((x) => x.ulid)).toEqual(["D", "C", "B", "A"]);
  });

  it("leaves its argument as it was", () => {
    const items = [n({ ulid: "A", read: true }), n({ ulid: "B" })];
    sortNotifications(items);
    expect(items.map((x) => x.ulid)).toEqual(["A", "B"]);
  });
});

describe("tabCounts and filterByTab", () => {
  const items = [
    n({ ulid: "1", kind: "overwritten" }),
    n({ ulid: "2", kind: "overwritten", read: true }),
    n({ ulid: "3", kind: "mention" }),
    n({ ulid: "4", kind: "assigned" }),
    n({ ulid: "5", kind: "revived" }),
    n({ ulid: "6", kind: "watch", read: true }),
  ];
  it("counts the unread of each tab", () => {
    expect(tabCounts(items)).toEqual({
      all: 4,
      mention: 1,
      assigned: 1,
      watch: 0,
      overwritten: 1,
      automation: 0,
      repair: 1,
    });
  });
  it("assigned and watch are in the all tab only", () => {
    expect(filterByTab(items, "all")).toHaveLength(6);
    expect(filterByTab(items, "overwritten").map((x) => x.ulid)).toEqual(["1", "2"]);
    expect(filterByTab(items, "mention").map((x) => x.ulid)).toEqual(["3"]);
    expect(filterByTab(items, "repair").map((x) => x.ulid)).toEqual(["5"]);
  });
});

describe("notificationText", () => {
  it("names the rule, the status and the event of an automation", () => {
    const t = notificationText(
      n({
        ulid: "1",
        kind: "automation",
        detail: { rule: "pr-opened", event: "pr.opened", status: "in_progress" },
      }),
      { statusLabel: (s) => (s === "in_progress" ? "進行中" : s) },
    );
    expect(t).toEqual({
      label: "自動化",
      text: "ルール pr-opened が 進行中 にしました（PR が開かれた）",
    });
  });

  it("tells why a rule was refused", () => {
    const t = notificationText(
      n({
        ulid: "1",
        kind: "automation_rejected",
        detail: { rule: "pr-merged", reason: "points: required" },
      }),
    );
    expect(t).toEqual({
      label: "自動化を拒否",
      text: "ルール pr-merged で動かせませんでした: points: required",
    });
  });

  it("tells which side of an issue was written over, and what was lost", () => {
    const item = (side: string) =>
      n({
        ulid: "1",
        kind: "issue_overwritten",
        detail: {
          ref: "github.com/org-a/web/issues/45",
          field: "title",
          side,
          lost: ["手で直した題"],
        },
      });
    // a notification made while the labels were still synced is worded in Japanese too
    const old = n({
      ulid: "2",
      kind: "issue_overwritten",
      detail: {
        ref: "github.com/org-a/web/issues/45",
        field: "labels",
        side: "hedgehug",
        lost: ["v2"],
      },
    });
    expect(notificationText(old)).toEqual({
      label: "自動化",
      text: "タスクのラベルが手で編集されていたため、org-a/web#45 の値に戻しました",
      mine: "ラベル: v2",
    });
    const comment = n({
      ulid: "3",
      kind: "issue_overwritten",
      detail: {
        ref: "github.com/org-a/web/issues/45",
        field: "comments:01JABCDEFGHJKMNPQRSTVWXYZ0",
        comment: "手元の本文を ここに書く",
        side: "github",
        lost: ["GitHub で直した本文"],
      },
    });
    expect(notificationText(comment)).toEqual({
      label: "自動化",
      text: "org-a/web#45 のコメント（手元の本文を ここに書く）が GitHub 上で手で編集されていたため、Hedgehug の値で上書きしました",
      mine: "コメント（手元の本文を ここに書く）: GitHub で直した本文",
    });
    // a mirror that wrote GitHub's text over a hand edit of a comment
    const mirrored = n({
      ulid: "5",
      kind: "issue_overwritten",
      detail: {
        ref: "github.com/org-a/web/issues/45",
        field: "comments:01JABCDEFGHJKMNPQRSTVWXYZ0",
        comment: "GitHub の本文",
        side: "task",
        lost: ["手で直した本文"],
      },
    });
    expect(notificationText(mirrored)).toEqual({
      label: "自動化",
      text: "タスクのコメント（GitHub の本文）が手で編集されていたため、org-a/web#45 の値に戻しました",
      mine: "コメント（GitHub の本文）: 手で直した本文",
    });
    // an item made before the text was kept names no ULID
    const oldComment = n({
      ulid: "4",
      kind: "issue_overwritten",
      detail: {
        ref: "github.com/org-a/web/issues/45",
        field: "comments:01JABCDEFGHJKMNPQRSTVWXYZ0",
        side: "github",
        lost: ["x"],
      },
    });
    expect(notificationText(oldComment).mine).toBe("コメント: x");
    expect(notificationText(item("github"))).toEqual({
      label: "自動化",
      text: "org-a/web#45 のタイトルが GitHub 上で手で編集されていたため、Hedgehug の値で上書きしました",
      mine: "タイトル: 手で直した題",
    });
    expect(notificationText(item("task")).text).toBe(
      "タスクのタイトルが手で編集されていたため、org-a/web#45 の値に戻しました",
    );
  });

  it("overwritten: the field, and my value as it was", () => {
    const t = notificationText(
      n({ ulid: "1", kind: "overwritten", detail: { field: "points", value: 2 } }),
    );
    expect(t.label).toBe("上書き");
    expect(t.text).toBe("自分が設定した points が、同時に編集された別の値で上書きされました");
    expect(t.mine).toBe("points: 2（自分）");
  });

  it("values: a string as it is, a list with commas, null as empty", () => {
    const mine = (value: unknown) =>
      notificationText(n({ ulid: "1", kind: "overwritten", detail: { field: "f", value } })).mine;
    expect(mine("abc")).toBe("f: abc（自分）");
    expect(mine(["a", "b"])).toBe("f: a, b（自分）");
    expect(mine(null)).toBe("f: 空（自分）");
    expect(mine({ x: 1 })).toBe('f: {"x":1}（自分）');
  });

  it("overwritten with no detail has no value to show", () => {
    const t = notificationText(n({ ulid: "1", kind: "overwritten" }));
    expect(t.mine).toBeUndefined();
    expect(t.text).toBe("同時に編集された別の値で上書きされました");
  });

  it("mention: by the author, in a comment or in the body", () => {
    expect(
      notificationText(
        n({ ulid: "1", kind: "mention", author: "tanaka", detail: { comment: "C1" } }),
      ),
    ).toMatchObject({ label: "メンション", text: "tanaka がコメントであなたをメンションしました" });
    expect(notificationText(n({ ulid: "1", kind: "mention", author: "tanaka" })).text).toBe(
      "tanaka が本文であなたをメンションしました",
    );
    expect(notificationText(n({ ulid: "1", kind: "mention" })).text).toBe(
      "本文であなたがメンションされました",
    );
  });

  it("assigned and watch", () => {
    expect(notificationText(n({ ulid: "1", kind: "assigned", author: "sato" }))).toMatchObject({
      label: "割り当て",
      text: "sato があなたを担当者に追加しました",
    });
    expect(
      notificationText(
        n({ ulid: "1", kind: "watch", author: "sato", detail: { changed: ["status", "due"] } }),
      ),
    ).toMatchObject({ label: "ウォッチ", text: "sato が status, due を変更しました" });
    expect(
      notificationText(n({ ulid: "1", kind: "watch", author: "sato", detail: { comment: "C" } }))
        .text,
    ).toBe("sato がコメントしました");
    expect(notificationText(n({ ulid: "1", kind: "watch", detail: { comment: "C" } })).text).toBe(
      "コメントがありました",
    );
  });

  it("repaired by its kind", () => {
    const t = (detail: unknown) =>
      notificationText(n({ ulid: "1", kind: "repaired", detail })).text;
    expect(t({ kind: "parent_moved" })).toBe("親が別の Store へ移ったため、ルートに移しました");
    expect(t({ kind: "ref_moved", value: { field: "blocked_by", target: "X" } })).toBe(
      "blocked_by が指していたタスクが別の Store へ移ったため、参照を外しました",
    );
    expect(t({ kind: "ref_moved" })).toBe("別の Store へ移ったタスクへの参照を外しました");
    expect(t({ kind: "sprint_detached", value: "S1" })).toBe(
      "スプリント S1 から外しました（移動先のワークスペースにはありません）",
    );
    expect(t({ kind: "release_detached", value: "v2" })).toBe(
      "リリース v2 から外しました（移動先のワークスペースにはありません）",
    );
    // a task that is not in the cache is named by its ULID
    expect(t({ kind: "parent_cycle", value: { from: "A", to: null } })).toBe(
      "親の循環を検出したため、ルートに戻しました（元の親: 「A」）",
    );
    expect(t({ kind: "parent_cycle", value: { from: "A", to: "B" } })).toBe(
      "親の循環を検出したため、「B」の下に戻しました（元の親: 「A」）",
    );
    expect(t({ kind: "parent_deleted", value: { from: "A", to: null } })).toBe(
      "親「A」が削除されたため、ルートに移しました",
    );
    expect(t({ kind: "parent_deleted", value: { from: "A", to: "B" } })).toBe(
      "親「A」が削除されたため、「B」の下に移しました",
    );
    expect(t({ kind: "parent_moved", value: { from: "A", to: "B" } })).toBe(
      "親「A」が別の Store へ移ったため、「B」の下に移しました",
    );
    expect(t({ kind: "parent_moved", value: { from: "A", to: null } })).toBe(
      "親「A」が別の Store へ移ったため、ルートに移しました",
    );
    // the kind that changed with it, and the titles from the cache
    const names = { A: "Old parent", B: "未分類" };
    expect(
      notificationText(
        n({
          ulid: "1",
          kind: "repaired",
          detail: {
            kind: "parent_deleted",
            value: { from: "A", to: "B", kind_from: "step", kind_to: "task" },
          },
        }),
        {
          taskName: (u) => names[u as "A" | "B"],
        },
      ).text,
    ).toBe(
      "親「Old parent」が削除されたため、「未分類」の下に移しました。種類を Step から Task に変えました",
    );
    expect(t({ kind: "other_repair", value: 3 })).toBe("同期の修復: other_repair（元の値: 3）");
    expect(t({ kind: "other_repair" })).toBe("同期の修復: other_repair");
    expect(t(undefined)).toBe("同期の修復");
    expect(notificationText(n({ ulid: "1", kind: "repaired" })).label).toBe("同期の修復");
  });

  it("revived, renumbered, rewrite_applied, purged_change", () => {
    expect(notificationText(n({ ulid: "1", kind: "revived" })).text).toBe(
      "削除と同時にあなたが編集していたため、編集を優先して復元しました",
    );
    expect(
      notificationText(n({ ulid: "1", kind: "renumbered", detail: { old: "REL-4", new: "REL-9" } }))
        .text,
    ).toBe("ID を REL-4 から REL-9 に振り直しました。REL-4 でも検索できます");
    expect(notificationText(n({ ulid: "1", kind: "rewrite_applied" })).text).toBe(
      "履歴の書き換え（purge）を取り込みました",
    );
    expect(notificationText(n({ ulid: "1", kind: "purged_change" })).text).toBe(
      "完全に削除されたタスクへの、未 push の変更を捨てました",
    );
  });

  it("prefix_duplicate names the workspace when it is known", () => {
    const item = n({
      ulid: "1",
      kind: "prefix_duplicate",
      task: null as never,
      detail: { kind: "prefix_duplicate", workspace: "W1", value: "REL" },
    });
    expect(notificationText(item).text).toBe("接頭辞がほかのワークスペースと重複しています");
    expect(
      notificationText(item, { workspaceName: (u) => (u === "W1" ? "release" : undefined) }).text,
    ).toBe("ワークスペース release の接頭辞がほかのワークスペースと重複しています");
    expect(
      notificationText(
        n({
          ulid: "1",
          kind: "prefix_duplicate",
          detail: { kind: "name_duplicate", workspace: "W1" },
        }),
      ).text,
    ).toBe("名前がほかのワークスペースと重複しています");
  });

  it("an unknown kind is shown by its name", () => {
    expect(notificationText(n({ ulid: "1", kind: "future_kind" }))).toEqual({
      label: "future_kind",
      text: "種類: future_kind",
    });
  });
});

describe("formatWhen", () => {
  const now = new Date(2026, 10, 4, 12, 0);
  it("today by the time, yesterday with the word, before that by the date", () => {
    expect(formatWhen(new Date(2026, 10, 4, 9, 5).toISOString(), now)).toBe("09:05");
    expect(formatWhen(new Date(2026, 10, 3, 18, 20).toISOString(), now)).toBe("昨日 18:20");
    expect(formatWhen(new Date(2026, 10, 1, 8, 0).toISOString(), now)).toBe("11/01 08:00");
    expect(formatWhen(new Date(2025, 11, 31, 8, 0).toISOString(), now)).toBe("2025/12/31 08:00");
  });
});
