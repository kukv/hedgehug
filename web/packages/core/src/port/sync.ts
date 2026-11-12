import type { SyncAllResult, SyncEventKind, SyncStatus, SyncStoreResult } from "../model/sync";

// Watching は、読み続けている出来事の流れ。done は、流れが終わったか stop で止めたら解決し、つなげなかった・
// 途中で切れたら reject する。core は DOM の AbortSignal を持たないので、止める関数で表す。
export interface Watching {
  done: Promise<void>;
  stop(): void;
}

// 同期の口。実装は api パッケージにある。存在しない Store の同期は NotFound を投げる。
export interface SyncPort {
  status(): Promise<SyncStatus>;
  syncStore(storeId: string): Promise<SyncStoreResult>;
  syncAll(): Promise<SyncAllResult>;
  // hold は、このタブが自動の同期を待たせる条件にあるかを知らせる(決定 C2-18)。
  hold(tab: string, hold: boolean): Promise<void>;
  // network は、このタブがネットワークにつながっているかを知らせる(決定 C2-18b)。
  network(tab: string, online: boolean): Promise<void>;
  // watch は、画面への反映の出来事を受け始める。つながったら open を呼ぶ。
  watch(on: { open(): void; event(kind: SyncEventKind): void }): Watching;
}
