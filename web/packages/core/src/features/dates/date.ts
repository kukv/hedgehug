// 日付の表示(01「日付と時刻」): 今年なら「M/D」、それ以外の年なら「YYYY/MM/DD」。空は「なし」(01「画面の移り方」)。

// formatDay は、YYYY-MM-DD を表示の形にする。thisYear は、このマシンの時刻帯での今年。
export function formatDay(ymd: string, thisYear: number): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(ymd);
  if (!m) return "なし";
  const [, y, mo, d] = m;
  if (Number(y) === thisYear) return `${Number(mo)}/${Number(d)}`;
  return `${y}/${mo}/${d}`;
}

const two = (n: number) => String(n).padStart(2, "0");

// formatDateTime は、日時を「日付 HH:MM」にする(日付は formatDay と同じ形)。どちらもこのマシンの時刻帯で見る。
export function formatDateTime(at: Date, now: Date): string {
  const ymd = `${at.getFullYear()}-${two(at.getMonth() + 1)}-${two(at.getDate())}`;
  return `${formatDay(ymd, now.getFullYear())} ${two(at.getHours())}:${two(at.getMinutes())}`;
}
