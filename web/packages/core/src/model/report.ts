import type { Sprint, Totals } from "./planning";

// レポート(05)。項目は API の DTO と同じ。日付は YYYY-MM-DD。
// null は「値なし」(画面は「—」に書く)。

export type ReportUnit = "points" | "count";
export type ReportLayers = "category" | "status";

export interface ReportSelection {
  unit: ReportUnit;
  // バーンダウンのスプリントの内部 ID。空は未選択。
  sprint: string;
  layers: ReportLayers;
  // 見積もりと実績の対象のスプリントの内部 ID。空はすべて。
  estimate: string;
}

export type ReportSprint = Sprint;

export interface BurnDay {
  date: string;
  total: number;
  remaining: number | null;
  ideal: number;
}

export interface Burndown {
  days: BurnDay[];
  // 表示する日の数。
  shown: number;
  // 完了したスプリントで、総量が確定の計画と違うときだけ、その確定の計画。
  committed: number | null;
}

export interface VelocityBar {
  sprint: ReportSprint;
  planned: number;
  done: number;
}

export interface FlowLayer {
  key: string;
  name: string;
  color: string;
}

export interface FlowDay {
  date: string;
  // layers と同じ順の件数。
  counts: number[];
}

export interface Durations {
  n: number;
  median: number | null;
  p85: number | null;
  skipped: number;
}

export interface EstimateRow {
  id: string;
  display: string;
  title: string;
  points: number | null;
  estimate: number | null;
  worked: number;
  diff: number | null;
  perPoint: number | null;
}

export interface EstimateTotal {
  points: number | null;
  estimate: number | null;
  worked: number;
  diff: number | null;
}

export interface Report {
  selection: ReportSelection;
  // 実際に数えた単位。
  unit: ReportUnit;
  // ポイントを選んでいるが件数で数えたか。
  fallback: boolean;
  sprints: ReportSprint[];
  burn: ReportSprint | null;
  burndown: Burndown;
  velocity: VelocityBar[];
  average: number | null;
  period: { from: string; to: string };
  layers: FlowLayer[];
  flow: FlowDay[];
  cycle: Durations;
  lead: Durations;
  estimates: EstimateRow[];
  estimateTotal: EstimateTotal;
}

export interface ReportActive {
  sprint: ReportSprint | null;
  unit: ReportUnit;
  burndown: Burndown;
  // 途中で追加した量。
  added: number;
  done: Totals;
  total: Totals;
}
