import { describe, expect, it } from "vitest";
import {
  avatarTone,
  formatDate,
  formatDateTime,
  formatRelative,
  formatTime,
  initials,
} from "./format";

const now = new Date(2026, 10, 4, 12, 0, 0); // 2026-11-04 12:00 local
const at = (y: number, mo: number, d: number, h = 0, mi = 0, s = 0) =>
  new Date(y, mo - 1, d, h, mi, s).toISOString();

describe("formatDate", () => {
  it("leaves out the year of this year", () => {
    expect(formatDate("2026-11-07", now)).toBe("11/07");
    expect(formatDate(at(2026, 1, 5, 9), now)).toBe("01/05");
  });

  it("shows the year of another year", () => {
    expect(formatDate("2027-01-29", now)).toBe("2027/01/29");
    expect(formatDate(at(2025, 12, 31, 23), now)).toBe("2025/12/31");
  });
});

describe("formatDateTime", () => {
  it("shows the time too", () => {
    expect(formatDateTime(at(2026, 11, 4, 10, 12), now)).toBe("11/04 10:12");
    expect(formatDateTime(at(2025, 3, 2, 9, 5), now)).toBe("2025/03/02 09:05");
  });
});

describe("formatTime", () => {
  it("pads the hour and the minute, and never writes 24", () => {
    expect(formatTime(at(2026, 11, 4, 9, 5))).toBe("09:05");
    expect(formatTime(at(2026, 11, 4, 0, 5))).toBe("00:05");
    expect(formatTime(at(2026, 11, 4, 23, 59))).toBe("23:59");
  });

  it("is --:-- for a time that is none", () => {
    expect(formatTime("not a time")).toBe("--:--");
  });
});

describe("formatRelative", () => {
  it("says now, minutes, hours, then the date", () => {
    expect(formatRelative(at(2026, 11, 4, 11, 59, 30), now)).toBe("今");
    expect(formatRelative(at(2026, 11, 4, 12, 0, 30), now)).toBe("今"); // a clock a little ahead
    expect(formatRelative(at(2026, 11, 4, 11, 57), now)).toBe("3 分前");
    expect(formatRelative(at(2026, 11, 4, 10, 0), now)).toBe("2 時間前");
    expect(formatRelative(at(2026, 11, 3, 11, 0), now)).toBe("11/03");
  });
});

describe("initials and avatarTone", () => {
  it("takes the first two characters, in capitals", () => {
    expect(initials("alice")).toBe("AL");
    expect(initials("田中太郎")).toBe("田中");
    expect(initials("  bob ")).toBe("BO");
    expect(initials("x")).toBe("X");
  });

  it("gives the same tone for the same name, within 0 to 3", () => {
    for (const n of ["alice", "bob", "田中太郎", "x", ""]) {
      const t = avatarTone(n);
      expect([0, 1, 2, 3]).toContain(t);
      expect(avatarTone(n)).toBe(t);
    }
    expect(new Set(["a", "b", "c", "d"].map(avatarTone)).size).toBe(4);
  });
});
