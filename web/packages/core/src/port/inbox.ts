import type { InboxCounts, InboxPage, InboxTab } from "../model/inbox";

// InboxCursor は、続きの印。前の一覧の最後の行の at と id。
export interface InboxCursor {
  at: string;
  id: number;
}

// 受信箱の口。実装は api パッケージにある。
export interface InboxPort {
  // list は、タブの行を新しい順に 300 件まで読む。before があれば、その行より前を読む。
  list(tab: InboxTab, before?: InboxCursor): Promise<InboxPage>;
  unread(): Promise<InboxCounts>;
  setRead(id: number, read: boolean): Promise<void>;
  readAll(tab: InboxTab): Promise<void>;
  // apply は「制約を無視して…」、keep は「変更しない」。
  apply(id: number): Promise<void>;
  keep(id: number): Promise<void>;
}
