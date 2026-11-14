import { createReportsPort } from "@hedgehug/api";
import { createActiveReports, createReports } from "@hedgehug/core";
import { reloads } from "./reloads";

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
