import type { Category } from "../../model/workspace";
import { formatDay } from "../dates/date";
import { relativeTime } from "../dates/relative";
import { dueLabel } from "../items/progress";
import { hoursOf } from "../notes/hours";
import type { SearchColumn, SearchRow } from "./types";

// テーブルのセルの表示の中身(E2a-12)。ID・種類・タイトル・ステータス・担当者・親は呼ぶ側(部品)が描くので、それ以外の列。

export type CellView =
  | { kind: "empty" }
  | { kind: "unused" }
  | { kind: "removed" }
  | { kind: "text"; text: string }
  | { kind: "due"; text: string; emphasis: "overdue" | "today" | null }
  | { kind: "check"; checked: boolean }
  | { kind: "url"; href: string }
  | { kind: "relative"; text: string }
  | { kind: "person"; name: string };

// updatedHint は、列「更新日」の見出しの title。
export const updatedHint = "GitHub と同期による変更は含みません";

const text = (t: string): CellView => (t === "" ? { kind: "empty" } : { kind: "text", text: t });

function day(ymd: string, now: Date): CellView {
  return ymd === "" ? { kind: "empty" } : { kind: "text", text: formatDay(ymd, now.getFullYear()) };
}

// custom は、追加プロパティのセル。列の型ではなく、セル自身の型で描く(行のワークスペースごとに型が違いうる)。
function custom(value: { text: string; type: string }, now: Date): CellView {
  if (value.text === "") return { kind: "empty" };
  switch (value.type) {
    case "date":
      return day(value.text, now);
    case "user":
      return { kind: "person", name: value.text };
    case "check":
      return { kind: "check", checked: value.text === "true" };
    case "url":
      // リンクにするのは http(s) だけ(Markdown と同じ)。javascript: のような値は、ただの文字にする。
      return /^https?:\/\//i.test(value.text)
        ? { kind: "url", href: value.text }
        : { kind: "text", text: value.text };
    default:
      return { kind: "text", text: value.text };
  }
}

export function cellView(row: SearchRow, column: SearchColumn, now: Date): CellView {
  const key = column.key;
  if (row.unused.includes(key)) return { kind: "unused" };
  if (row.removed.includes(key)) return { kind: "removed" };
  switch (key) {
    case "priority":
      return text(row.priority?.name ?? "");
    case "start":
      return day(row.start, now);
    case "end": {
      const due = dueLabel(row.end, row.status?.category as Category | undefined, now);
      return due ? { kind: "due", ...due } : { kind: "empty" };
    }
    case "points":
      return row.points === null ? { kind: "empty" } : text(String(row.points));
    case "estimate":
      return row.estimate === null ? { kind: "empty" } : text(`${hoursOf(row.estimate)} 時間`);
    case "system":
      return text(row.systems.join(", "));
    case "sprint":
      return text(row.sprint);
    case "release":
      return text(row.release);
    case "project":
      return text(row.project.title);
    case "created":
      return { kind: "relative", text: relativeTime(new Date(row.created), now) };
    case "updated":
      return { kind: "relative", text: relativeTime(new Date(row.updated), now) };
    default: {
      const value = row.custom[key];
      return value ? custom(value, now) : { kind: "empty" };
    }
  }
}
