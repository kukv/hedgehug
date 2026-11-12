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

export interface Toast {
  id: string;
  text: string;
  error?: boolean;
  action?: string;
}

// KindName は、種類のアイコンに出す種類。Project はプロジェクトのアイコン。
export type KindName = "Epic" | "Story" | "Task" | "Bug" | "Step" | "Project";

// Category は、ステータスの分類(03)。点の色を決める。
export type Category = "todo" | "doing" | "done" | "canceled";
