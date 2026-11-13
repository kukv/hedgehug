import { NotFound, Refused } from "../../model/errors";
import type { Member } from "../../model/member";
import type {
  AutomationAction,
  AutomationRule,
  AutomationRuleInput,
  AutomationTrigger,
  WorkspaceAutomation,
  WorkspaceSettings,
} from "../../model/workspace";
import { relativeTime } from "../dates/relative";
import { rulesOf, usesStatus } from "./projectRules";

// 自動化のタブ(03「自動化」)。文言は 03 の文字どおり。

export const automationHelp =
  "GitHub の出来事やステータスの変化で、アイテムを自動で変えます。自動化が変えたものは受信箱の「自動化」に出ます。";
export const rulesEmpty = "ルールはありません";
export const recentEmpty = "まだ動いていません";
export const ruleCreated = "ルールを作成しました";
export const ruleDeleted = "ルールを削除しました";
export const deleteRuleTitle = "ルールを削除";
export const deleteRuleBody = (name: string) => `「${name}」を削除します。`;
export const switchRefusal = "きっかけと同じステータスには変更できません。ルールを編集してください";
export const unassignedOption = "未割り当てにする";
export const allMergedLabel = "リンク中の PR がすべてマージ済みのときだけ";
const unassigned = "@none";
const sameStatus = "きっかけと同じステータスには変更できません";

export const triggerOptions: { value: AutomationTrigger; label: string }[] = [
  { value: "pr_open", label: "PR がリンクされて Open のとき" },
  { value: "review_requested", label: "レビューが依頼されたとき" },
  { value: "pr_merged", label: "リンクした PR がマージされたとき" },
  { value: "issue_closed", label: "リンクした Issue が閉じたとき" },
  { value: "status", label: "ステータスが次になったとき" },
  { value: "steps_done", label: "Step がすべて完了したとき" },
];

export const actionOptions: { value: AutomationAction; label: string }[] = [
  { value: "status", label: "ステータスを変更" },
  { value: "assignee", label: "担当者を変更" },
];

const statusName = (s: WorkspaceSettings, id: string) =>
  s.statuses.find((v) => v.id === id)?.name ?? "";

function triggerText(s: WorkspaceSettings, r: AutomationRuleInput): string {
  if (r.trigger === "status") return `ステータスが ${statusName(s, r.triggerStatus)} になったとき`;
  if (r.trigger === "pr_merged" && r.allMerged) return "リンクした PR がすべてマージされたとき";
  return triggerOptions.find((o) => o.value === r.trigger)?.label ?? "";
}

function memberName(members: Member[], login: string): string {
  const m = members.find((x) => x.login.toLowerCase() === login.toLowerCase());
  return m?.name || login;
}

function actionText(s: WorkspaceSettings, members: Member[], r: AutomationRuleInput): string {
  if (r.action === "status") return `ステータスを ${statusName(s, r.value)} に`;
  return `担当者を ${r.value === unassigned ? "未割り当て" : memberName(members, r.value)} に`;
}

// ruleSummary は、一覧の説明「{きっかけ} → {すること}」(B4-18)。
export function ruleSummary(
  s: WorkspaceSettings,
  members: Member[],
  r: AutomationRuleInput,
): string {
  return `${triggerText(s, r)} → ${actionText(s, members, r)}`;
}

export interface RuleRow {
  id: string;
  name: string;
  summary: string;
  enabled: boolean;
  // 専用ルールの名前のあとに添える、持ち主のプロジェクトの ID の印「HH-P1」「HH-P1(削除済み)」(03「ルールの一覧」)
  scope?: string;
}

const scopeMark = (r: AutomationRule) => `${r.scopeLabel}${r.scopeDeleted ? "(削除済み)" : ""}`;

export function ruleRows(
  a: WorkspaceAutomation,
  s: WorkspaceSettings,
  members: Member[],
): RuleRow[] {
  return a.rules.map((r) => ({
    id: r.id,
    name: r.name,
    summary: ruleSummary(s, members, r),
    enabled: r.enabled,
    ...(r.scope && r.scopeLabel ? { scope: scopeMark(r) } : {}),
  }));
}

export interface ProjectRuleRow extends RuleRow {
  // 出どころ「ワークスペース」「このプロジェクトだけ」
  origin: string;
  // このプロジェクトだけのルール(編集・削除が付く)
  own: boolean;
  // スイッチを動かせない(ワークスペースで無効のルール)
  locked: boolean;
  note?: string;
}

export const originWorkspace = "ワークスペース";
export const originProject = "このプロジェクトだけ";
export const disabledInWorkspace = "ワークスペースで無効";

// projectRuleRows は、プロジェクトの「自動化」の区画の行(03「プロジェクトの自動化」)。判定されるルール
// (ワークスペース全体と、このプロジェクトだけ)を、渡された順に。ワークスペースのルールのスイッチは「ここで
// 動く」(無効のルールは OFF で動かせない)、専用ルールのスイッチは有効・無効。
export function projectRuleRows(
  a: WorkspaceAutomation,
  s: WorkspaceSettings,
  members: Member[],
  projectId: string,
): ProjectRuleRow[] {
  const excluded = rulesOf(s, projectId).disabledRules;
  return a.rules
    .filter((r) => !r.scope || r.scope === projectId)
    .map((r) => {
      const own = r.scope === projectId;
      const locked = !own && !r.enabled;
      return {
        id: r.id,
        name: r.name,
        summary: ruleSummary(s, members, r),
        enabled: own ? r.enabled : r.enabled && !excluded.includes(r.id),
        origin: own ? originProject : originWorkspace,
        own,
        locked,
        ...(locked ? { note: disabledInWorkspace } : {}),
      };
    });
}

