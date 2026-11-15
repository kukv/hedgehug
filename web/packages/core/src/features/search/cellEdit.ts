import type { WorkspaceProperty } from "../../model/workspace";
import type { Context } from "../items/detailContext";
import {
  type ItemValues,
  type PropertyRow,
  propertyRow,
  // biome-ignore lint/suspicious/noShadowRestrictedNames: 仕様の名前(アイテムの値を読む)
  valueOf,
} from "../items/properties";
import { asOptions, chosen, releaseChoices, sprintChoices } from "../planning/choices";
import type { SearchColumn, SearchGroup, SearchRow } from "./types";

// テーブルのセルの編集(E2b)。列と行から、詳細パネルと同じプロパティの行か、スプリント・リリースの選択肢を決める。

export type CellEditor =
  | { kind: "property"; row: PropertyRow }
  | {
      kind: "plan";
      key: "sprint" | "release";
      options: { value: string; label: string }[];
      value: string;
    }
  | { kind: "display" };

const display: CellEditor = { kind: "display" };

// rowValues は、検索の行から、プロパティの入力欄の値を読む形(ItemValues)にする。
export function rowValues(row: SearchRow): ItemValues {
  return {
    status: row.statusId,
    assignee: row.assignee?.login ?? "",
    priority: row.values.priority,
    start: row.start,
    end: row.end,
    points: row.points,
    estimate: row.estimate,
    systems: row.values.systems,
    choices: row.values.choices,
    fields: row.values.fields,
  };
}

const builtinOf: Record<string, string> = {
  status: "status",
  assignee: "assignee",
  priority: "priority",
  start: "start",
  end: "end",
  points: "points",
  estimate: "estimate",
  system: "systems",
};

// propertyOf は、列に当たる、行のワークスペースのプロパティ。固定の列は組み込み、`p:` の列は小文字のキーで探す。
function propertyOf(ctx: Context, column: SearchColumn): WorkspaceProperty | undefined {
  const list = ctx.settings.properties;
  if (column.key.startsWith("p:")) {
    const key = column.key.slice(2);
    return list.find((p) => !p.builtin && p.key.toLowerCase() === key);
  }
  const key = builtinOf[column.key];
  return key === undefined ? undefined : list.find((p) => p.builtin && p.key === key);
}

function planEditor(row: SearchRow, key: "sprint" | "release", ctx: Context): CellEditor {
  const current = row.values[key];
  const choices =
    key === "sprint" ? sprintChoices(ctx.plan, current) : releaseChoices(ctx.plan, current);
  return { kind: "plan", key, options: asOptions(choices), value: chosen(choices, current) };
}

// cellEditor は、セルを編集する部品の中身。context が読めていない、削除済みの行、この行で使わないセル、
// 表示だけの列、URL のプロパティは display(編集しない)。非表示のプロパティも編集できる。
export function cellEditor(
  row: SearchRow,
  column: SearchColumn,
  ctx: Context | undefined,
): CellEditor {
  if (!ctx || row.deleted || row.unused.includes(column.key)) return display;
  if (column.key === "sprint" || column.key === "release") {
    return planEditor(row, column.key, ctx);
  }
  const p = propertyOf(ctx, column);
  if (!p || p.type === "url") return display;
  const value = valueOf(rowValues(row), p);
  return { kind: "property", row: propertyRow(ctx.settings, p, value, ctx.people, row.projectId) };
}

// groupKeys は、グループの見出しの文字。同じ見出しは、2 回目から `${label}#2` のように数を付ける。
export function groupKeys(groups: SearchGroup[]): string[] {
  const seen = new Map<string, number>();
  return groups.map((g) => {
    const n = (seen.get(g.label) ?? 0) + 1;
    seen.set(g.label, n);
    return n === 1 ? g.label : `${g.label}#${n}`;
  });
}
