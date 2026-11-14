import { formatDay } from "../dates/date";

// 累積フロー図の期間(05「累積フロー図」)。期間は保存しない。

type Period = { from: string; to: string };

const dayMs = 86_400_000;

// defaultPeriod は、初めの期間。今日の 20 日前から今日まで(直近 21 日)。today は YYYY-MM-DD。
export function defaultPeriod(today: string): Period {
  const [y = 0, m = 1, d = 1] = today.split("-").map(Number);
  const from = new Date(Date.UTC(y, m - 1, d) - 20 * dayMs).toISOString().slice(0, 10);
  return { from, to: today };
}

// checkPeriod は、期間の検査。合わなければ日付入力の下に出す理由、合えば null。
// 文と順は Go の planning.CheckPeriod と同じ(YYYY-MM-DD は文字列の比較で日付の順になる)。
export function checkPeriod(from: string, to: string, today: string): string | null {
  if (from === "") return "開始日を入力してください";
  if (to === "") return "終了日を入力してください";
  if (to < from) return "終了日は開始日以降にしてください";
  if (to > today) return "終了日は今日以前にしてください";
  return null;
}

// periodLabel は、見出しの横の文。null は初めの期間。thisYear は、このマシンの時刻帯での今年。
export function periodLabel(p: Period | null, thisYear: number): string {
  if (p === null) return "直近 21 日・件数";
  return `${formatDay(p.from, thisYear)} 〜 ${formatDay(p.to, thisYear)}・件数`;
}
