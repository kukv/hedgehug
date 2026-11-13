import type { Item, Kind, PropertyInput } from "../../model/item";
import type { Member } from "../../model/member";
import type { WorkspaceProperty, WorkspaceSettings, WorkspaceValue } from "../../model/workspace";
import { rulesOf, usesIn } from "../workspaces/projectRules";
import { categoryLabels } from "../workspaces/values";
import { transitionRefusal } from "../workspaces/workflow";

// 詳細パネルと作成ダイアログのプロパティの行(02「プロパティの入力と保存」、決定 19〜21)。

export const deletedValue = "(削除された値)";
const unusedHint = "このプロジェクトでは使わない";

export interface ValueOption {
  id: string;
  label: string;
  hint?: string;
  checked: boolean;
  // disabled は選べない項目、reason はその理由(遷移の制約。A3b-5)。
  disabled?: boolean;
  reason?: string;
}
export interface People {
  members: Member[];
  me: string;
}
export interface PropertyRow {
  property: WorkspaceProperty;
  extra: boolean;
  value: PropertyInput;
  text: string;
  deleted: boolean;
  options: ValueOption[];
}

export type ItemValues = Pick<
  Item,
  | "status"
  | "priority"
  | "assignee"
  | "start"
  | "end"
  | "points"
  | "estimate"
  | "systems"
  | "choices"
  | "fields"
>;

const chooser = ["status", "priority", "systems", "single", "multi"];

// visibleProperties は、非表示でなく、その種類で使うプロパティ(設定の並び)。
export function visibleProperties(settings: WorkspaceSettings, kind: Kind): WorkspaceProperty[] {
  return settings.properties.filter((p) => !p.hidden && p.kinds.includes(kind));
}

// valueOf は、アイテムの値を、入力欄に入れる形で返す。数は文字列、複数は ID の配列、チェックは真偽、空は ""。
// biome-ignore lint/suspicious/noShadowRestrictedNames: 仕様の名前(アイテムの値を読む)
export function valueOf(it: ItemValues, p: WorkspaceProperty): PropertyInput {
  if (p.builtin) return builtinValue(it, p.key);
  if (p.type === "multi") return it.choices[p.id] ?? [];
  if (p.type === "single") return it.choices[p.id]?.[0] ?? "";
  const raw = it.fields[p.id];
  if (p.type === "check") return raw === true;
  return raw === undefined || raw === null ? "" : String(raw);
}

function builtinValue(it: ItemValues, key: string): PropertyInput {
  switch (key) {
    case "points":
      return it.points === null ? "" : String(it.points);
    case "estimate":
      return it.estimate === null ? "" : String(it.estimate);
    case "systems":
      return it.systems;
    case "status":
    case "priority":
    case "assignee":
    case "start":
    case "end":
      return it[key];
  }
  return "";
}

function listOf(settings: WorkspaceSettings, p: WorkspaceProperty): WorkspaceValue[] {
  if (p.type === "status") return settings.statuses;
  if (p.type === "priority") return settings.priorities;
  if (p.type === "systems") return settings.systems;
  return p.choices;
}

// statusOptions は、ステータスのメニューの項目(決定 21)。workflow なら、遷移の制約で移れない項目を無効にし、
// 理由を添える(02「ステータスの変更」、A3b-4)。作成ダイアログは制約を適用しないので false。
// project があれば、そのプロジェクトが使うステータスだけを並べる。いまの値が使わないものなら、その 1 項目を
// チェックつきで残し、「このプロジェクトでは使わない」と添える(03「使うステータス」)。
export function statusOptions(
  settings: WorkspaceSettings,
  current: string,
  workflow = false,
  project?: string,
): ValueOption[] {
  const rules = project === undefined ? undefined : rulesOf(settings, project);
  const unused = (id: string) => rules !== undefined && !usesIn(settings, rules, id);
  return settings.statuses
    .filter((s) => !unused(s.id) || s.id === current)
    .map((s) => {
      const reason = workflow ? transitionRefusal(settings, current, s.id, project) : null;
      return {
        id: s.id,
        label: s.name,
        hint: unused(s.id) ? unusedHint : s.category ? categoryLabels[s.category] : undefined,
        checked: s.id === current,
        ...(reason ? { disabled: true, reason } : {}),
      };
    });
}

// projectStatusOptions は、プロジェクトが使うステータスのメニュー。
export const projectStatusOptions = (
  settings: WorkspaceSettings,
  project: string,
  current: string,
  workflow = false,
): ValueOption[] => statusOptions(settings, current, workflow, project);

export const unknownUser = "不明なユーザー";

// peopleOptions は、担当者のメニュー。ログイン名はメンバーと大文字小文字を区別せず比べる(01)。
// メンバーにいない値は、「不明なユーザー」として選ばれた項目にする。
function peopleOptions(people: People, current: string): ValueOption[] {
  const same = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();
  const options: ValueOption[] = [
    { id: "", label: "未割り当て", checked: current === "" },
    ...people.members.map((m) => ({
      id: m.login,
      label: same(m.login, people.me) ? `${m.name}(あなた)` : m.name,
      checked: current !== "" && same(m.login, current),
    })),
  ];
  if (current !== "" && !options.some((o) => o.checked)) {
    options.push({ id: current, label: unknownUser, checked: true });
  }
  return options;
}

function choiceRow(settings: WorkspaceSettings, p: WorkspaceProperty, value: PropertyInput) {
  const list = listOf(settings, p);
  const ids = Array.isArray(value) ? value : value ? [String(value)] : [];
  const names = ids.map((id) => list.find((v) => v.id === id)?.name);
  const required = p.type === "status" || p.type === "priority";
  const deleted = names.some((n) => n === undefined) || (required && ids.length === 0);
  const text = deleted ? deletedValue : names.length > 0 ? names.join(", ") : "なし";
  const options =
    p.type === "status"
      ? statusOptions(settings, ids[0] ?? "")
      : list.map((v) => ({ id: v.id, label: v.name, checked: ids.includes(v.id) }));
  if (p.type === "single") options.unshift({ id: "", label: "なし", checked: ids.length === 0 });
  return { text, deleted, options };
}

// propertyRows は、種類で使う表示中のプロパティの行。values はプロパティの入力欄の値を返す。
export function propertyRows(
  settings: WorkspaceSettings,
  kind: Kind,
  values: (p: WorkspaceProperty) => PropertyInput,
  people: People,
): PropertyRow[] {
  return visibleProperties(settings, kind).map((p) => {
    const value = values(p);
    const base = { property: p, extra: !p.builtin, value };
    if (chooser.includes(p.type)) return { ...base, ...choiceRow(settings, p, value) };
    if (p.type === "user") {
      const options = peopleOptions(people, String(value));
      const text = options.find((o) => o.checked)?.label ?? "";
      return { ...base, text, deleted: false, options };
    }
    return { ...base, text: String(value), deleted: false, options: [] };
  });
}
