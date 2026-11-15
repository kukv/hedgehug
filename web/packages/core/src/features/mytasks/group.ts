import type { StoreNode } from "../catalog/catalog";
import { dayOf } from "../dates/date";
import type { SearchRow } from "../search/types";

// マイタスクのまとめ方(04「マイタスク」)。日付は YYYY-MM-DD の文字列のまま比べる。

export type MyTasksGroupBy = "due" | "project" | "workspace";

export const myTasksGroupChoices: { id: MyTasksGroupBy; label: string }[] = [
  { id: "due", label: "期限" },
  { id: "project", label: "プロジェクト" },
  { id: "workspace", label: "ワークスペース" },
];

export interface MyTaskGroup {
  key: string;
  label: string;
  warn: boolean;
  // store は、ワークスペースのまとめ方だけ。見出しに Store の名前と色を添える。
  store: { name: string; color: string } | null;
  rows: SearchRow[];
}

// addDays は、YYYY-MM-DD の n 日あと。正午で作るので、夏時間の切り替えでも日付がずれない。
function addDays(ymd: string, n: number): string {
  const [y = 0, m = 1, d = 1] = ymd.split("-").map(Number);
  return dayOf(new Date(y, m - 1, d + n, 12));
}

// weekdayOf は、月=1〜日=7。
function weekdayOf(ymd: string): number {
  const [y = 0, m = 1, d = 1] = ymd.split("-").map(Number);
  const w = new Date(y, m - 1, d, 12).getDay();
  return w === 0 ? 7 : w;
}

const dueBuckets = [
  { key: "overdue", label: "期限切れ", warn: true },
  { key: "today", label: "今日", warn: true },
  { key: "week", label: "今週", warn: false },
  { key: "later", label: "来週以降", warn: false },
  { key: "none", label: "期限なし", warn: false },
];

function dueKey(end: string, today: string, weekEnd: string): string {
  if (end === "") return "none";
  if (end < today) return "overdue";
  if (end === today) return "today";
  return end <= weekEnd ? "week" : "later";
}

// byEnd は、終了日の早い順(ない行は末尾)。同じ日は渡した順のまま(sort は安定)。
function byEnd(a: SearchRow, b: SearchRow): number {
  if (a.end === b.end) return 0;
  if (a.end === "") return 1;
  if (b.end === "") return -1;
  return a.end < b.end ? -1 : 1;
}

// Draft は、キーごとに行を配ったグループ(並べる前)。
interface Draft {
  key: string;
  label: string;
  storeIndex: number;
  store: MyTaskGroup["store"];
  rows: SearchRow[];
}

function distribute(rows: SearchRow[], by: "project" | "workspace", stores: StoreNode[]): Draft[] {
  const drafts = new Map<string, Draft>();
  for (const r of rows) {
    const key = `${r.storeId}/${by === "project" ? r.projectId : r.workspaceId}`;
    let d = drafts.get(key);
    if (!d) {
      const index = stores.findIndex((s) => s.id === r.storeId);
      const s = stores[index];
      const w = s?.workspaces.find((x) => x.id === r.workspaceId);
      d = {
        key,
        label: by === "project" ? r.project.title : (w?.name ?? r.workspaceId),
        storeIndex: index < 0 ? stores.length : index,
        store:
          by === "workspace"
            ? { name: s?.name ?? r.storeName, color: s?.color ?? r.storeColor }
            : null,
        rows: [],
      };
      drafts.set(key, d);
    }
    d.rows.push(r);
  }
  return [...drafts.values()];
}

// groupMyTasks は、行(サーバーの並び)をまとめ方ごとの見出しに分ける。today は YYYY-MM-DD、
// stores は catalog の並び(ワークスペース名と、同名のときの並びに使う)。
export function groupMyTasks(
  rows: SearchRow[],
  by: MyTasksGroupBy,
  today: string,
  stores: StoreNode[],
): MyTaskGroup[] {
  if (by === "due") {
    const weekEnd = addDays(today, 7 - weekdayOf(today));
    return dueBuckets
      .map((b) => ({
        key: b.key,
        label: b.label,
        warn: b.warn,
        store: null,
        rows: rows.filter((r) => dueKey(r.end, today, weekEnd) === b.key).sort(byEnd),
      }))
      .filter((g) => g.rows.length > 0);
  }
  return distribute(rows, by, stores)
    .sort((a, b) => {
      if (a.label !== b.label) return a.label < b.label ? -1 : 1;
      return a.storeIndex - b.storeIndex;
    })
    .map((d) => ({
      key: d.key,
      label: d.label,
      warn: false,
      store: d.store,
      rows: d.rows.sort(byEnd),
    }));
}
