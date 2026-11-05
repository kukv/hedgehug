import { describe, expect, it } from "vitest";
import { authorLabel, groupByDay, type Log, opLabel, trailerLine } from "./activity";

function log(over: Partial<Log> = {}): Log {
  return {
    author: { name: "koki", email: "koki@example.com" },
    date: "2026-11-04T10:30:00",
    hash: "0123456789abcdef0123456789abcdef01234567",
    id: null,
    op: "set",
    store: "shared",
    subject: "set REL-1",
    task: null,
    trailers: [],
    ...over,
  };
}

describe("opLabel", () => {
  it("names every op the server writes", () => {
    const table = Object.fromEntries(
      [
        "add",
        "set",
        "status",
        "mv",
        "move-in",
        "move-out",
        "rm",
        "restore",
        "comment",
        "merge",
        "repair",
        "schema",
        "view",
        "workspace",
        "purge",
        "revert-overwritten",
        "undo",
      ].map((o) => [o, opLabel(o)]),
    );
    expect(table).toEqual({
      add: "作成",
      set: "変更",
      status: "ステータスの変更",
      mv: "移動",
      "move-in": "別の Store から移動",
      "move-out": "別の Store へ移動",
      rm: "削除",
      restore: "復元",
      comment: "コメント",
      merge: "同期のマージ",
      repair: "同期の修復",
      schema: "スキーマの変更",
      view: "ビューの変更",
      workspace: "ワークスペースの変更",
      purge: "完全な削除",
      "revert-overwritten": "上書きを戻す",
      undo: "取り消し",
    });
  });

  it("names the other ops of internal/app/usecase and the sync, and shows an unknown one as it is", () => {
    expect(opLabel("edit")).toBe("本文の編集");
    expect(opLabel("dep")).toBe("依存の変更");
    expect(opLabel("watch")).toBe("ウォッチ");
    expect(opLabel("unwatch")).toBe("ウォッチの解除");
    expect(opLabel("migrate")).toBe("スキーマの移行");
    expect(opLabel("init")).toBe("Store の作成");
    expect(opLabel("setup")).toBe("同期の設定");
    expect(opLabel("replay")).toBe("同期のやり直し");
    expect(opLabel("link")).toBe("リンク");
    expect(opLabel("unlink")).toBe("リンクの解除");
    expect(opLabel("start")).toBe("作業の開始");
    expect(opLabel("gh-fetch")).toBe("GitHub の取り込み");
    expect(opLabel("something-new")).toBe("something-new");
    expect(opLabel(null)).toBe("変更");
  });
});

describe("authorLabel", () => {
  it("is the author, and 同期（author）for a merge and a repair", () => {
    expect(authorLabel(log())).toBe("koki");
    expect(authorLabel(log({ op: "merge" }))).toBe("同期（koki）");
    expect(authorLabel(log({ op: "repair" }))).toBe("同期（koki）");
    expect(authorLabel(log({ op: null }))).toBe("koki");
  });
});

describe("groupByDay", () => {
  const now = new Date(2026, 10, 4, 15, 0);

  it("heads the days: 今日, 昨日, then the date, with the year when it differs", () => {
    const logs = [
      log({ hash: "a", date: "2026-11-04T11:00:00" }),
      log({ hash: "b", date: "2026-11-04T09:00:00" }),
      log({ hash: "c", date: "2026-11-03T23:59:00" }),
      log({ hash: "d", date: "2026-11-02T08:00:00" }),
      log({ hash: "e", date: "2025-11-02T08:00:00" }),
    ];
    const days = groupByDay(logs, now);
    expect(days.map((d) => d.name)).toEqual([
      "今日 · 11/04（水）",
      "昨日 · 11/03（火）",
      "11/02（月）",
      "2025/11/02（日）",
    ]);
    expect(days.map((d) => d.logs.map((l) => l.hash))).toEqual([["a", "b"], ["c"], ["d"], ["e"]]);
  });

  it("puts the commits of a day together when the dates come out of order", () => {
    const days = groupByDay(
      [
        log({ hash: "a", date: "2026-11-04T11:00:00" }),
        log({ hash: "b", date: "2026-11-03T11:00:00" }),
        log({ hash: "c", date: "2026-11-04T09:00:00" }),
      ],
      now,
    );
    expect(days.map((d) => d.key)).toEqual(["2026-11-04", "2026-11-03"]);
    expect(days.map((d) => d.logs.map((l) => l.hash))).toEqual([["a", "c"], ["b"]]);
  });

  it("puts the commits with a date that is none in one day, 日時不明", () => {
    const days = groupByDay(
      [log({ hash: "a", date: "nope" }), log({ hash: "b", date: "" }), log({ hash: "c" })],
      now,
    );
    expect(days.map((d) => [d.key, d.name, d.logs.length])).toEqual([
      ["unknown", "日時不明", 2],
      ["2026-11-04", "今日 · 11/04（水）", 1],
    ]);
  });

  it("is empty for no logs", () => {
    expect(groupByDay([], now)).toEqual([]);
  });
});

describe("trailerLine", () => {
  it("is the 7 characters of the sha and the trailers as they are", () => {
    expect(trailerLine(log())).toBe("0123456");
    expect(
      trailerLine(
        log({
          trailers: [
            { key: "Hedgehug-Op", value: "merge" },
            { key: "Hedgehug-Overwritten", value: "REL-11 points 2 koki" },
          ],
        }),
      ),
    ).toBe("0123456 · Hedgehug-Op: merge · Hedgehug-Overwritten: REL-11 points 2 koki");
  });
});
