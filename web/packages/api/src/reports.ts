import type {
  Burndown,
  FlowDay,
  Report,
  ReportActive,
  ReportSprint,
  ReportsPort,
  VelocityBar,
} from "@hedgehug/core";
import { call } from "./errors";
import {
  type ReportActive as ApiActive,
  type ReportBurndown as ApiBurndown,
  type ReportFlowDay as ApiFlowDay,
  type ReportSprint as ApiSprint,
  type ReportVelocity as ApiVelocity,
  type ReportView as ApiView,
  getReport,
  getReportActive,
  putReportSelection,
} from "./generated";

// レポートの口の実装。API の型を core の型に直す(設計書 6 節)。
// 空の一覧は null で届いてもよいので、空にする。

// API の列挙(状態・単位・層)は core の型と同じ文字列。
const toSprint = (s: ApiSprint): ReportSprint => s;

const toBurndown = (b: ApiBurndown): Burndown => ({ ...b, days: b.days ?? [] });

const toVelocity = (v: ApiVelocity): VelocityBar => ({ ...v, sprint: toSprint(v.sprint) });

const toFlowDay = (d: ApiFlowDay): FlowDay => ({ ...d, counts: d.counts ?? [] });

const toReport = (v: ApiView): Report => ({
  ...v,
  sprints: (v.sprints ?? []).map(toSprint),
  burn: v.burn ?? null,
  burndown: toBurndown(v.burndown),
  velocity: (v.velocity ?? []).map(toVelocity),
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
