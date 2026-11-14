import { describe, expect, it } from "vitest";
import type { ReportSprint } from "../../model/report";
import { burndownTable, heading, velocityTable } from "./tables";

const sprint = (name: string): ReportSprint => ({
  id: name,
  name,
  goal: "",
  start: "2026-11-02",
  end: "2026-11-15",
  state: "closed",
  confirmed: null,
});

describe("表", () => {
  it("heading は単位を添える", () => {
    expect(heading("総量", "points")).toBe("総量 (pt)");
    expect(heading("完了", "count")).toBe("完了 (件)");
  });

  it("burndownTable: 表示する日より後の残りは「—」、理想は常に小数第 1 位まで(F3-22)", () => {
    const days = [
      { date: "2026-11-02", total: 8, remaining: 8, ideal: 8 },
      { date: "2026-11-03", total: 8, remaining: 5.5, ideal: 5.3 },
      { date: "2026-11-04", total: 8.25, remaining: null, ideal: 2.7 },
      { date: "2026-11-05", total: 8, remaining: null, ideal: 0 },
    ];
    expect(burndownTable(days, "points", 2026)).toEqual({
      columns: ["日付", "総量 (pt)", "残り (pt)", "理想 (pt)"],
      rows: [
        ["11/2", "8", "8", "8.0"],
        ["11/3", "8", "5.5", "5.3"],
        ["11/4", "8.25", "—", "2.7"],
        ["11/5", "8", "—", "0.0"],
      ],
    });
    expect(burndownTable([], "count", 2026).columns).toEqual([
      "日付",
      "総量 (件)",
      "残り (件)",
      "理想 (件)",
    ]);
  });

  it("burndownTable: 数は浮動小数点の誤差を出さない", () => {
    const days = [{ date: "2026-11-02", total: 0.1 + 0.2, remaining: 0.1 + 0.2, ideal: 0.3 }];
    expect(burndownTable(days, "points", 2026).rows[0]).toEqual(["11/2", "0.3", "0.3", "0.3"]);
  });

  it("velocityTable", () => {
    const bars = [
      { sprint: sprint("Sprint 1"), planned: 10, done: 7.5 },
      { sprint: sprint("Sprint 2"), planned: 3, done: 0 },
    ];
    expect(velocityTable(bars, "count")).toEqual({
      columns: ["スプリント", "計画 (件)", "完了 (件)"],
      rows: [
        ["Sprint 1", "10", "7.5"],
        ["Sprint 2", "3", "0"],
      ],
    });
  });
});
