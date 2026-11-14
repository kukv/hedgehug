import type { BurnDay, ReportUnit, VelocityBar } from "../../model/report";
import { formatDay } from "../dates/date";
import { pt } from "../planning/sprints";
import { unitLabel } from "./texts";

// レポートの「表で見る」の列と行(05「レポート」)。見出しに単位を書くので、セルの値は数だけ(pt の書き方)。

export type Table = { columns: string[]; rows: string[][] };

// heading は、表の列の見出し。「総量 (pt)」のように単位を添える。
export const heading = (name: string, u: ReportUnit): string => `${name} (${unitLabel(u)})`;

// burndownTable は、バーンダウンの表。レポートとスプリントのカードで同じ表を出す。
// 表示する日より後の「残り」は「—」(F3-9)。「理想」は API が丸めた値を常に小数第 1 位まで書く(F3-22)。
export function burndownTable(days: BurnDay[], unit: ReportUnit, thisYear: number): Table {
  return {
    columns: ["日付", heading("総量", unit), heading("残り", unit), heading("理想", unit)],
    rows: days.map((d) => [
      formatDay(d.date, thisYear),
      pt(d.total),
      d.remaining === null ? "—" : pt(d.remaining),
      d.ideal.toFixed(1),
    ]),
  };
}

// velocityTable は、ベロシティの表。
export function velocityTable(bars: VelocityBar[], unit: ReportUnit): Table {
  return {
    columns: ["スプリント", heading("計画", unit), heading("完了", unit)],
    rows: bars.map((v) => [v.sprint.name, pt(v.planned), pt(v.done)]),
  };
}
