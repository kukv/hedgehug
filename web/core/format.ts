// How dates and names are shown (Foundations: dates are Mono; this year
// without the year).

const DATE_ONLY = /^(\d{4})-(\d{2})-(\d{2})$/;

const pad = (n: number): string => String(n).padStart(2, "0");

// parse reads an ISO date (2026-11-07, a date property) as that day in the
// local time zone, and an ISO time as the moment it is.
function parse(iso: string): Date {
  const m = DATE_ONLY.exec(iso);
  return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : new Date(iso);
}

// formatDate: 11/07 this year, 2027/01/29 in another year.
export function formatDate(iso: string, now: Date): string {
  const d = parse(iso);
  const md = `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`;
  return d.getFullYear() === now.getFullYear() ? md : `${d.getFullYear()}/${md}`;
}

// formatDateTime: 11/04 10:12, with the year in another year.
export function formatDateTime(iso: string, now: Date): string {
  const d = parse(iso);
  return `${formatDate(iso, now)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// formatRelative: 今, 3 分前, 2 時間前, and the date from a day on.
export function formatRelative(iso: string, now: Date): string {
  const ms = now.getTime() - parse(iso).getTime();
  if (ms < 60_000) return "今";
  if (ms < 3_600_000) return `${Math.floor(ms / 60_000)} 分前`;
  if (ms < 86_400_000) return `${Math.floor(ms / 3_600_000)} 時間前`;
  return formatDate(iso, now);
}

// initials: the first two characters of a name, in capitals for letters.
export function initials(name: string): string {
  return Array.from(name.trim()).slice(0, 2).join("").toUpperCase();
}

// avatarTone picks one of the four colour pairs of the design (tokens
// --avatar-0 ... --avatar-3) from the name, the same each time.
export function avatarTone(name: string): 0 | 1 | 2 | 3 {
  let h = 0;
  for (const c of name) h = (h * 31 + (c.codePointAt(0) ?? 0)) >>> 0;
  return (h % 4) as 0 | 1 | 2 | 3;
}
