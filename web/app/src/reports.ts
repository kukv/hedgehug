import { createReportsPort } from "@hedgehug/api";
import { createActiveReports, createReports, dayOf } from "@hedgehug/core";
import { reloads } from "./reloads";
import { syncNow } from "./wire";

// レポート(05「レポート」)。wire.ts が 300 行に近いので、planning.ts と同じくここで作る。
// Store の出来事で読み直す(wire.ts の reflect が reloads を呼ぶ。F3-19)。
const port = createReportsPort();
// reports はレポートのタブ、activeReports はスプリントの画面のカードと概要が使う。
export const reports = createReports(port);
export const activeReports = createActiveReports(port);
reloads.add(() => reports.reload());
reloads.add(() => activeReports.reload());

// スプリントの画面からレポートを開く前に、バーンダウンのスプリントを保存する(F3-16)。
// レポートのタブはまだ開いていないので、reports.select は使えず、口を直接呼ぶ。
export const selectReport = port.select;

// 日付が変わったら activeReports を読み直す(01「日付と時刻」、F3-19)。購読をやめる関数を返す。
// スプリントの画面と概要のタブが、開いているあいだだけ使う。
export function reloadActiveOnDayChange(): () => void {
  let seen: string | null = null;
  return syncNow.subscribe((now) => {
    const day = dayOf(now);
    if (seen !== null && day !== seen) void activeReports.reload();
    seen = day;
  });
}
