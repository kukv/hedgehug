import type { Burndown, FlowDay, Report, ReportActive, ReportsPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  type ReportActive as ApiActive,
  type ReportBurndown as ApiBurndown,
  type ReportFlowDay as ApiFlowDay,
  type ReportView as ApiView,
  getReport,
  getReportActive,
  putReportSelection,
} from "./generated";

// レポートの口の実装。API の型を core の型に直す(設計書 6 節)。
// 空の一覧は null で届いてもよいので、空にする。API のスプリント(状態の列挙も)は core の型と同じ形なので、そのまま使う。

const toBurndown = (b: ApiBurndown): Burndown => ({ ...b, days: b.days ?? [] });

const toFlowDay = (d: ApiFlowDay): FlowDay => ({ ...d, counts: d.counts ?? [] });

const toReport = (v: ApiView): Report => ({
  ...v,
  sprints: v.sprints ?? [],
  burn: v.burn ?? null,
  burndown: toBurndown(v.burndown),
  velocity: v.velocity ?? [],
  layers: v.layers ?? [],
  flow: (v.flow ?? []).map(toFlowDay),
  estimates: v.estimates ?? [],
});

const toActive = (a: ApiActive): ReportActive => ({
  ...a,
  sprint: a.sprint ?? null,
  burndown: toBurndown(a.burndown),
});

export function createReportsPort(): ReportsPort {
  return {
    report: (s, w, period) =>
      call(async () => toReport(await getReport(s, w, period ?? undefined))),
    active: (s, w) => call(async () => toActive(await getReportActive(s, w))),
    select: (s, w, patch) => call(() => putReportSelection(s, w, patch)),
  };
}
