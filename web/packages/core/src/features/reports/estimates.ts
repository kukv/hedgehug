import { formatHours } from "../notes/hours";
import { pt } from "../planning/sprints";

// 見積もりと実績の表の文(05「見積もりと実績」)。時間は 100 分の 1 時間の整数で受け取る。

const none = "—";

export const emptyEstimates =
  "見積もり(ポイントか見積もり時間)と作業時間の両方があるアイテムがありません。アイテムの「作業時間」で記録できます。";

export const sprintScopeNote =
  "作業時間は、アイテムに付いた記録のすべてです(別のスプリントにいたあいだの記録も含みます)。";

export const hoursText = (h: number | null): string =>
  h === null ? none : `${formatHours(h)} 時間`;

// diffText は、作業時間から見積もり時間を引いた差。正だけ符号を付け、警告の色(over)にする。
export function diffText(d: number | null): { text: string; over: boolean } {
  if (d === null) return { text: none, over: false };
  if (d > 0) return { text: `+${formatHours(d)} 時間`, over: true };
  if (d < 0) return { text: `-${formatHours(-d)} 時間`, over: false };
  return { text: "0 時間", over: false };
}

// perPointText は、時間/pt。API が丸めた値を、常に小数第 1 位まで書く。
export const perPointText = (v: number | null): string => (v === null ? none : v.toFixed(1));

export const pointsText = (p: number | null): string => (p === null ? none : `${pt(p)} pt`);
