import type { Schemas } from "../../core/api/client";
import type { SyncStatus } from "../../core/connection";

export type SyncReport = Schemas["SyncReport"];
export type SyncKind = "synced" | "pending" | "error" | "offline";
export type SyncSummary = {
  kind: SyncKind;
  noRemote: boolean;
  unpushed: number;
  lastSuccess: string | null;
  nextFetch: string | null;
  pendingSince: string | null;
  pushAt: string | null;
  errors: { store: string; message: string }[];
};

function newest(a: string | null, b: string | undefined): string | null {
  if (!b) return a;
  return a === null || Date.parse(b) > Date.parse(a) ? b : a;
}

function oldest(a: string | null, b: string | undefined): string | null {
  if (!b) return a;
  return a === null || Date.parse(b) < Date.parse(a) ? b : a;
}

/**
 * Folds the sync status of every store into the one state the card shows.
 * Only stores with a remote count. Offline wins over an error, an error over pending.
 */
export function summarizeSync(statuses: Iterable<SyncStatus>, online: boolean): SyncSummary {
  const s: SyncSummary = {
    kind: "synced",
    noRemote: true,
    unpushed: 0,
    lastSuccess: null,
    nextFetch: null,
    pendingSince: null,
    pushAt: null,
    errors: [],
  };
  for (const st of statuses) {
    if ((st.remotes ?? []).length === 0) continue;
    s.noRemote = false;
    s.unpushed += st.unpushed ?? 0;
    s.lastSuccess = newest(s.lastSuccess, st.last_success);
    s.nextFetch = oldest(s.nextFetch, st.next_fetch);
    s.pendingSince = oldest(s.pendingSince, st.pending_since);
    s.pushAt = oldest(s.pushAt, st.push_at);
    const message = st.last_error || st.error;
    if (message) s.errors.push({ store: st.store, message });
  }
  if (s.noRemote) return s;
  if (!online) s.kind = "offline";
  else if (s.errors.length > 0) s.kind = "error";
  else if (s.unpushed > 0 || s.pendingSince) s.kind = "pending";
  return s;
}

/** The toast that tells how "sync now" went. */
export function syncMessage(reports: SyncReport[] | null): string {
  const rs = reports ?? [];
  const failed = rs.filter((r) => r.error);
  const first = failed[0];
  if (first) {
    const more = failed.length > 1 ? ` ほか ${failed.length - 1} 件` : "";
    return `同期に失敗しました: ${first.store}: ${first.error}${more}`;
  }
  if (rs.length === 0 || rs.every((r) => r.no_remote)) return "同期する remote がありません";
  const merges = rs.reduce((n, r) => n + r.merges, 0);
  return merges > 0 ? `同期しました · マージ ${merges} 件` : "同期しました";
}
