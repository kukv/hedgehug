import type {
  AutomationRule,
  Category,
  ProjectRules,
  WorkspaceSettings,
  WorkspaceTemplate,
  WorkspaceValue,
} from "../../model/workspace";
import { categoryLabels } from "./values";

// プロジェクト別のルール(03「プロジェクト別のルール」)の判定と、画面の行・文。判定は Go の
// internal/domain/projectrule(judge.go・drafts.go・follow.go)と同じ規則と文言。

// templateDefaults の値で、「テンプレートを使わない」の印。
export const noTemplate = "@none";

const emptyRules: ProjectRules = {
  hiddenStatuses: [],
  workflow: {},
  templates: [],
  templateDefaults: {},
  disabledRules: [],
};

// rulesOf は、プロジェクトのルール。なければ空(ワークスペースの設定のまま)。
export function rulesOf(settings: WorkspaceSettings, projectId: string): ProjectRules {
  return settings.projectRules[projectId] ?? emptyRules;
}

// templateDefaultOf は、プロジェクトの種類 kind の既定(未設定なら undefined)。
export function templateDefaultOf(r: ProjectRules, kind: string): string | undefined {
  return Object.entries(r.templateDefaults).find(([k]) => k === kind)?.[1];
}

const categories: Category[] = ["todo", "doing", "done", "canceled"];

// defaultCategory は、id を既定のステータスにしている分類(分類の並びで最初のもの)。
function defaultCategory(settings: WorkspaceSettings, id: string): Category | null {
  return categories.find((c) => settings.categoryDefaults[c] === id) ?? null;
}

// usesIn は、ルール r のプロジェクトがステータス id を使うか(B5a-5)。外していないか、初期ステータスか、
// 分類ごとの既定のどれかなら使う(外した印は残すが、使うものとして扱う)。
export function usesIn(settings: WorkspaceSettings, r: ProjectRules, id: string): boolean {
  return (
    !r.hiddenStatuses.includes(id) ||
    settings.initialStatus === id ||
    defaultCategory(settings, id) !== null
  );
}

export function usesStatus(settings: WorkspaceSettings, projectId: string, id: string): boolean {
  return usesIn(settings, rulesOf(settings, projectId), id);
}

export const unusedRefusal = (name: string): string =>
  `このプロジェクトでは「${name}」を使いません`;

// workspaceAllowed は、from からワークスペースの制約で変えられる To(ステータスの並び。いまあるものだけ)。
// 空なら制限なし。
export function workspaceAllowed(settings: WorkspaceSettings, from: string): string[] {
  const set = settings.workflow[from] ?? [];
  return settings.statuses.map((v) => v.id).filter((id) => id !== from && set.includes(id));
}

// projectTargets は、from の行のプロジェクトのチェックのうち、From と To の両方が使うステータスのもの。
export function projectTargets(
  settings: WorkspaceSettings,
  r: ProjectRules,
  from: string,
): string[] {
  if (!usesIn(settings, r, from)) return [];
  const set = r.workflow[from] ?? [];
  return settings.statuses
    .map((v) => v.id)
    .filter((id) => id !== from && usesIn(settings, r, id) && set.includes(id));
}

// allowedIn は、from から変えられる To(ステータスの並び)と、制限があるか(B5a-6)。ワークスペースの
// 許す To とプロジェクトの許す To の両方に入るもので、使わないステータスと from は除く。
export function allowedIn(
  settings: WorkspaceSettings,
  r: ProjectRules,
  from: string,
): { to: string[]; restricted: boolean } {
  const ws = workspaceAllowed(settings, from);
  const pr = projectTargets(settings, r, from);
  const to = settings.statuses
    .map((v) => v.id)
    .filter(
      (id) =>
        id !== from &&
        usesIn(settings, r, id) &&
        (ws.length === 0 || ws.includes(id)) &&
        (pr.length === 0 || pr.includes(id)),
    );
  return { to, restricted: ws.length > 0 || pr.length > 0 };
}

export function allowedTargets(settings: WorkspaceSettings, projectId: string, from: string) {
  return allowedIn(settings, rulesOf(settings, projectId), from);
}

// usableIn は、種類 kind のテンプレートのうち、ルール r のプロジェクトで使えるもの(B5a-9)。使うテンプレート
// のうち、いまあるテンプレートが 0 件なら、種類の合うすべて。
export function usableIn(
  settings: WorkspaceSettings,
  r: ProjectRules,
  kind: string,
): WorkspaceTemplate[] {
  const narrowed = r.templates.some((id) => settings.templates.some((t) => t.id === id));
  return settings.templates.filter(
    (t) => t.kind === kind && (!narrowed || r.templates.includes(t.id)),
  );
}

// defaultIn は、種類 kind の既定のテンプレート(B5a-9)。プロジェクトの既定が「なし」なら null、テンプレート
// ID ならそれが使えればそれ・使えなければ null(ワークスペースの既定には戻らない)、未設定ならワークス
// ペースの既定が使えればそれ、なければ null。
export function defaultIn(
  settings: WorkspaceSettings,
  r: ProjectRules,
  kind: string,
): WorkspaceTemplate | null {
  const usable = usableIn(settings, r, kind);
  const own = templateDefaultOf(r, kind);
  const id = own === undefined || own === "" ? settings.templateDefaults[kind] : own;
  if (id === undefined || id === noTemplate) return null;
  return usable.find((t) => t.id === id) ?? null;
}

