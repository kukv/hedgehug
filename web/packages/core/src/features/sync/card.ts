import type { SyncStatus, SyncStoreStatus } from "../../model/sync";
import { olderThanADay, relativeTime } from "../dates/relative";

// 同期カードと「Store ごとの同期状態」の文(06「同期の状態の表示」)。非表示の Store は数えない。

export interface SyncRow {
  storeId: string;
  name: string;
  // summary は「同期エラー: {短い理由}」「未送信 X」「最新」。
  summary: string;
  // time は、最後に成功した同期の相対時間か「未同期」。
  time: string;
  // stale は、最後の成功から 24 時間以上(相対時間を警告の色にする)。
  stale: boolean;
}

export interface SyncCard {
  summary: string;
  error: boolean;
  last: string;
  stale: string | null;
  rows: SyncRow[];
}

const visible = (status: SyncStatus | null): SyncStoreStatus[] =>
  status?.stores.filter((s) => !s.hidden) ?? [];

function rowOf(s: SyncStoreStatus, now: Date): SyncRow {
  let summary = s.unsent > 0 ? `未送信 ${s.unsent}` : "最新";
  if (s.error !== "") summary = `同期エラー: ${s.error}`;
  return {
    storeId: s.storeId,
    name: s.name,
    summary,
    time: s.lastSuccess ? relativeTime(s.lastSuccess, now) : "未同期",
    stale: s.lastSuccess !== null && olderThanADay(s.lastSuccess, now),
  };
}

function summaryOf(stores: SyncStoreStatus[]): { summary: string; error: boolean } {
  const errors = stores.filter((s) => s.error !== "").length;
  if (errors > 0) return { summary: `${errors} 件の Store で同期エラー`, error: true };
  const unsent = stores.reduce((n, s) => n + s.unsent, 0);
  return { summary: unsent > 0 ? `未送信 ${unsent}` : "すべて最新", error: false };
}

// syncCard は、カードの文。remote がある表示中の Store がなければ null(カードを出さない)。
export function syncCard(status: SyncStatus | null, now: Date): SyncCard | null {
  const stores = visible(status);
  if (stores.length === 0) return null;
  const times = stores.flatMap((s) => (s.lastSuccess ? [s.lastSuccess.getTime()] : []));
  const newest = times.length > 0 ? relativeTime(new Date(Math.max(...times)), now) : "なし";
  const rows = stores.map((s) => rowOf(s, now));
  const stale = rows.filter((r) => r.stale).length;
  return {
    ...summaryOf(stores),
    last: `最終同期 ${newest} · Store ${stores.length} 件`,
    stale: stale > 0 ? `${stale} 件の Store が 24 時間以上同期されていません` : null,
    rows,
  };
}

// hasSyncError は、860px 以下のメニューのボタンに点を付けるか。閲覧のみも数え、非表示は数えない。
export function hasSyncError(status: SyncStatus | null): boolean {
  return visible(status).some((s) => s.error !== "");
}

// offlineBand は、オフラインの帯の文(06「オフライン」)。
export function offlineBand(auto: boolean): string {
  return auto
    ? "オフラインです。変更はこのマシンにコミットし、つながったら push します。"
    : "オフラインです。変更はこのマシンにコミットします。つながったら「すべて同期」で push してください。";
}

// offlineSyncMessage は、オフラインで手動の同期をしたときのエラーのトースト。
export const offlineSyncMessage = "オフラインのため同期できません。つながったら同期します";

// isSyncAllKey は、「すべて同期」のキー(Shift+Ctrl+S、Mac は Shift+Cmd+S)か。
export function isSyncAllKey(e: {
  key: string;
  shiftKey: boolean;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
}): boolean {
  return e.key.toLowerCase() === "s" && e.shiftKey && (e.ctrlKey || e.metaKey) && !e.altKey;
}

// syncAllHint は、「すべて同期」のボタンの案内。
export function syncAllHint(mac: boolean): string {
  return mac ? "Shift+Cmd+S" : "Shift+Ctrl+S";
}