export interface StatusOption {
  value: string;
  label: string;
}

const allStatuses = (s: WorkspaceSettings): StatusOption[] =>
  s.statuses.map((v) => ({ value: v.id, label: v.name }));

// scopeStatuses は、ダイアログのステータスの選択肢。専用ルール(scope あり)は、そのプロジェクトで使うものだけ
// (03「プロジェクトの自動化」)。
export function scopeStatuses(s: WorkspaceSettings, scope?: string): StatusOption[] {
  const used = scope ? s.statuses.filter((v) => usesStatus(s, scope, v.id)) : s.statuses;
  return used.map((v) => ({ value: v.id, label: v.name }));
}

// keepStatus は、保存済みのルールのステータス id が選択肢にないとき(ステータスの削除の書き換えで、使わない
// ステータスを指しているとき)、いまの値を見せるために、定義の順で選択肢に足す。
export function keepStatus(
  s: WorkspaceSettings,
  options: readonly StatusOption[],
  id: string,
): StatusOption[] {
  if (options.some((o) => o.value === id) || !s.statuses.some((v) => v.id === id)) {
    return [...options];
  }
  const keep = new Set([...options.map((o) => o.value), id]);
  return allStatuses(s).filter((o) => keep.has(o.value));
}

const firstOf = (s: WorkspaceSettings, statuses: readonly StatusOption[]) =>
  statuses[0]?.value ?? s.statuses[0]?.id ?? "";

// newRuleForm は、作成ダイアログの初期値(03)。値は完了の既定、選択肢になければ選択肢の先頭。
export function newRuleForm(
  s: WorkspaceSettings,
  statuses: readonly StatusOption[] = allStatuses(s),
): AutomationRuleInput {
  const done = s.categoryDefaults.done;
  return {
    name: "",
    trigger: "pr_merged",
    triggerStatus: "",
    allMerged: false,
    action: "status",
    value: done && statuses.some((o) => o.value === done) ? done : firstOf(s, statuses),
  };
}

// ruleForm は、編集ダイアログの初期値。入力の 6 項目だけを持つ(id・enabled は送らない)。
export function ruleForm(r: AutomationRuleInput): AutomationRuleInput {
  const { name, trigger, triggerStatus, allMerged, action, value } = r;
  return { name, trigger, triggerStatus, allMerged, action, value };
}

// withTrigger は、きっかけを変える。条件は pr_merged のときだけ残し、ステータスのきっかけには選択肢の先頭(B4-19)。
export function withTrigger(
  f: AutomationRuleInput,
  trigger: AutomationTrigger,
  s: WorkspaceSettings,
  statuses: readonly StatusOption[] = allStatuses(s),
): AutomationRuleInput {
  return {
    ...f,
    trigger,
    triggerStatus: trigger === "status" ? f.triggerStatus || firstOf(s, statuses) : "",
    allMerged: trigger === "pr_merged" ? f.allMerged : false,
  };
}

// withAction は、することを変え、値を初期化する(ステータスなら選択肢の先頭、担当者なら未割り当て)。
export function withAction(
  f: AutomationRuleInput,
  action: AutomationAction,
  s: WorkspaceSettings,
  statuses: readonly StatusOption[] = allStatuses(s),
): AutomationRuleInput {
  return { ...f, action, value: action === "status" ? firstOf(s, statuses) : unassigned };
}

export function valueOptions(
  f: AutomationRuleInput,
  s: WorkspaceSettings,
  members: Member[],
  statuses: readonly StatusOption[] = allStatuses(s),
): StatusOption[] {
  if (f.action === "status") return [...statuses];
  return [
    { value: unassigned, label: unassignedOption },
    ...members.map((m) => ({ value: m.login, label: m.name || m.login })),
  ];
}

export const ruleSaveFailed = "ルールを保存できませんでした";

// ruleSaveRefusal は、ルールの保存の失敗を、ダイアログの中に出す文にする。断りはその文言、指したものがない
// (ほかで消された・使わないステータスになった)ときは ruleSaveFailed。ほかの失敗は投げ直す。
export function ruleSaveRefusal(e: unknown): string {
  if (e instanceof Refused) return e.message;
  if (e instanceof NotFound) return ruleSaveFailed;
  throw e;
}

const sameStatusRule = (r: AutomationRuleInput) =>
  r.trigger === "status" && r.action === "status" && r.triggerStatus === r.value;

// ruleFormRefusal は、Go の Check と同じ検査と文言(03 の保存の条件)。通るなら null。
export function ruleFormRefusal(
  f: AutomationRuleInput,
  rules: AutomationRule[],
  editingId: string,
): string | null {
  const name = f.name.trim();
  if (name === "") return "名前を入力してください";
  if (rules.some((r) => r.id !== editingId && r.name === name)) return "同じ名前のルールがあります";
  return sameStatusRule(f) ? sameStatus : null;
}

// switchOnRefusal は、スイッチを ON にする前の検査(03)。
export function switchOnRefusal(r: AutomationRule): string | null {
  return sameStatusRule(r) ? switchRefusal : null;
}

export interface RecentRow {
  display: string;
  text: string;
  time: string;
}

export function recentRows(a: WorkspaceAutomation, now: Date): RecentRow[] {
  return a.recent.map((r) => ({
    display: r.display,
    text: r.text,
    time: relativeTime(r.at, now),
  }));
}
