import { describe, expect, it } from "vitest";
import type { Worklog } from "../../model/notes";
import { formatHours, hoursOf, parseHours, stepHours, worklogHeading } from "./hours";

const empty = "時間を入力してください";
const scale = "時間は小数第 2 位までで入力してください";

describe("parseHours", () => {
  it("reads a decimal into hundredths of an hour", () => {
    const ok: Record<string, number> = {
      "1": 100,
      "1.5": 150,
      "0.25": 25,
      "0.1": 10,
      ".5": 50,
      "1.": 100,
      "1.250": 125,
      "0.07": 7,
      " 2 ": 200,
      "12": 1200,
      "00.5": 50,
      "999999999": 99999999900,
    };
    for (const [input, hundredths] of Object.entries(ok)) {
      expect(parseHours(input), input).toEqual({ ok: true, hundredths });
    }
  });

  it("refuses an empty, non-numeric or non-positive input", () => {
    for (const input of [
      "",
      " ",
      "0",
      "0.0",
      "0.000",
      "-1",
      "-0.5",
      "abc",
      "1e2",
      "1,5",
      ".",
      "１",
      "1000000000",
    ]) {
      expect(parseHours(input), input).toEqual({ ok: false, message: empty });
    }
  });

  it("refuses a nonzero digit past the second decimal place", () => {
    for (const input of ["0.123", "1.005", "0.001", "2.999"]) {
      expect(parseHours(input), input).toEqual({ ok: false, message: scale });
    }
  });
});

describe("formatHours", () => {
  it("shows two decimals at most and drops trailing zeros", () => {
    const want: Record<number, string> = {
      0: "0",
      7: "0.07",
      10: "0.1",
      25: "0.25",
      30: "0.3",
      100: "1",
      110: "1.1",
      125: "1.25",
      200: "2",
      1234: "12.34",
    };
    for (const [h, s] of Object.entries(want)) expect(formatHours(Number(h))).toBe(s);
  });

  it("formats a number of hours the same way", () => {
    expect(hoursOf(6)).toBe("6");
    expect(hoursOf(1.5)).toBe("1.5");
    expect(hoursOf(0.29)).toBe("0.29");
  });
});

describe("stepHours", () => {
  it("moves by a quarter hour and never below zero", () => {
    expect(stepHours("", 1)).toBe("0.25");
    expect(stepHours("1", 1)).toBe("1.25");
    expect(stepHours("1.25", -1)).toBe("1");
    expect(stepHours("0.25", -1)).toBe("");
    expect(stepHours("0", -1)).toBe("");
    expect(stepHours("abc", 1)).toBe("0.25");
    expect(stepHours("0.1", 1)).toBe("0.35");
  });
});

function w(id: string, hours: string): Worklog {
  return { id, by: "hanako", byName: "花子", date: "2026-11-13", hours, memo: "" };
}

describe("worklogHeading", () => {
  it("adds the hundredths, then the estimate and the points when there are any", () => {
    const rows = [w("1", "0.3"), w("2", "1.25"), w("3", "0.45")];
    expect(worklogHeading(rows, { estimate: null, points: null })).toBe("2 時間");
    expect(worklogHeading(rows, { estimate: 6, points: 3 })).toBe(
      "2 時間 · 見積もり 6 時間 · 3 pt",
    );
    expect(worklogHeading([], { estimate: 1.5, points: null })).toBe("0 時間 · 見積もり 1.5 時間");
    expect(worklogHeading(rows, { estimate: null, points: 0.5 })).toBe("2 時間 · 0.5 pt");
  });

  it("adds in hundredths so that 0.1 + 0.2 is 0.3", () => {
    expect(worklogHeading([w("1", "0.1"), w("2", "0.2")], { estimate: null, points: null })).toBe(
      "0.3 時間",
    );
  });
});
