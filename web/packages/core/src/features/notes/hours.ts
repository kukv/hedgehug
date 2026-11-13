import type { Worklog } from "../../model/notes";

// 作業時間の時間の検査と表記(02「作業時間」)。サーバーの internal/domain/worklog と同じ規則で、
// 画面でも先に検査する(A4a-9)。時間は 100 分の 1 時間の整数で持ち、文字列のまま見る(浮動小数点の丸めに左右されない)。

const empty = "時間を入力してください";
const scale = "時間は小数第 2 位までで入力してください";
const decimal = /^(\d*)(?:\.(\d*))?$/;
// 整数部の桁数の上限。100 分の 1 時間にして 2^53 を超えないようにする(仕様に上限はない。届く値ではない)。
const maxWholeDigits = 9;

export type ParsedHours = { ok: true; hundredths: number } | { ok: false; message: string };

const fail = (message: string): ParsedHours => ({ ok: false, message });

// parseHours は、時間の入力を 100 分の 1 時間の整数にする。空・数でない・0 以下は「時間を入力してください」、
// 小数第 3 位以下に 0 でない数字があれば「時間は小数第 2 位までで入力してください」(末尾の 0 は数えない)。
export function parseHours(input: string): ParsedHours {
  const m = decimal.exec(input.trim());
  const intPart = m?.[1] ?? "";
  const fracPart = m?.[2] ?? "";
  if (!m || (intPart === "" && fracPart === "")) return fail(empty);
  const whole = intPart.replace(/^0+/, "");
  const frac = fracPart.replace(/0+$/, "");
  if (frac.length > 2) return fail(scale);
  if (whole.length > maxWholeDigits) return fail(empty);
  const hundredths = Number(whole === "" ? "0" : whole) * 100 + Number(`${frac}00`.slice(0, 2));
  return hundredths > 0 ? { ok: true, hundredths } : fail(empty);
}

// formatHours は、100 分の 1 時間の整数を、小数第 2 位まで・末尾の 0 を省いた表記にする(0.3・1.25・2)。「 時間」は付けない。
export function formatHours(hundredths: number): string {
  const whole = Math.floor(hundredths / 100);
  const frac = hundredths % 100;
  if (frac === 0) return String(whole);
  return `${whole}.${String(frac).padStart(2, "0").replace(/0+$/, "")}`;
}

// hoursOf は、見積もり時間のような時間の数を、同じ表記にする。
export const hoursOf = (hours: number): string => formatHours(Math.round(hours * 100));

// stepHours は、時間の欄の入力を 0.25 時間ずつ増減する(steps は 1 か -1)。0 以下になるときは空にする。
// 読めない入力は 0 として数える。
export function stepHours(text: string, steps: number): string {
  const parsed = parseHours(text);
  const next = Math.max(0, (parsed.ok ? parsed.hundredths : 0) + steps * 25);
  return next === 0 ? "" : formatHours(next);
}

function sum(rows: Worklog[]): number {
  let total = 0;
  for (const r of rows) {
    const p = parseHours(r.hours);
    if (p.ok) total += p.hundredths;
  }
  return total;
}

// worklogHeading は、見出しの「N 時間」に、見積もり時間とポイントがあれば続ける(02「作業時間」)。
export function worklogHeading(
  rows: Worklog[],
  item: { estimate: number | null; points: number | null },
): string {
  const parts = [`${formatHours(sum(rows))} 時間`];
  if (item.estimate !== null) parts.push(`見積もり ${hoursOf(item.estimate)} 時間`);
  if (item.points !== null) parts.push(`${item.points} pt`);
  return parts.join(" · ");
}
