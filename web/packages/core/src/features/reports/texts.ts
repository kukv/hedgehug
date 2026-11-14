import type {
  Burndown,
  Durations,
  ReportActive,
  ReportSprint,
  ReportUnit,
} from "../../model/report";
import { formatDay } from "../dates/date";
import { pill, pt } from "../planning/sprints";

// レポートの画面の文(05「レポート」)。小数の値は API が丸めていて、画面は toFixed(1) で常に小数第 1 位まで書く。

const none = "—";

export const unitLabel = (u: ReportUnit): "pt" | "件" => (u === "points" ? "pt" : "件");

// amount は、値と単位。件数も pt と同じ書き方で数を書く。
export const amount = (v: number, u: ReportUnit): string => `${pt(v)} ${unitLabel(u)}`;

export const fallbackNote = "ポイントのあるアイテムがないため、件数で数えています";

// burnStatus は、バーンダウンの見出しの横。完了は「完了」、実行中はピル。
export function burnStatus(s: ReportSprint, today: string): string {
  return s.state === "closed" ? "完了" : (pill(s, today) ?? "");
}

// burnTotals は、表示する最後の日の総量と残り。表示する日がなければ「—」。
export function burnTotals(b: Burndown, u: ReportUnit): { total: string; remaining: string } {
  const last = b.days[b.shown - 1];
  return {
    total: `総量 ${last ? amount(last.total, u) : none}`,
    remaining: `残り ${last && last.remaining !== null ? amount(last.remaining, u) : none}`,
  };
}

export const committedNote = (c: number, u: ReportUnit): string =>
  `確定の計画 ${amount(c, u)}と開始日の総量が違うのは、完了のあとにアイテムを削除・移動したり、ステータスの分類を変えたりして、いまの状態から数え直しているためです`;

export const notStarted = (start: string, thisYear: number): string =>
  `開始日(${formatDay(start, thisYear)})から表示します`;

export const averageText = (avg: number | null, u: ReportUnit): string =>
  `完了した直近 3 スプリントの平均 ${avg === null ? none : `${avg.toFixed(1)} ${unitLabel(u)}`}`;

// durationLines は、サイクルタイム・リードタイムの 1 行。0 件(中央値なし)の値は「—」。
export function durationLines(
  d: Durations,
  kind: "cycle" | "lead",
): { label: string; value: string; p85: string | null } {
  const skipped =
    kind === "cycle" && d.skipped > 0 ? `、着手を経ていない ${d.skipped} 件を除く` : "";
  const label =
    kind === "cycle"
      ? `サイクルタイム 中央値(着手→完了・${d.n} 件${skipped})`
      : `リードタイム 中央値(作成→完了・${d.n} 件)`;
  return {
    label,
    value: d.median === null ? none : `${d.median.toFixed(1)} 日`,
    p85: d.p85 === null ? null : `85% は ${d.p85.toFixed(1)} 日以内`,
  };
}

export const addedText = (v: number, u: ReportUnit): string => `途中で追加 +${amount(v, u)}`;

// pointsCard は、概要の「完了ポイント」のカード。常にポイントで書く。
export function pointsCard(a: ReportActive | null): { heading: string; value: string } {
  if (!a?.sprint) return { heading: "実行中のスプリントなし", value: none };
  return {
    heading: `${a.sprint.name} の完了ポイント`,
    value: `${pt(a.done.points)} / ${pt(a.total.points)} pt`,
  };
}
