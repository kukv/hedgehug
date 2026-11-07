// The rules of automations.yaml as the settings show them: the words of a rule, how it went in the
// last 7 days, and what it means in a project. The server sends no text.
import { eventLabel, fieldLabel, type Rule, type RuleStat } from "./github";
import { defaultStatusIn, type SchemaView } from "./schema";

/** The events a rule can be on; issue.mirror belongs to the sync, not to a rule. */
export const RULE_EVENTS = ["pr.opened", "pr.review_requested", "pr.merged", "issue.closed"];

export const WHEN_ALL_MERGED = "all_linked_prs_merged";

const statusLabel = (s: SchemaView, name: string): string =>
  (s.statuses ?? []).find((x) => x.name === name)?.label || name;

/** The status a rule moves a task to: its own, or the default of its category. */
export const ruleTarget = (r: Rule, s: SchemaView): string =>
  r.status ?? defaultStatusIn(s, r.category ?? "");

/** ステータス → 進行中, or カテゴリ done の既定（完了）. */
export function setLabel(r: Rule, s: SchemaView): string {
  const target = statusLabel(s, ruleTarget(r, s));
  return r.status ? `ステータス → ${target}` : `カテゴリ ${r.category} の既定（${target}）`;
}

export const onLabel = (r: Rule): string => eventLabel(r.on);

export const whenLabel = (when: string): string =>
  when === WHEN_ALL_MERGED ? "リンク中の PR がすべてマージ済み" : when;

/** The rule as automations.yaml writes it. */
export function ruleYaml(r: Rule): string {
  const parts = [`id: ${r.id}`, `on: ${r.on}`];
  if (r.when) parts.push(`when: ${r.when}`);
  if (r.project) parts.push(`where: project:${r.project.ulid}`);
  return parts.join(" · ");
}

/** 直近 7 日 5 回 · 拒否 1, or 無効. */
export function statLabel(r: Rule, stat: RuleStat | undefined): string {
  if (!r.enabled) return "無効";
  const head = `直近 7 日 ${stat?.applied ?? 0} 回`;
  return stat?.rejected ? `${head} · 拒否 ${stat.rejected}` : head;
}

function rejectedNote(stat: RuleStat | undefined): string {
  if (!stat?.rejected) return "";
  const [first, ...rest] = stat.rejected_tasks;
  if (!first) return `直近 7 日で、制約により ${stat.rejected} 回拒否されました。`;
  const on = rest.length > 0 ? `${first.id} ほか ${rest.length} 件` : first.id;
  return `直近 7 日では ${on} で、制約により ${stat.rejected} 回拒否されました。`;
}

/**
 * What a rule may be refused for: the fields its status needs, and how often it was refused.
 * required are the names as the screen shows them. Null with nothing to say.
 */
export function requiredNote(
  r: Rule,
  s: SchemaView,
  required: readonly string[],
  stat: RuleStat | undefined,
): string | null {
  const refused = rejectedNote(stat);
  if (required.length === 0) return refused || null;
  const names = required.join("・");
  const head = `${statusLabel(s, ruleTarget(r, s))}には ${names} が必要です。${names} が空のタスクでは拒否され、通知に出ます。`;
  return head + refused;
}

/** A rule on the settings of a project. project: what the switch is sent with. */
export type ProjectRuleRow = {
  source: string;
  own: boolean;
  checked: boolean;
  fixed: boolean;
  state: string;
  project: string | undefined;
};

/**
 * What a rule is in a project. r comes from the list of the project, whose enabled is both switches
 * together; whole from the list of the Store, whose enabled is the rule's own.
 */
export function projectRuleRow(
  r: Rule,
  whole: Rule | undefined,
  ulid: string,
  store: string,
): ProjectRuleRow {
  const own = r.project?.ulid === ulid;
  const source = own ? "このプロジェクトだけ" : (r.project?.title ?? store);
  const row = { source, own, project: ulid };
  if (r.disabled_in.some((p) => p.ulid === ulid)) {
    return { ...row, checked: false, fixed: false, state: "このプロジェクトで無効" };
  }
  if (own) {
    return {
      ...row,
      project: undefined,
      checked: r.enabled,
      fixed: false,
      state: r.enabled ? "有効" : "無効",
    };
  }
  if (whole && !whole.enabled) {
    return { ...row, checked: false, fixed: true, state: `${source} で無効` };
  }
  if (!r.enabled) {
    return { ...row, checked: false, fixed: true, state: "親のプロジェクトで無効" };
  }
  return { ...row, checked: true, fixed: false, state: "このプロジェクトで有効" };
}

export type IssueField = { name: string; label: string; to: string; note: string };

/** The rows of the table of the issue sync, in the order of IssueFieldNames. */
export const ISSUE_FIELDS: readonly IssueField[] = [
  { name: "title", to: "title", note: "" },
  { name: "body", to: "body", note: "本文の末尾に hedgehug:<ULID> を入れます" },
  {
    name: "status",
    to: "status",
    note: "完了カテゴリは closed（completed）、中止は closed（not_planned）",
  },
  { name: "assignees", to: "assignees", note: "そのリポジトリで割り当てられる人だけ" },
  { name: "labels", to: "labels", note: "リポジトリにあるラベルだけ。ないものは作らずに警告" },
  { name: "milestone", to: "props", note: "存在しないマイルストーンは作らずに警告" },
].map((f) => ({ ...f, label: fieldLabel(f.name) }));

/** The fields with one turned on or off, in the order of the table. */
export function withField(names: readonly string[], name: string, on: boolean): string[] {
  const next = new Set(names);
  if (on) next.add(name);
  else next.delete(name);
  return ISSUE_FIELDS.map((f) => f.name).filter((n) => next.has(n));
}
