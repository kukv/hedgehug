import type { Route } from "../router/route";

// コマンドパレットの群・見出し・上限(02「コマンドパレット」、A5c-7)。

export type GroupId =
  | "command"
  | "query"
  | "selected"
  | "create"
  | "settings"
  | "recent"
  | "item"
  | "project"
  | "workspace"
  | "view";

export interface Candidate {
  key: string;
  group: GroupId;
  label: string;
  hint?: string;
  disabled?: boolean;
  reason?: string;
  // route は、Ctrl+Enter で新しいタブに開く行き先(A5c-12)。ないものは Enter と同じ。
  route?: Route;
  run: () => void | Promise<void>;
}

// Row は、パレットの 1 行。押せるのは candidate だけ。more は押せない 1 行(「ほか N 件…」「一致するものはありません」)。
export type Row =
  | { kind: "head"; text: string }
  | { kind: "more"; text: string }
  | { kind: "candidate"; c: Candidate };

// order は群の順、heads は見出し(selected は呼ぶ側が「{ID} に対して」を渡す)。
const order: GroupId[] = [
  "command",
  "query",
  "selected",
  "create",
  "settings",
  "recent",
  "item",
  "project",
  "workspace",
  "view",
];
const heads: Record<GroupId, string> = {
  command: "コマンド",
  query: "このクエリで",
  selected: "選んでいるアイテム",
  create: "作成",
  settings: "設定",
  recent: "最近開いたもの",
  item: "アイテム",
  project: "プロジェクト",
  workspace: "ワークスペース",
  view: "ビュー",
};
// limits は群の上限。アイテムは、全体の 40 から shared の群が出した件数を引いた残り(最低 17)。
const limits: Record<GroupId, number> = {
  command: 8,
  query: 3,
  selected: 4,
  create: 3,
  settings: 6,
  recent: 8,
  item: 40,
  project: 5,
  workspace: 5,
  view: 5,
};
const shared: GroupId[] = ["command", "project", "workspace", "view"];
// typedOnly は入力があるときだけ、emptyOnly は入力が空のときだけ出す群。
const typedOnly: GroupId[] = [
  "query",
  "create",
  "settings",
  "item",
  "project",
  "workspace",
  "view",
];
const emptyOnly: GroupId[] = ["recent"];

export const noMatch = "一致するものはありません";
export const more = (n: number) => `ほか ${n} 件あります。入力を続けて絞り込んでください`;

// buildRows は、群ごとの候補(一致と並びは済んだもの)を、群の順・見出し・上限・「ほか N 件」の行にする。
// 入力の空・非空で出さない群を除く。出す候補が 1 つもないときは「一致するものはありません」の 1 行。
// head は見出しの差し替え(選んでいるアイテムの「{ID} に対して」など)。
export function buildRows(
  groups: Partial<Record<GroupId, Candidate[]>>,
  input: string,
  head: Partial<Record<GroupId, string>> = {},
): Row[] {
  const empty = input.trim() === "";
  const shown = (g: GroupId) =>
    (empty ? !typedOnly.includes(g) : !emptyOnly.includes(g)) ? (groups[g] ?? []) : [];
  const used = shared.reduce((n, g) => n + Math.min(shown(g).length, limits[g]), 0);
  const limitOf = (g: GroupId) => (g === "item" ? Math.max(17, limits.item - used) : limits[g]);
  const rows: Row[] = [];
  for (const g of order) {
    const list = shown(g);
    if (list.length === 0) continue;
    const cap = limitOf(g);
    rows.push({ kind: "head", text: head[g] ?? heads[g] });
    for (const c of list.slice(0, cap)) rows.push({ kind: "candidate", c });
    if (list.length > cap) rows.push({ kind: "more", text: more(list.length - cap) });
  }
  return rows.length === 0 ? [{ kind: "more", text: noMatch }] : rows;
}
