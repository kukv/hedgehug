import { amount, type BurnDay, formatDay, type ReportUnit, unitLabel } from "@hedgehug/core";

// 表のセルと棒の値の数。見出しに単位を書くので、値には単位を付けない(amount の数の部分)。
export const num = (v: number, u: ReportUnit): string =>
  amount(v, u).slice(0, -unitLabel(u).length - 1);

// 表の列の見出し。「総量 (pt)」のように単位を添える。
export const heading = (name: string, u: ReportUnit): string => `${name} (${unitLabel(u)})`;

// burndownTable は、バーンダウンの「表で見る」の列と行。レポートとスプリントのカードで同じ表を出す。
// 表示する日より後の「残り」は「—」(F3-9)。
export function burndownTable(
  days: BurnDay[],
  unit: ReportUnit,
  thisYear: number,
): { columns: string[]; rows: string[][] } {
  return {
    columns: ["日付", heading("総量", unit), heading("残り", unit), heading("理想", unit)],
    rows: days.map((d) => [
      formatDay(d.date, thisYear),
      num(d.total, unit),
      d.remaining === null ? "—" : num(d.remaining, unit),
      num(d.ideal, unit),
    ]),
  };
}