// ---- 画面の材料 ----

export interface UsedStatusRow {
  id: string;
  name: string;
  color: string;
  category: Category;
  // 使うステータスならチェック(外せないものはチェックのまま)
  checked: boolean;
  // 初期ステータス・分類の既定は外せない(チェックを無効にする)
  disabled: boolean;
  // 外せない理由
  note?: string;
  // 外してあるステータスを使っているアイテムの件数の文
  usage?: string;
}

function cannotHideNote(settings: WorkspaceSettings, v: WorkspaceValue): string | undefined {
  const initial = settings.initialStatus === v.id;
  const cat = defaultCategory(settings, v.id);
  const label = cat === null ? "" : categoryLabels[cat];
  if (initial && cat !== null) return `初期ステータスで、${label}の既定でもあるため外せません`;
  if (initial) return "初期ステータスのため外せません";
  if (cat !== null) return `${label}の既定のため外せません`;
  return undefined;
}

// usedStatusRows は、「使うステータス」の区画の行(定義の順)と件数「{使う数} / {全数}」。items は
// プロジェクトのアイテム(削除済みを除く)。
export function usedStatusRows(
  settings: WorkspaceSettings,
  projectId: string,
  items: readonly { status: string }[],
): { count: string; rows: UsedStatusRow[] } {
  const r = rulesOf(settings, projectId);
  const rows = settings.statuses.map((v): UsedStatusRow => {
    const used = usesIn(settings, r, v.id);
    const note = cannotHideNote(settings, v);
    const n = items.filter((it) => it.status === v.id).length;
    return {
      id: v.id,
      name: v.name,
      color: v.color,
      category: v.category ?? "todo",
      checked: used,
      disabled: note !== undefined,
      ...(note ? { note } : {}),
      ...(!used && n > 0 ? { usage: `${n} 件が使用中` } : {}),
    };
  });
  return { count: `${rows.filter((x) => x.checked).length} / ${rows.length}`, rows };
}

// RuleProject は、ルールの一覧に出すプロジェクト(削除済みを除いたもの)。display は表示 ID(HH-P1)。
export interface RuleProject {
  id: string;
  display: string;
  title: string;
}

export type RuleSection = "statuses" | "workflow" | "templates" | "automation";

// hasRules は、プロジェクトのルールに区画 section の行があるか(「ワークスペースの設定に戻す」の有効・無効)。
// 自動化の専用ルールは含まない(ルールの一覧から数える)。
export function hasRules(r: ProjectRules, section: RuleSection): boolean {
  switch (section) {
    case "statuses":
      return r.hiddenStatuses.length > 0;
    case "workflow":
      return Object.keys(r.workflow).length > 0;
    case "templates":
      return r.templates.length > 0 || Object.keys(r.templateDefaults).length > 0;
    case "automation":
      return r.disabledRules.length > 0;
  }
}

// rulesProjects は、サブタブの区画に設定を持つプロジェクトの 1 行(03「一覧」)。ルールが空のプロジェクトと、
// projects にない(削除済みの)プロジェクトは除く。自動化の区画は、専用ルール(rules の scope)を持つプロジェクトも
// 数える。なければ null。
export function rulesProjects(
  settings: WorkspaceSettings,
  section: RuleSection,
  projects: readonly RuleProject[],
  rules: readonly AutomationRule[] = [],
): { projects: RuleProject[]; text: string } | null {
  const found = projects.filter((p) => {
    if (section === "automation" && rules.some((x) => x.scope === p.id)) return true;
    const r = settings.projectRules[p.id];
    return r !== undefined && hasRules(r, section);
  });
  if (found.length === 0) return null;
  const names = found.map((p) => `${p.display} ${p.title}`).join("・");
  return {
    projects: found,
    text: `プロジェクト別のルールがあるプロジェクト: ${names}(${found.length} 件)`,
  };
}

export const rulesHeading = (workspaceName: string): string =>
  `このプロジェクトだけのルールです。ステータスや種類の定義は ${workspaceName} のままです。`;

// templateDeleteLines は、テンプレートの削除の確認に添える文(B5a-17)。変わるプロジェクトの数と、
// すべてのテンプレートを使えるようになるプロジェクト。削除済みのテンプレートの残りは数えない。
export function templateDeleteLines(
  settings: WorkspaceSettings,
  templateId: string,
  projects: readonly RuleProject[],
): string[] {
  const affected = projects.filter((p) => {
    const r = settings.projectRules[p.id];
    return (
      r !== undefined &&
      (r.templates.includes(templateId) || Object.values(r.templateDefaults).includes(templateId))
    );
  });
  if (affected.length === 0) return [];
  const unlimited = affected.filter((p) => {
    const r = rulesOf(settings, p.id);
    return (
      r.templates.includes(templateId) &&
      !r.templates.some((id) => id !== templateId && settings.templates.some((t) => t.id === id))
    );
  });
  const lines = [`${affected.length} 件のプロジェクト別のルールからも削除されます。`];
  if (unlimited.length > 0) {
    lines.push(
      `${unlimited.map((p) => p.display).join("・")} は、すべてのテンプレートを使えるようになります。`,
    );
  }
  return lines;
}
