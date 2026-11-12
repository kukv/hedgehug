import { formatDay } from "./date";

// 相対時間(01「日付と時刻」の表)。1 分未満は「たった今」、7 日以上は日付。未来の時刻は「たった今」にする。

const minute = 60_000;
const hour = 60 * minute;
const day = 24 * hour;

const pad = (n: number) => String(n).padStart(2, "0");

export function relativeTime(at: Date, now: Date): string {
  const elapsed = now.getTime() - at.getTime();
  if (elapsed < minute) return "たった今";
  if (elapsed < hour) return `${Math.floor(elapsed / minute)} 分前`;
  if (elapsed < day) return `${Math.floor(elapsed / hour)} 時間前`;
  if (elapsed < 7 * day) return `${Math.floor(elapsed / day)} 日前`;
  const ymd = `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`;
  return formatDay(ymd, now.getFullYear());
}

// olderThanADay は、at から 24 時間以上たったか(06「24 時間以上同期されていません」)。
export function olderThanADay(at: Date, now: Date): boolean {
  return now.getTime() - at.getTime() >= day;
}
