// GitHub as the screens show it: which state a chip tells, the words for a review, a CI run and
// where a link came from, the addresses of a link, and the trailers of the automations (the server
// sends no text; the words are the screen's).
import type { Schemas } from "./api/client";
import { formatRelative, formatTime } from "./format";

export type Link = Schemas["LinkView"];
export type GitHubRepo = Schemas["GitHubRepoView"];
export type Rule = Schemas["RuleView"];
export type IssueFields = Schemas["FieldsView"];

export type PrTone = "open" | "draft" | "merged" | "fail" | "changes" | "closed" | "unread";
export type PrState = { text: string; tone: PrTone };
export type PrChip = PrState & { label: string; href: string; more: number };

/** The one state of a pull request: merged, draft, CI 失敗, 修正依頼, approved, open, in that order. A pull request not read yet has none. */
export function prState(l: Link): PrState {
  if (l.state === "merged") return { text: "merged", tone: "merged" };
  if (l.state === "draft") return { text: "draft", tone: "draft" };
  if (l.state === "closed") return { text: "closed", tone: "closed" };
  if (!l.state) return { text: "", tone: "unread" };
  if (l.checks === "failure") return { text: "CI 失敗", tone: "fail" };
  if (l.review === "changes_requested") return { text: "修正依頼", tone: "changes" };
  if (l.review === "approved") return { text: "approved", tone: "open" };
  return { text: "open", tone: "open" };
}

// What is left to do first: open, draft and not read yet, then closed, then merged.
const rank = (l: Link): number => (l.state === "merged" ? 2 : l.state === "closed" ? 1 : 0);

/** The pull requests of the links, the ones not merged first; the same rank keeps the order of the links. */
export function prLinks(links: readonly Link[] | undefined): Link[] {
  return (links ?? [])
    .filter((l) => l.type === "github_pr")
    .map((l, i) => ({ l, i }))
    .sort((a, b) => rank(a.l) - rank(b.l) || a.i - b.i)
    .map((x) => x.l);
}

/** org-a/web#88 as a chip writes it: web#88. */
export const shortLabel = (label: string): string => label.slice(label.indexOf("/") + 1);

/** The page of a link on GitHub: its ref names the host. */
export const linkUrl = (l: Link): string => `https://${l.ref}`;

/** The chip of a task: its first pull request and how many others it has, or null with none. */
export function prChip(links: readonly Link[] | undefined): PrChip | null {
  const [first, ...rest] = prLinks(links);
  if (!first) return null;
  return {
    ...prState(first),
    label: shortLabel(first.label),
    href: linkUrl(first),
    more: rest.length,
  };
}

/** <host>/<owner>/<repo> of a ref. */
export const repoOf = (ref: string): string => ref.split("/").slice(0, 3).join("/");

