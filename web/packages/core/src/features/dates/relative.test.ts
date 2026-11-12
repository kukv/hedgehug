import { describe, expect, it } from "vitest";
import { olderThanADay, relativeTime } from "./relative";

// 時刻は、このマシンの時間帯の日付で表すので、テストは時間帯を持たない書き方(ローカル時刻)で作る。
const now = new Date(2026, 10, 13, 12, 0, 0);
const ago = (ms: number) => new Date(now.getTime() - ms);
const min = 60_000;
const hour = 60 * min;
const day = 24 * hour;

describe("relativeTime (01 の表)", () => {
  it.each([
    [0, "たった今"],
    [59_999, "たった今"],
    [min, "1 分前"],
    [59 * min, "59 分前"],
    [hour, "1 時間前"],
    [23 * hour + 59 * min, "23 時間前"],
    [day, "1 日前"],
    [6 * day, "6 日前"],
  ])("%d ms ago is %s", (ms, want) => {
    expect(relativeTime(ago(ms), now)).toBe(want);
  });

  it("shows the date from 7 days", () => {
    expect(relativeTime(new Date(2026, 10, 6, 12, 0, 0), now)).toBe("11/6");
    expect(relativeTime(new Date(2025, 11, 31, 9, 0, 0), now)).toBe("2025/12/31");
  });

  it("treats a time in the future as just now", () => {
    expect(relativeTime(new Date(now.getTime() + 5 * min), now)).toBe("たった今");
  });
});

describe("olderThanADay (06: 経過時間で判定する)", () => {
  it("is true from 24 hours", () => {
    expect(olderThanADay(ago(day - 1), now)).toBe(false);
    expect(olderThanADay(ago(day), now)).toBe(true);
  });
});
