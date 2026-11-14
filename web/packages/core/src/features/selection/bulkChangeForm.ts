import type {
  BulkChangeInput,
  BulkChangeResult,
  BulkChoices,
  BulkField,
} from "../../model/bulkChange";
import type { Category } from "../../model/workspace";
import { categoryLabels } from "../workspaces/values";

// 一括変更ダイアログの項目・選択肢・説明・確認の文(F2b2)。

// bulkFields は、項目のセレクトの順と表示名。
export const bulkFields: { id: BulkField; label: string }[] = [
  { id: "status", label: "ステータス" },
  { id: "assignee", label: "担当者" },
  { id: "priority", label: "優先度" },
  { id: "sprint", label: "スプリント" },
  { id: "release", label: "リリース" },
  { id: "systemsAdd", label: "関連システムを追加" },
  { id: "systemsRemove", label: "関連システムを削除" },
  { id: "points", label: "ポイント" },
  { id: "estimate", label: "見積もり時間" },
  { id: "start", label: "開始日" },
  { id: "end", label: "終了日" },
];

const labelOf = (field: BulkField) => bulkFields.find((f) => f.id === field)?.label ?? field;

// changeButton は、ダイアログの実行ボタンの文言。
export function changeButton(field: BulkField): string {
  return field === "systemsAdd" || field === "systemsRemove"
    ? labelOf(field)
    : `${labelOf(field)}を変更`;
}

export interface BulkOption {
  value: string;
  label: string;
  group?: string;
}

const categoryPrefix = "category:";
const categoryGroup = "分類で変更";
const categories: Category[] = ["todo", "doing", "done", "canceled"];

const names = (list: string[]): BulkOption[] => list.map((n) => ({ value: n, label: n }));

// valueOptions は、セレクトの選択肢。値を選んで送らない項目(ポイントなど)は空。
export function valueOptions(field: BulkField, c: BulkChoices): BulkOption[] {
  switch (field) {
    case "status":
      return [
        ...names(c.statuses),
        ...categories.map((k) => ({
          value: `${categoryPrefix}${k}`,
          label: categoryLabels[k],
          group: categoryGroup,
        })),
      ];
    case "priority":
      return names(c.priorities);
    case "sprint":
      return [{ value: "", label: "バックログ" }, ...names(c.sprints)];
    case "release":
      return [{ value: "", label: "なし" }, ...names(c.releases)];
    case "assignee":
      return [
        { value: "", label: "未割り当て" },
        ...c.members.map((m) => ({ value: m.login, label: m.name })),
      ];
    case "systemsAdd":
    case "systemsRemove":
      return names(c.systems);
    default:
      return [];
  }
}

// inputOf は、セレクトで選んだ選択肢の value を送る入力にする。`category:` で始まるものは分類で変える。
export function inputOf(field: BulkField, option: string): BulkChangeInput {
  if (option.startsWith(categoryPrefix)) {
    return {
      field,
      value: "",
      values: [],
      category: option.slice(categoryPrefix.length) as Category,
    };
  }
  return { field, value: option, values: [], category: "" };
}

// summaryText は、ダイアログの概要。
export function summaryText(n: number, kinds: string[]): string {
  return kinds.length === 0 ? `${n} 件` : `${n} 件(${kinds.join("・")})`;
}

const typed: BulkField[] = ["points", "estimate", "start", "end"];

// explanation は、ダイアログの説明(1 行ずつ)。
export function explanation(field: BulkField): string[] {
  const lines = [
    "値は名前で合わせます。ワークスペースに同じ名前の値がないアイテム、ワークフローの制約に合わないアイテム、プロジェクトで使わないステータスにはできないアイテムは変えずに残し、件数を知らせます。",
  ];
  if (typed.includes(field)) lines.push("空にすると値を削除します。");
  return lines;
}

// confirmLines は、確認ダイアログの本文。件数は予告の changed、値は予告の valueText。
export function confirmLines(input: BulkChangeInput, r: BulkChangeResult): string[] {
  const n = r.changed;
  if (input.field === "systemsAdd")
    return [`${n} 件に関連システム「${r.valueText}」を追加します。`];
  if (input.field === "systemsRemove") {
    return [`${n} 件から関連システム「${r.valueText}」を削除します。`];
  }
  if (input.category !== "") {
    const label = categoryLabels[input.category];
    return [
      `${n} 件のステータスを 分類「${label}」(各ワークスペースの既定のステータス)に変更します。`,
    ];
  }
  return [`${n} 件の${labelOf(input.field)}を ${r.valueText} に変更します。`];
}
