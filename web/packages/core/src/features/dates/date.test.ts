import { describe, expect, it } from "vitest";
import { dayOf, formatDay } from "./date";

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
