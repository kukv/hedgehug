// 検索のテーブルの設定(E2a-13〜16)。画面の状態 search.table の中身と、選べる行。

import { isSelected, type SelectedRow } from "../selection/rowSelection";
import type { SearchColumn, SearchRow, SearchTableRequest } from "./types";

export interface TableSettings {
  columns: string[];
  group: string;
  sort: { key: string; desc: boolean } | null;
}

export const defaultTable: TableSettings = {
  columns: ["id", "title", "status", "assignee", "priority", "end", "points"],
  group: "none",
  sort: null,
};

export const groupChoices: { id: string; label: string }[] = [
  { id: "none", label: "なし" },
  { id: "status", label: "ステータス" },
  { id: "assignee", label: "担当者" },
  { id: "project", label: "プロジェクト" },
  { id: "parent", label: "親" },
  { id: "kind", label: "種類" },
  { id: "priority", label: "優先度" },
  { id: "sprint", label: "スプリント" },
  { id: "system", label: "関連システム" },
  { id: "created", label: "作成日" },
  { id: "updated", label: "更新日" },
];

const titleKey = "title";

const isRecord = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;

// readTable は、保存された値から設定を作る。形の合わない項目だけ初めの値にする。
export function readTable(raw: unknown): TableSettings {
  const r = isRecord(raw) ? raw : {};
  const columns =
    Array.isArray(r.columns) && r.columns.every((c) => typeof c === "string")
      ? [...(r.columns as string[])]
      : [...defaultTable.columns];
  const group =
    typeof r.group === "string" && groupChoices.some((g) => g.id === r.group)
      ? r.group
      : defaultTable.group;
  const s = r.sort;
  const sort =
    isRecord(s) && typeof s.key === "string" && typeof s.desc === "boolean"
      ? { key: s.key, desc: s.desc }
      : null;
  return { columns, group, sort };
}

// shownColumns は、t.columns の順で、対象にある列だけ。
export function shownColumns(t: TableSettings, columns: SearchColumn[]): SearchColumn[] {
  return t.columns.flatMap((k) => columns.filter((c) => c.key === k));
}

// columnChoices は、「表示する列」の選択肢。対象の列の並びで、タイトルは選べない。
export function columnChoices(
  t: TableSettings,
  columns: SearchColumn[],
): { id: string; label: string; checked: boolean; disabled?: boolean }[] {
  return columns.map((c) => ({
    id: c.key,
    label: c.name,
    checked: t.columns.includes(c.key),
    ...(c.key === titleKey ? { disabled: true } : {}),
  }));
}

// toggleColumn は、列を出す(末尾)・隠す。並べ替えに使っている列を隠すと、列見出しの並べ替えも解除する。
export function toggleColumn(t: TableSettings, key: string): TableSettings {
  if (key === titleKey) return t;
  if (t.columns.includes(key)) {
    return {
      ...t,
      columns: t.columns.filter((k) => k !== key),
      sort: t.sort?.key === key ? null : t.sort,
    };
  }
  return { ...t, columns: [...t.columns, key] };
}

// nextSort は、列見出しを押したときの並べ替え(昇順 → 降順 → なし)。別の列なら昇順から。
export function nextSort(t: TableSettings, key: string): TableSettings {
  if (t.sort?.key !== key) return { ...t, sort: { key, desc: false } };
  return { ...t, sort: t.sort.desc ? null : { key, desc: true } };
}

// tableRequest は、検索の API に渡すテーブルの設定。追加プロパティの `p:` を外して小文字のキーにする。
export function tableRequest(t: TableSettings): SearchTableRequest {
  return {
    sort: t.sort ? { key: t.sort.key.replace(/^p:/, "").toLowerCase(), desc: t.sort.desc } : null,
    group: t.group,
  };
}

// selectable は、選べる行(削除済みでない行)。行の順に 1 回ずつ。
export function selectable(rows: SearchRow[]): SelectedRow[] {
  return rows
    .filter((r) => r.deleted === null)
    .map((r) => ({
      storeId: r.storeId,
      itemId: r.itemId,
      display: r.display,
      readOnly: r.readOnly,
    }));
}

// headerCheck は、選べる行が 1 件以上あり、すべて選ばれているか。
export function headerCheck(rows: SearchRow[], selected: SelectedRow[]): boolean {
  const s = selectable(rows);
  return s.length > 0 && s.every((r) => isSelected(selected, r.storeId, r.itemId));
}
