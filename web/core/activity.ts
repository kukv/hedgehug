// The activity feed as the screen shows it: the words for an op, who is shown as the author, the
// days the commits are grouped by, and the line of trailers under a commit (the server sends no
// text; the words are the screen's).
import type { Schemas } from "./api/client";
import { daysAgo, formatDate, pad } from "./format";

export type Log = Schemas["LogView"];

/** The words for every Hedgehug-Op that internal/app/usecase, internal/app/storage/store and internal/app/remote/replicate write. */
const OP_LABELS: Readonly<Record<string, string>> = {
  add: "作成",
  set: "変更",
  edit: "本文の編集",
  status: "ステータスの変更",
  dep: "依存の変更",
  link: "リンク",
  unlink: "リンクの解除",
  "gh-fetch": "GitHub の取り込み",
  watch: "ウォッチ",
  unwatch: "ウォッチの解除",
  mv: "移動",
  "move-in": "別の Store から移動",
  "move-out": "別の Store へ移動",
  rm: "削除",
  restore: "復元",
  comment: "コメント",
  merge: "同期のマージ",
  repair: "同期の修復",
  replay: "同期のやり直し",
  setup: "同期の設定",
  init: "Store の作成",
  schema: "スキーマの変更",
  migrate: "スキーマの移行",
  view: "ビューの変更",
  workspace: "ワークスペースの変更",
  purge: "完全な削除",
  "revert-overwritten": "上書きを戻す",
  undo: "取り消し",
};

/** The words for an op; an op that is not known is shown as it is, a commit with none as 変更. */
export function opLabel(op: string | null): string {
  if (op === null) return "変更";
  return OP_LABELS[op] ?? op;
}

/** Who made a commit: the author, and for a merge or a repair the sync of that author's machine. */
export function authorLabel(l: Log): string {
  return l.op === "merge" || l.op === "repair" ? `同期（${l.author.name}）` : l.author.name;
}

const WEEKDAYS = ["日", "月", "火", "水", "木", "金", "土"];

/** The head of a day: 今日 · 11/04（水）, 昨日 · 11/03（火）, 11/02（月）, 2025/11/02（日）. */
function dayName(d: Date, now: Date): string {
  const date = `${formatDate(d.toISOString(), now)}（${WEEKDAYS[d.getDay()]}）`;
  const diff = daysAgo(d, now);
  if (diff === 0) return `今日 · ${date}`;
  if (diff === 1) return `昨日 · ${date}`;
  return date;
}

export type Day = { key: string; name: string; logs: Log[] };

/** The commits in a group for each day of the local time, the days in the order they first come. A date that is none goes in the day 日時不明. */
export function groupByDay(logs: Log[], now: Date): Day[] {
  const days = new Map<string, Day>();
  for (const l of logs) {
    const d = new Date(l.date);
    const valid = !Number.isNaN(d.getTime());
    const key = valid
      ? `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
      : "unknown";
    let day = days.get(key);
    if (!day) {
      day = { key, name: valid ? dayName(d, now) : "日時不明", logs: [] };
      days.set(key, day);
    }
    day.logs.push(l);
  }
  return [...days.values()];
}

/** `0123456 · Hedgehug-Op: merge · …`: the short sha and the trailers as they are written. */
export function trailerLine(l: Log): string {
  return [l.hash.slice(0, 7), ...l.trailers.map((t) => `${t.key}: ${t.value}`)].join(" · ");
}
