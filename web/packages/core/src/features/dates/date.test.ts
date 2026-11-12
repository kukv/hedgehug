import { describe, expect, it } from "vitest";
import { formatDay } from "./date";

describe("formatDay", () => {
  it("shows M/D this year, YYYY/MM/DD otherwise, and なし when empty", () => {
    expect(formatDay("2026-03-05", 2026)).toBe("3/5");
    expect(formatDay("2025-12-31", 2026)).toBe("2025/12/31");
    expect(formatDay("", 2026)).toBe("なし");
  });
});
