import { amount, type ReportUnit, unitLabel } from "@hedgehug/core";

// 表のセルと棒の値の数。見出しに単位を書くので、値には単位を付けない(amount の数の部分)。
export const num = (v: number, u: ReportUnit): string =>
  amount(v, u).slice(0, -unitLabel(u).length - 1);

// 表の列の見出し。「総量 (pt)」のように単位を添える。
export const heading = (name: string, u: ReportUnit): string => `${name} (${unitLabel(u)})`;
