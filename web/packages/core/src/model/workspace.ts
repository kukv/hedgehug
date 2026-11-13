import type { Kind } from "./item";

export interface Workspace {
  id: string;
  storeId: string;
  name: string;
  prefix: string;
}

export type Category = "todo" | "doing" | "done" | "canceled";

export type PropertyType =
  | "status"
  | "user"
  | "priority"
  | "date"
  | "number"
  | "duration"
  | "systems"
  | "text"
  | "single"
  | "multi"
  | "check"
  | "url";

export interface WorkspaceValue {
  id: string;
  name: string;
  color: string;
  category?: Category;
}

export interface WorkspaceProperty {
  id: string;
  key: string;
  name: string;
  type: PropertyType;
  builtin: boolean;
  kinds: string[];
  hidden: boolean;
  choices: WorkspaceValue[];
}

// ProjectRules は、1 つのプロジェクトのルール(B5a-18)。どれも空ならワークスペースの設定のまま。
export interface ProjectRules {
  // 使わないステータスの ID
  hiddenStatuses: string[];
  // From → このプロジェクトで変更できる To(制限のない From はない)
  workflow: Record<string, string[]>;
  // 使うテンプレートの ID(空ならすべて使う)
  templates: string[];
  // 種類 → テンプレート ID か "@none"(決めていない種類はない)
  templateDefaults: Partial<Record<Kind, string>>;
  // 動かさないワークスペースのルールの ID
  disabledRules: string[];
}

export interface WorkspaceSettings {
  properties: WorkspaceProperty[];
  statuses: WorkspaceValue[];
  priorities: WorkspaceValue[];
  systems: WorkspaceValue[];
  initialStatus: string;
  initialPriority: string;
  categoryDefaults: Partial<Record<Category, string>>;
  // From → 許可する To(制限のない From はない)
  workflow: Record<string, string[]>;
  templates: WorkspaceTemplate[];
  // 種類 → テンプレート ID
  templateDefaults: Partial<Record<string, string>>;
  // プロジェクトの ID → そのプロジェクトのルール(ルールのないプロジェクトはない)
  projectRules: Record<string, ProjectRules>;
}

export interface WorkspaceDetail extends Workspace {
  oldPrefixes: string[];
  settings: WorkspaceSettings;
}

export interface WorkspaceUsage {
  items: number;
  firstNumbers: number[];
  // 値の ID ごと・追加プロパティの ID ごとの件数
  values: Record<string, number>;
  properties: Record<string, number>;
}

// ValueList は、"statuses"・"priorities"・"systems" か、選択肢のプロパティの ID。
export type ValueList = string;

export interface NewProperty {
  name: string;
  key: string;
  type: PropertyType;
  kinds: string[];
}

export const stepParent = "@parent"; // Step の担当者「親と同じ」
export const stepNone = "@none"; // Step の担当者「未割り当て」

export type ChildKind = "Story" | "Task" | "Bug";

export interface TemplateStep {
  id: string;
  title: string;
  assignee: string;
}

export interface TemplateChild {
  id: string;
  kind: ChildKind;
  title: string;
  description: string;
  values: Record<string, unknown>;
  steps: TemplateStep[];
}

export interface TemplateInput {
  name: string;
  kind: string;
  title: string;
  description: string;
  values: Record<string, unknown>;
  children: TemplateChild[];
  steps: TemplateStep[];
}

export interface WorkspaceTemplate extends TemplateInput {
  id: string;
}

export interface SaveAsTemplateInput {
  name: string;
  title: string;
  structure: boolean;
}

export type AutomationTrigger =
  | "pr_open"
  | "review_requested"
  | "pr_merged"
  | "issue_closed"
  | "status"
  | "steps_done";
export type AutomationAction = "status" | "assignee";

export interface AutomationRuleInput {
  name: string;
  trigger: AutomationTrigger;
  // きっかけが status のときのステータスの ID。ほかは空
  triggerStatus: string;
  // きっかけが pr_merged のときの条件
  allMerged: boolean;
  action: AutomationAction;
  // ステータスの ID、ログイン名、または @none(未割り当て)
  value: string;
  // このプロジェクトだけのルールならプロジェクトの ID。空か未指定ならワークスペース全体。作成のときだけ送る
  scope?: string;
}

export interface AutomationRule extends AutomationRuleInput {
  id: string;
  enabled: boolean;
  // 専用ルールの持ち主のプロジェクトの表示 ID(HH-P1)と、削除済みか。一覧でだけ埋まる
  scopeLabel?: string;
  scopeDeleted?: boolean;
}

export interface AutomationRun {
  at: Date;
  item: string;
  display: string;
  text: string;
}

export interface WorkspaceAutomation {
  rules: AutomationRule[];
  recent: AutomationRun[];
}
