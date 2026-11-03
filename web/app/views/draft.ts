import { ApiError } from "../../core/api/client";
import { errorMessage } from "../../core/messages";
import {
  DEFAULT_KANBAN,
  DEFAULT_TABLE,
  type KanbanSettings,
  type TableSettings,
  type ViewDraft,
} from "../../core/views";
import type { Layout } from "../router";
import { splitTerms } from "../tree/filter";

/** The layouts in the order of the tabs. */
export const LAYOUTS: { layout: Layout; label: string }[] = [
  { layout: "tree", label: "ツリー" },
  { layout: "kanban", label: "カンバン" },
  { layout: "table", label: "テーブル" },
];

const KANBAN_COLUMNS: Record<KanbanSettings["columns"], string> = {
  status: "ステータス",
  category: "カテゴリ",
};
const SWIMLANES: Record<KanbanSettings["swimlane"], string> = {
  none: "なし",
  assignee: "担当者",
  project: "プロジェクト",
};
const GROUPS: Record<TableSettings["groupBy"], string> = {
  none: "なし",
  parent: "親",
  status: "ステータス",
  assignee: "担当者",
  project: "プロジェクト",
};

/** The settings of the layout in a line, for the dialog that saves a view (細部 51). */
export function summaryOf(d: ViewDraft): string {
  if (d.layout === "kanban") {
    return `カンバン · 列: ${KANBAN_COLUMNS[d.kanban.columns]} · スイムレーン: ${SWIMLANES[d.kanban.swimlane]}`;
  }
  if (d.layout === "table") {
    return `テーブル · 列 ${d.table.columns.length} · グループ: ${GROUPS[d.table.groupBy]}`;
  }
  return "ツリー";
}

/**
 * Whether two names are one view for the server: names that differ only in case or in Unicode
 * normalization are the same (internal/view SameName), since a file system may not tell them apart.
 */
export function sameViewName(a: string, b: string): boolean {
  return a.normalize("NFC").toLowerCase() === b.normalize("NFC").toLowerCase();
}

const TREE_ORDER = new ApiError({ status: 400, type: "view.tree_order" });

/**
 * Why a tree view cannot keep this query (細部 54): the tree follows rank, so the server refuses a
 * sort: or a group: in it (view.tree_order). Null when nothing is wrong.
 */
export function treeOrderProblem(d: Pick<ViewDraft, "layout" | "query">): string | null {
  if (d.layout !== "tree") return null;
  return splitTerms(d.query).some((t) => /^(sort|group):/.test(t))
    ? errorMessage(TREE_ORDER)
    : null;
}

/** A draft of a new view: the settings that are not given are the defaults of the layout. */
export function newDraft(i: {
  layout: Layout;
  query: string;
  kanban?: KanbanSettings;
  table?: TableSettings;
}): ViewDraft {
  return {
    layout: i.layout,
    query: i.query,
    kanban: i.kanban ?? DEFAULT_KANBAN,
    table: i.table ?? DEFAULT_TABLE,
  };
}