/** The repository without github.com, as people write it. */
export const repoLabel = (repo: string): string => repo.replace(/^github\.com\//, "");

/** The link as POST and DELETE /api/tasks/{id}/links take it: [<host>/]owner/repo#n. */
export function linkArg(l: Link): string {
  const number = l.ref.slice(l.ref.lastIndexOf("/") + 1);
  return `${repoLabel(repoOf(l.ref))}#${number}`;
}

const REVIEWS: Readonly<Record<string, string>> = {
  pending: "レビュー待ち",
  approved: "approved",
  changes_requested: "修正依頼",
};

/** The review of a pull request; none and the review of a draft are —. */
export function reviewLabel(l: Link): string {
  if (l.state === "draft") return "—";
  return REVIEWS[l.review ?? ""] ?? "—";
}

const CHECKS: Readonly<Record<string, string>> = {
  success: "CI 成功",
  failure: "CI 失敗",
  pending: "CI 実行中",
};

/** The CI of a pull request; empty when it has none. */
export const checksLabel = (l: Link): string => CHECKS[l.checks ?? ""] ?? "";

const VIAS: Readonly<Record<string, string>> = {
  branch: "ブランチ名から自動リンク",
  mark: "本文の印から自動リンク",
  id: "ID から自動リンク",
  comment: "コメントから自動リンク",
};

/** What linked it; empty for a link made by a person. */
export const viaLabel = (l: Link): string => VIAS[l.via ?? ""] ?? "";

/** The state of an issue, with the tone of its chip. */
export function issueState(l: Link): PrState {
  if (!l.state) return { text: "", tone: "unread" };
  if (l.state === "open") return { text: "open", tone: "open" };
  if (l.state_reason === "not_planned") return { text: "not planned", tone: "closed" };
  return { text: "closed", tone: "merged" };
}

export const SYNC_MODES: readonly { value: Link["sync"]; label: string }[] = [
  { value: "push", label: "push · Hedgehug が正本" },
  { value: "mirror", label: "mirror · GitHub が正本" },
  { value: "link", label: "link · リンクだけ" },
];

const FIELDS: Readonly<Record<string, string>> = {
  title: "タイトル",
  body: "本文",
  status: "状態",
  assignees: "担当者",
  labels: "ラベル",
  milestone: "マイルストーン",
};

/** The fields a link of an issue syncs: its own, else the ones of its mode. The mode link has none. */
export function syncedFields(l: Link, fields: IssueFields | undefined): string {
  if (l.sync === "link") return "";
  const names = l.fields ?? (l.sync === "push" ? fields?.push : fields?.mirror) ?? [];
  return names.map((n) => FIELDS[n] ?? n).join("・");
}

const EVENTS: Readonly<Record<string, string>> = {
  "pr.opened": "PR が開かれた",
  "pr.review_requested": "レビューを依頼された",
  "pr.merged": "PR がマージされた",
  "issue.closed": "Issue が閉じられた",
  "issue.mirror": "Issue の反映",
};

export const eventLabel = (event: string): string => EVENTS[event] ?? event;

export type Applied = { rule: string; event: string };
export type Rejected = { task: string; rule: string; reason: string };

/** The value of Hedgehug-Automation: <rule> <event>. */
export function parseApplied(v: string): Applied | null {
  const [rule, event, ...rest] = v.split(" ");
  return rule && event && rest.length === 0 ? { rule, event } : null;
}

/** The value of Hedgehug-Automation-Rejected: <ULID> <rule> <reason>. */
export function parseRejected(v: string): Rejected | null {
  const m = /^(\S+) (\S+) (.*)$/s.exec(v);
  if (!m) return null;
  const reason = (m[3] ?? "").replace(/\\([\\nr])/g, (_, c: string) =>
    c === "n" ? "\n" : c === "r" ? "\r" : c,
  );
  return { task: m[1] ?? "", rule: m[2] ?? "", reason };
}

/** The words of a kanban column a rule moves tasks into, or null for a rule that shows none. */
export function ruleHint(r: Rule): string | null {
  if (r.on === "pr.opened") return "PR を開くと自動でここへ";
  if (r.on === "pr.merged") {
    return r.when ? "全 PR のマージで自動でここへ" : "PR のマージで自動でここへ";
  }
  return null;
}

export type FetchTimes = { last?: string; next?: string; slowedUntil?: string };

const later = (a: string | undefined, b: string | undefined): string | undefined =>
  !a ? b : !b ? a : Date.parse(a) >= Date.parse(b) ? a : b;
const earlier = (a: string | undefined, b: string | undefined): string | undefined =>
  !a ? b : !b ? a : Date.parse(a) <= Date.parse(b) ? a : b;

/** The times of the repositories named: the latest read, the first read to come, the last recovery. */
export function fetchTimes(all: readonly GitHubRepo[], repos: ReadonlySet<string>): FetchTimes {
  const out: FetchTimes = {};
  for (const r of all) {
    if (!repos.has(r.repo)) continue;
    out.last = later(out.last, r.last_fetch);
    out.next = earlier(out.next, r.next_fetch);
    out.slowedUntil = later(out.slowedUntil, r.slowed_until);
  }
  return out;
}

/** 3 分前に取得; empty before the first read. */
export function fetchedText(last: string | undefined, now: Date): string {
  if (!last) return "";
  const when = formatRelative(last, now);
  return when === "今" ? "たった今取得" : `${when}に取得`;
}

/** GitHub の取得 3 分前 · 次は 2 分後; the second half only for a read still to come. */
export function fetchLine(t: FetchTimes, now: Date): string {
  if (!t.last) return "GitHub からまだ取得していません";
  const head = `GitHub の取得 ${formatRelative(t.last, now)}`;
  const ms = t.next ? Date.parse(t.next) - now.getTime() : 0;
  return ms > 0 ? `${head} · 次は ${Math.ceil(ms / 60_000)} 分後` : head;
}

// The reads of a repository are every 5 minutes, and three times that while the rate limit is low
// (internal/app/storage/ghstate); the API gives the time of the recovery, not the interval.
const SLOWED_MINUTES = 15;

/** The band of a rate limit, or null when the reads are not held back. */
export function slowedText(until: string | undefined, now: Date): string | null {
  if (!until || Date.parse(until) <= now.getTime()) return null;
  return `取得を ${SLOWED_MINUTES} 分おきに延ばしています · ${formatTime(until)} に回復`;
}
