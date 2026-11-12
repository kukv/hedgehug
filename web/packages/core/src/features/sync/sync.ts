import type { SyncStatus } from "../../model/sync";
import type { SyncPort } from "../../port/sync";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { offlineSyncMessage } from "./card";

// 同期の状態と手動の同期(06「手動で同期する」「同期の結果」)。トーストの文言はサーバーが作る(決定 C2-21)。

export interface SyncDeps {
  port: SyncPort;
  toasts: Toasts;
  // online は、このタブがネットワークにつながっているか(navigator.onLine)。
  online(): boolean;
  // onSynced は、手動の同期が終わったあと(待たずに反映する。決定 S12)。
  onSynced(): void;
}

export interface SyncState {
  status: Readable<SyncStatus | null>;
  // load は、同期の状態を読み直す。失敗したら、いまの状態のまま(A2 の決定 29)。
  load(): Promise<void>;
  syncAll(): Promise<void>;
  syncStore(storeId: string): Promise<void>;
}

export function createSync(deps: SyncDeps): SyncState {
  const status = writable<SyncStatus | null>(null);
  let latest = 0;

  async function load() {
    latest += 1;
    const mine = latest;
    try {
      const next = await deps.port.status();
      if (mine === latest) status.set(next);
    } catch {
      // 通信の失敗は出さない(決定 S16)。
    }
  }

  // run は、オフラインなら送らずにトーストを出し、そうでなければ同期して、結果を出して読み直す。
  async function run(sync: () => Promise<void>) {
    if (!deps.online()) {
      deps.toasts.show(offlineSyncMessage, { error: true });
      return;
    }
    try {
      await sync();
    } catch {
      return; // 決定 S16
    }
    deps.onSynced();
    await load();
  }

  return {
    status: readonly(status),
    load,
    syncAll: () =>
      run(async () => {
        const r = await deps.port.syncAll();
        deps.toasts.show(r.lines.join("\n"), { error: r.result === "failed" });
      }),
    syncStore: (storeId) =>
      run(async () => {
        const r = await deps.port.syncStore(storeId);
        deps.toasts.show(r.message, { error: r.result === "failed" });
        if (r.mirrorWarning !== "") deps.toasts.show(r.mirrorWarning, { warn: true });
      }),
  };
}
