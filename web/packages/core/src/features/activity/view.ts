import type { ActivityKind, ActivityRow } from "../../model/activity";
import type { StoreNode } from "../catalog/catalog";
import { dayOf, formatFullDay } from "../dates/date";

// アクティビティの表示の値(06「アクティビティ」)。日付と時刻はこのマシンの時刻帯で見る。

// SelectOption は、ui の Select の選択肢と同じ形(core は ui を import できないので同じ形を持つ)。
export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export const activityEmpty = "この条件のコミットはありません";

const kindLabels: Record<ActivityKind, string> = {
  all: "すべて",
  user: "人の変更だけ",
  sync: "同期だけ",
  automation: "自動化だけ",
};

export const activityKindOptions: SelectOption[] = (Object.keys(kindLabels) as ActivityKind[]).map(
  (value) => ({ value, label: kindLabels[value] }),
);

// workspaceOptions は、サイドバーの Store の一覧から作る「すべて」と「{Store 名} / {ワークスペース名}」。
export function workspaceOptions(stores: StoreNode[]): SelectOption[] {
  return [
    { value: "", label: "すべて" },
    ...stores.flatMap((s) =>
      s.workspaces.map((w) => ({ value: `${s.id}/${w.id}`, label: `${s.name} / ${w.name}` })),
    ),
  ];
}

const pad = (n: number) => String(n).padStart(2, "0");

// clockOf は、時刻(RFC 3339)の「HH:MM」。
export function clockOf(at: string): string {
  const d = new Date(at);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// yesterdayOf は、YYYY-MM-DD の前の日。
function yesterdayOf(ymd: string): string {
  const [y = 0, m = 1, d = 1] = ymd.split("-").map(Number);
  // 正午で作るので、夏時間の切り替えでも日付がずれない。
  return dayOf(new Date(y, m - 1, d - 1, 12));
}

// activityDays は、行(新しい順)を日付ごとにまとめる。見出しは「今日」「昨日」、ほかは「YYYY/MM/DD」。today は YYYY-MM-DD。
export function activityDays(
  rows: ActivityRow[],
  today: string,
): { head: string; rows: ActivityRow[] }[] {
  const yesterday = yesterdayOf(today);
  const days: { day: string; head: string; rows: ActivityRow[] }[] = [];
  for (const row of rows) {
    const day = dayOf(new Date(row.at));
    const last = days.at(-1);
    if (last?.day === day) {
      last.rows.push(row);
      continue;
    }
    const head = day === today ? "今日" : day === yesterday ? "昨日" : formatFullDay(day);
    days.push({ day, head, rows: [row] });
  }
  return days.map(({ head, rows: r }) => ({ head, rows: r }));
}
