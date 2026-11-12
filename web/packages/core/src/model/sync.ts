// 同期の状態(06「同期の状態の表示」)。一覧は remote がある Store だけで、非表示の Store も含む。

export interface SyncStoreStatus {
  storeId: string;
  name: string;
  hidden: boolean;
  readOnly: boolean;
  // error は、同期エラーの短い理由(06「同期のエラー」の表の文言)。エラーでなければ空。
  error: string;
  unsent: number;
  // lastSuccess は、最後に成功した同期の時刻。1 度も成功していなければ null。
  lastSuccess: Date | null;
}

export interface SyncStatus {
  // auto は、「裏で自動的に同期する」がオンか(オフラインの帯の文を選ぶ)。
  auto: boolean;
  stores: SyncStoreStatus[];
}

// 手動の同期の結果(決定 C2-21)。文言はサーバーが作る。
export interface SyncStoreResult {
  result: "synced" | "skipped" | "failed";
  message: string;
  // mirrorWarning は、ミラーに送れなかったときの警告。なければ空。
  mirrorWarning: string;
}

export interface SyncAllResult {
  result: "synced" | "failed" | "nothing";
  lines: string[];
}

// 画面への反映の出来事の種類。store は Store の変化、sync は同期の状態の変化。
export type SyncEventKind = "store" | "sync";
