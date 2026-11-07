import { describe, expect, it } from "vitest";
import {
  authorLabel,
  automationOf,
  groupByDay,
  type Log,
  opLabel,
  rejectionsOf,
  trailerLine,
} from "./activity";

describe("the trailers of the automations", () => {
  const trailers = [
    { key: "Hedgehug-Op", value: "gh-fetch" },
    { key: "Hedgehug-Automation", value: "pr-opened pr.opened" },
    { key: "Hedgehug-Automation-Rejected", value: "U1 pr-merged points: required" },
    { key: "Hedgehug-Automation-Rejected", value: "U2 pr-merged points: required" },
  ];

  it("reads the rule and the refusals", () => {
    const l = log({ trailers });
    expect(automationOf(l)).toEqual({ rule: "pr-opened", event: "pr.opened" });
    expect(rejectionsOf(l).map((r) => r.task)).toEqual(["U1", "U2"]);
  });

  it("finds nothing on a commit of a person", () => {
    const l = log({ trailers: [{ key: "Hedgehug-Op", value: "set" }] });
    expect(automationOf(l)).toBeNull();
    expect(rejectionsOf(l)).toEqual([]);
  });
});

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
  it("names the ops in the words of the screen", () => {
    expect(opLabel("status")).toBe("ステータスの変更");
    expect(opLabel("dep")).toBe("依存の変更");
    expect(opLabel("schema")).toBe("プロジェクトの設定の変更");
    expect(opLabel("gh-mirror")).toBe("Issue からの反映");
    expect(opLabel("gh-push")).toBe("Issue への反映");
    expect(opLabel("sprint-close")).toBe("スプリントのクローズ");
    expect(opLabel("worklog-add")).toBe("作業時間の記録");
  });

  it("names every op that internal/app writes into a commit", () => {
    const ops = [
      "init",
      "add",
      "set",
      "edit",
      "bulk-set",
      "status",
      "dep",
      "mv",
      "rm",
      "restore",
      "purge",
      "revert-overwritten",
      "move-in",
      "move-out",
      "migrate",
      "schema",
      "undo",
      "workspace",
      "comment",
      "watch",
      "unwatch",
      "view",
      "setup",
      "merge",
      "repair",
      "replay",
      "link",
      "unlink",
      "start",
      "automation",
      "gh-fetch",
      "gh-mirror",
      "gh-push",
      "worklog-add",
      "worklog-rm",
      "template-set",
      "template-rm",
      "sprint-add",
      "sprint-set",
      "sprint-rm",
      "sprint-start",
      "sprint-close",
      "release-add",
      "release-set",
      "release-rm",
    ];
    for (const op of ops) expect(opLabel(op), op).not.toBe(op);
  });

  it("shows an unknown op as it is, and a commit with none as 変更", () => {
    expect(opLabel("something-new")).toBe("something-new");
    expect(opLabel(null)).toBe("変更");
    expect(opLabel("")).toBe("変更");
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
