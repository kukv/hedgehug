export interface SelectOption {
  value: string;
  label: string;
}

export interface Tab {
  id: string;
  label: string;
  count?: number;
}

export type MenuItem =
  | { id: string; label: string; hint?: string; danger?: boolean; disabled?: boolean }
  | { separator: true };

// ValueOption は、値を選ぶメニュー(ValueMenu)の 1 行。checked はいまの値。
export interface ValueOption {
  id: string;
  label: string;
  hint?: string;
  checked: boolean;
  // disabled は選べない項目、reason はその理由(遷移の制約。A3b-5)。
  disabled?: boolean;
  reason?: string;
}

export interface Toast {
  id: string;
  text: string;
  error?: boolean;
  // warn は、警告の色で出す。
  warn?: boolean;
  action?: string;
}

// KindName は、種類のアイコンに出す種類。Project はプロジェクトのアイコン。
export type KindName = "Epic" | "Story" | "Task" | "Bug" | "Step" | "Project";

// Category は、ステータスの分類(03)。点の色を決める。
export type Category = "todo" | "doing" | "done" | "canceled";

// SyncCardView は、同期カードに出す文(core の syncCard が作る)。
export interface SyncCardView {
  summary: string;
  error: boolean;
  last: string;
  stale: string | null;
  rows: { storeId: string; name: string; summary: string; time: string; stale: boolean }[];
}
