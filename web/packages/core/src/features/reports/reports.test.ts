import { describe, expect, it } from "vitest";
import type { Report, ReportActive, ReportSelection } from "../../model/report";
import type { ReportsPort } from "../../port/reports";
import { get } from "../../reactive/store";
import { createActiveReports, createReports } from "./reports";

const reportOf = (tag: string): Report =>
  ({ selection: { unit: "points", sprint: tag, layers: "category", estimate: "" } }) as Report;
const activeOf = (tag: string): ReportActive => ({ added: tag.length }) as ReportActive;

type Period = { from: string; to: string } | null;

function portOf() {
  const reads: { key: string; period: Period }[] = [];
  const saved: Partial<ReportSelection>[] = [];
  const fail = { report: false, select: false, active: false };
  const port: ReportsPort = {
    async report(s, w, period) {
      reads.push({ key: `${s}/${w}`, period });
      if (fail.report) throw new Error("read");
      return reportOf(`${s}/${w}#${reads.length}`);
    },
    async active(s, w) {
      if (fail.active) throw new Error("read");
      return activeOf(`${s}/${w}`);
    },
    async select(_s, _w, patch) {
      if (fail.select) throw new Error("save");
      saved.push(patch);
    },
  };
  return { port, reads, saved, fail };
}

describe("createReports", () => {
  it("open で読み、別のワークスペースを開くと前の state を消す", async () => {
    const { port, reads } = portOf();
    const r = createReports(port);
    await r.open("a", "w1");
    expect(get(r.state)?.workspaceId).toBe("w1");
    const p = r.open("a", "w2");
    expect(get(r.state)).toBeNull();
    await p;
    expect(get(r.state)?.workspaceId).toBe("w2");
    expect(reads.map((x) => x.key)).toEqual(["a/w1", "a/w2"]);
  });

  it("select は変えた項目だけを保存してから読み直す", async () => {
    const { port, reads, saved } = portOf();
    const r = createReports(port);
    await r.open("a", "w1");
    await r.select({ unit: "count" });
    expect(saved).toEqual([{ unit: "count" }]);
    expect(reads).toHaveLength(2);
  });

  it("select の保存に失敗したら投げ、読み直さない", async () => {
    const { port, reads, fail } = portOf();
    const r = createReports(port);
    await r.open("a", "w1");
    fail.select = true;
    await expect(r.select({ unit: "count" })).rejects.toThrow("save");
    expect(reads).toHaveLength(1);
  });

  it("setPeriod で期間を付けて読み直し、close して open し直すと期間なし", async () => {
    const { port, reads } = portOf();
    const r = createReports(port);
    await r.open("a", "w1");
    const p = { from: "2026-11-01", to: "2026-11-10" };
    await r.setPeriod(p);
    expect(reads[1]?.period).toEqual(p);
    await r.reload();
    expect(reads[2]?.period).toEqual(p);
    r.close();
    expect(get(r.state)).toBeNull();
    await r.open("a", "w1");
    expect(reads[3]?.period).toBeNull();
  });

  it("setPeriod(null) で初めの期間に戻す", async () => {
    const { port, reads } = portOf();
    const r = createReports(port);
    await r.open("a", "w1");
    await r.setPeriod({ from: "2026-11-01", to: "2026-11-10" });
    await r.setPeriod(null);
    expect(reads[2]?.period).toBeNull();
  });

  it("読めなかった期間は覚えない", async () => {
    const { port, reads, fail } = portOf();
    const r = createReports(port);
    await r.open("a", "w1");
    fail.report = true;
    await r.setPeriod({ from: "2026-11-01", to: "2026-11-10" });
    fail.report = false;
    await r.reload();
    expect(reads[reads.length - 1]?.period).toBeNull();
  });

  it("reload の失敗では前の state のまま", async () => {
    const { port, fail } = portOf();
    const r = createReports(port);
    await r.open("a", "w1");
    const before = get(r.state);
    fail.report = true;
    await r.reload();
    expect(get(r.state)).toBe(before);
  });

  it("開いていなければ reload は何もしない", async () => {
    const { port, reads } = portOf();
    const r = createReports(port);
    await r.reload();
    expect(reads).toHaveLength(0);
  });
});

describe("createActiveReports", () => {
  it("open・reload・close", async () => {
    const { port, fail } = portOf();
    const a = createActiveReports(port);
    await a.open("a", "w1");
    const first = get(a.state);
    expect(first?.workspaceId).toBe("w1");
    expect(first?.active.added).toBe("a/w1".length);
    fail.active = true;
    await a.reload();
    expect(get(a.state)).toBe(first);
    a.close();
    expect(get(a.state)).toBeNull();
  });
});
