import { describe, expect, it } from "vitest";
import { checkPeriod, defaultPeriod, periodLabel } from "./flowPeriod";

describe("defaultPeriod", () => {
  it("今日の 20 日前から今日まで", () => {
    expect(defaultPeriod("2026-11-14")).toEqual({ from: "2026-10-25", to: "2026-11-14" });
  });
  it("月と年をまたぐ", () => {
    expect(defaultPeriod("2027-01-05")).toEqual({ from: "2026-12-16", to: "2027-01-05" });
  });
});

describe("checkPeriod", () => {
  const today = "2026-11-14";
  it("開始が空なら、まず開始日の文", () => {
    expect(checkPeriod("", "", today)).toBe("開始日を入力してください");
  });
  it("終了が空", () => {
    expect(checkPeriod("2026-11-01", "", today)).toBe("終了日を入力してください");
  });
  it("終了日が開始日より前", () => {
    expect(checkPeriod("2026-11-10", "2026-11-09", today)).toBe("終了日は開始日以降にしてください");
  });
  it("終了日が今日より後", () => {
    expect(checkPeriod("2026-11-10", "2026-11-15", today)).toBe("終了日は今日以前にしてください");
  });
  it("同じ日と今日までは認める", () => {
    expect(checkPeriod("2026-11-14", "2026-11-14", today)).toBeNull();
  });
});

describe("periodLabel", () => {
  it("初めの期間(null)", () => {
    expect(periodLabel(null, 2026)).toBe("直近 21 日・件数");
  });
  it("変えた期間は日付の書き方で", () => {
    expect(periodLabel({ from: "2026-11-01", to: "2026-11-10" }, 2026)).toBe("11/1 〜 11/10・件数");
    expect(periodLabel({ from: "2025-12-31", to: "2026-11-10" }, 2026)).toBe(
      "2025/12/31 〜 11/10・件数",
    );
  });
});
