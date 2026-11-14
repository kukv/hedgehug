import type { BulkChangeResult } from "../../model/bulkChange";
import type { SelectedRow } from "./rowSelection";

// 一括変更の結果の文(F2b2)。トーストと、変えずに残したものの理由の一覧。

// 理由の一覧に並べる ID の数。これを超えたぶんは「ほか K 件」にする。
const listed = 3;

// reasonList は、変えずに残したものを理由ごとにまとめ、続けて保存できなかった Store を並べる。
// ID は選んだ行の表示から引く(行にないものは内部 ID のまま)。
export function reasonList(r: BulkChangeResult, rows: SelectedRow[]): string {
  const display = (store: string, item: string) =>
    rows.find((x) => x.storeId === store && x.itemId === item)?.display ?? item;
  const groups = new Map<string, string[]>();
  for (const k of r.kept) {
    const ids = groups.get(k.reason) ?? [];
    ids.push(display(k.store, k.item));
    groups.set(k.reason, ids);
  }
  const parts = [...groups].map(([reason, ids]) => {
    const shown = ids.slice(0, listed).join("・");
    const more = ids.length - listed;
    return more > 0 ? `${shown} ほか ${more} 件(${reason})` : `${shown}(${reason})`;
  });
  for (const f of r.failed) parts.push(`${f.name} の ${f.count} 件(保存できませんでした)`);
  return parts.join("、");
}

// offBacklogText は、バックログに表示されなくなったものの添え書き(なければ空)。
export function offBacklogText(r: BulkChangeResult): string {
  const { done, canceled } = r.offBacklog;
  const n = done + canceled;
  if (n === 0) return "";
  const kinds = [done > 0 ? "完了" : "", canceled > 0 ? "中止" : ""].filter(Boolean).join("・");
  return `。${n} 件は${kinds}のため、バックログには表示されません`;
}

// bulkChangeToast は、実行したあとのトースト。すべて変わったとき以外はエラーの色にする。
// suffix は、変わったものがあるときだけ、末尾に付ける。
export function bulkChangeToast(
  r: BulkChangeResult,
  rows: SelectedRow[],
  suffix: string,
): { text: string; error: boolean } {
  const kept = r.kept.length + r.failed.reduce((n, f) => n + f.count, 0);
  const tail = offBacklogText(r) + suffix;
  if (r.changed === 0) {
    const list = reasonList(r, rows);
    return {
      text: list === "" ? "変わるものはありません" : `変更できたものはありません: ${list}`,
      error: true,
    };
  }
  if (r.kept.length === 0 && r.failed.length === 0) {
    return { text: `${r.changed} 件を変えました${tail}`, error: false };
  }
  const head = `${r.changed} 件を変えました。${kept} 件はそのままです: ${reasonList(r, rows)}`;
  return { text: head + tail, error: true };
}
