import { describe, expect, it } from "vitest";
import { dayOf, formatDateTime, formatDay } from "./date";

describe("formatDay", () => {
  it("shows M/D this year, YYYY/MM/DD otherwise, and なし when empty", () => {
    expect(formatDay("2026-03-05", 2026)).toBe("3/5");
    expect(formatDay("2025-12-31", 2026)).toBe("2025/12/31");
    expect(formatDay("", 2026)).toBe("なし");
  });
});

describe("dayOf", () => {
  it("is the YYYY-MM-DD of the machine's time zone", () => {
    expect(dayOf(new Date(2026, 0, 5, 23, 59))).toBe("2026-01-05");
  });
});

describe("formatDateTime (01「日付と時刻」)", () => {
  const now = new Date(2026, 10, 14, 12, 0);
  it("shows M/D HH:MM for this year and YYYY/MM/DD HH:MM for other years", () => {
    expect(formatDateTime(new Date(2026, 10, 13, 9, 5), now)).toBe("11/13 09:05");
    expect(formatDateTime(new Date(2025, 11, 31, 23, 59), now)).toBe("2025/12/31 23:59");
  });

  it("uses the date and time of this machine's time zone, early in the morning too", () => {
    expect(formatDateTime(new Date(2026, 0, 1, 0, 30), now)).toBe("1/1 00:30");
  });
});
