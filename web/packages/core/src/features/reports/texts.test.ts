import { describe, expect, it } from "vitest";
import type { Burndown, ReportActive, ReportSprint } from "../../model/report";
import {
  addedText,
  amount,
  averageText,
  burnStatus,
  burnTotals,
  committedNote,
  durationLines,
  fallbackNote,
  notStarted,
  pointsCard,
  unitLabel,
} from "./texts";

const sprint = (over: Partial<ReportSprint> = {}): ReportSprint => ({
  id: "SP4",
  name: "Sprint 4",
  goal: "",
  start: "2026-11-02",
  end: "2026-11-15",
  state: "active",
  confirmed: null,
  ...over,
});

describe("単位", () => {
  it("unitLabel と amount", () => {
    expect(unitLabel("points")).toBe("pt");
    expect(unitLabel("count")).toBe("件");
    expect(amount(12, "points")).toBe("12 pt");
    expect(amount(0.5, "count")).toBe("0.5 件");
  });
  it("fallbackNote", () => {
    expect(fallbackNote).toBe("ポイントのあるアイテムがないため、件数で数えています");
  });
});

describe("バーンダウン", () => {
  it("burnStatus は完了なら「完了」、実行中ならピル", () => {
    expect(burnStatus(sprint({ state: "closed" }), "2026-11-14")).toBe("完了");
    expect(burnStatus(sprint(), "2026-11-14")).toBe("残り 2 日");
  });
  const burn: Burndown = {
    days: [
      { date: "2026-11-02", total: 10, remaining: 10, ideal: 10 },
      { date: "2026-11-03", total: 12, remaining: 8.5, ideal: 9 },
      { date: "2026-11-04", total: 12, remaining: null, ideal: 8 },
    ],
    shown: 2,
    committed: null,
  };
  it("burnTotals は表示する最後の日の値", () => {
    expect(burnTotals(burn, "points")).toEqual({ total: "総量 12 pt", remaining: "残り 8.5 pt" });
  });
  it("burnTotals は 0 日なら「—」", () => {
    expect(burnTotals({ ...burn, shown: 0 }, "count")).toEqual({
      total: "総量 —",
      remaining: "残り —",
    });
  });
  it("committedNote", () => {
    expect(committedNote(10, "points")).toBe(
      "確定の計画 10 ptと開始日の総量が違うのは、完了のあとにアイテムを削除・移動したり、ステータスの分類を変えたりして、いまの状態から数え直しているためです",
    );
  });
  it("notStarted", () => {
    expect(notStarted("2026-11-20", 2026)).toBe("開始日(11/20)から表示します");
  });
  it("addedText", () => {
    expect(addedText(3, "count")).toBe("途中で追加 +3 件");
  });
});

describe("ベロシティと所要時間", () => {
  it("averageText", () => {
    expect(averageText(3.3, "points")).toBe("完了した直近 3 スプリントの平均 3.3 pt");
    expect(averageText(4, "count")).toBe("完了した直近 3 スプリントの平均 4.0 件");
    expect(averageText(null, "points")).toBe("完了した直近 3 スプリントの平均 —");
  });
  it("durationLines のサイクルタイム", () => {
    const l = durationLines({ n: 5, median: 3, p85: 6.4, skipped: 2 }, "cycle");
    expect(l.label).toBe("サイクルタイム 中央値(着手→完了・5 件、着手を経ていない 2 件を除く)");
    expect(l.value).toBe("3.0 日");
    expect(l.p85).toBe("85% は 6.4 日以内");
  });
  it("skipped が 0 なら除く文は付けない", () => {
    expect(durationLines({ n: 5, median: 3, p85: 6, skipped: 0 }, "cycle").label).toBe(
      "サイクルタイム 中央値(着手→完了・5 件)",
    );
  });
  it("リードタイムは skipped を見ない", () => {
    expect(durationLines({ n: 2, median: 1.2, p85: 2, skipped: 9 }, "lead")).toEqual({
      label: "リードタイム 中央値(作成→完了・2 件)",
      value: "1.2 日",
      p85: "85% は 2.0 日以内",
    });
  });
  it("0 件は「—」で 85% は null", () => {
    expect(durationLines({ n: 0, median: null, p85: null, skipped: 0 }, "lead")).toEqual({
      label: "リードタイム 中央値(作成→完了・0 件)",
      value: "—",
      p85: null,
    });
  });
});

describe("pointsCard", () => {
  const active = (s: ReportSprint | null): ReportActive => ({
    sprint: s,
    unit: "points",
    burndown: { days: [], shown: 0, committed: null },
    added: 0,
    done: { count: 1, points: 3 },
    total: { count: 4, points: 10.5 },
  });
  it("実行中のスプリントがあるとき", () => {
    expect(pointsCard(active(sprint()))).toEqual({
      heading: "Sprint 4 の完了ポイント",
      value: "3 / 10.5 pt",
    });
  });
  it("実行中がないとき", () => {
    expect(pointsCard(active(null))).toEqual({ heading: "実行中のスプリントなし", value: "—" });
    expect(pointsCard(null)).toEqual({ heading: "実行中のスプリントなし", value: "—" });
  });
});
