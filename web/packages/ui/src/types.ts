export interface SelectOption {
  value: string;
  label: string;
  // disabled は、表示はするが選べない選択肢(未選択を表す「—」など)。
  disabled?: boolean;
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

// Markdown の表示(Markdown.svelte)に渡す形。core の markdown.ts の Block・Inline と同じ形で、ui は core を
// import できないのでここに持つ。app が core の字句分けの結果をそのまま渡す。
export type MarkdownInline =
  | { kind: "text"; text: string }
  | { kind: "bold"; text: string }
  | { kind: "code"; text: string }
  | { kind: "link"; url: string }
  | { kind: "mention"; login: string };

export interface MarkdownListItem {
  check: "none" | "unchecked" | "checked";
  inlines: MarkdownInline[];
}

export type MarkdownBlock =
  | { kind: "heading"; level: 1 | 2 | 3; inlines: MarkdownInline[] }
  | { kind: "list"; ordered: boolean; items: MarkdownListItem[] }
  | { kind: "code"; text: string }
  | { kind: "paragraph"; inlines: MarkdownInline[] };

// コマンドパレット(CommandPalette.svelte)に渡す形。core の PaletteState・Row と同じ形で、ui は core を import
// できないのでここに持つ。app が core の状態をそのまま渡す。押せるのは candidate だけで、more は押せない 1 行
// (「ほか N 件…」「一致するものはありません」)。
export type PaletteRow =
  | { kind: "head"; text: string }
  | { kind: "more"; text: string }
  | {
      kind: "candidate";
      c: { key: string; label: string; hint?: string; disabled?: boolean; checked?: boolean };
    };

export interface PaletteView {
  input: string;
  rows: PaletteRow[];
  // selected は、選んでいる行の位置。押せる行がないときは -1。
  selected: number;
  footer: string;
  // selectedHead は、Store の点を頭に付ける見出し(storeColor はトークン --t-<色>-fg の色の名前)。
  selectedHead: { text: string; storeColor?: string } | null;
}
