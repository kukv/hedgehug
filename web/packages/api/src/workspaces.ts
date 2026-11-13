import {
  type AutomationRule,
  type ChildKind,
  type ProjectRules,
  type PropertyType,
  ruleForm,
  type TemplateChild,
  type TemplateInput,
  type TemplateStep,
  type WorkspaceDetail,
  type WorkspaceProperty,
  type WorkspacesPort,
  type WorkspaceTemplate,
  type WorkspaceUsage,
  type WorkspaceValue,
} from "@hedgehug/core";
import { call } from "./errors";
import {
  type AddWorkspaceProperty,
  type WorkspaceDetail as ApiDetail,
  type ProjectRules as ApiProjectRules,
  type WorkspaceProperty as ApiProperty,
  type AutomationRule as ApiRule,
  type WorkspaceTemplate as ApiTemplate,
  type WorkspaceTemplateChild as ApiTemplateChild,
  type WorkspaceTemplateInput as ApiTemplateInput,
  type WorkspaceTemplateStep as ApiTemplateStep,
  type WorkspaceUsage as ApiUsage,
  type WorkspaceValue as ApiValue,
  addWorkspaceProperty,
  addWorkspaceValue,
  changeWorkspacePrefix,
  createAutomationRule,
  createWorkspace,
  createWorkspaceTemplate,
  deleteAutomationRule,
  deleteWorkspaceProperty,
  deleteWorkspaceTemplate,
  deleteWorkspaceValue,
  getWorkspace,
  getWorkspaceAutomation,
  getWorkspaceUsage,
  moveWorkspace,
  moveWorkspaceProperty,
  moveWorkspaceValue,
  renameWorkspace,
  renameWorkspaceProperty,
  renameWorkspaceValue,
  type SetWorkspaceValueColorColor,
  saveItemAsTemplate,
  setAutomationRuleEnabled,
  setWorkspaceCategoryDefault,
  setWorkspaceInitialPriority,
  setWorkspaceInitialStatus,
  setWorkspacePropertyHidden,
  setWorkspacePropertyKinds,
  setWorkspaceStatusCategory,
  setWorkspaceTemplateDefault,
  setWorkspaceTransition,
  setWorkspaceValueColor,
  updateAutomationRule,
  updateWorkspaceTemplate,
} from "./generated";

// ワークスペースと設定の口の実装。API の型を core の型に直す(設計書 6 節)。
// 空の一覧・集計は null で届いてもよいので、空にする。

const toValue = (v: ApiValue): WorkspaceValue => v;

function toProperty(p: ApiProperty): WorkspaceProperty {
  return {
    ...p,
    type: p.type as PropertyType,
    kinds: p.kinds ?? [],
    choices: (p.choices ?? []).map(toValue),
  };
}

const toStep = (s: ApiTemplateStep): TemplateStep => s;

const toChild = (c: ApiTemplateChild): TemplateChild => ({
  ...c,
  kind: c.kind as ChildKind,
  values: c.values ?? {},
  steps: (c.steps ?? []).map(toStep),
});

const toProjectRules = (r: ApiProjectRules | null): ProjectRules => ({
  hiddenStatuses: r?.hiddenStatuses ?? [],
  workflow: Object.fromEntries(
    Object.entries(r?.workflow ?? {}).map(([from, tos]) => [from, tos ?? []]),
  ),
  templates: r?.templates ?? [],
  templateDefaults: r?.templateDefaults ?? {},
  disabledRules: r?.disabledRules ?? [],
});

const toTemplate = (t: ApiTemplate): WorkspaceTemplate => ({
  ...t,
  values: t.values ?? {},
  children: (t.children ?? []).map(toChild),
  steps: (t.steps ?? []).map(toStep),
});

function toDetail(storeId: string, d: ApiDetail): WorkspaceDetail {
  const s = d.settings;
  return {
    id: d.id,
    storeId,
    name: d.name,
    prefix: d.prefix,
    oldPrefixes: d.oldPrefixes ?? [],
    settings: {
      properties: (s.properties ?? []).map(toProperty),
      statuses: (s.statuses ?? []).map(toValue),
      priorities: (s.priorities ?? []).map(toValue),
      systems: (s.systems ?? []).map(toValue),
      initialStatus: s.initialStatus,
      initialPriority: s.initialPriority,
      categoryDefaults: s.categoryDefaults ?? {},
      workflow: Object.fromEntries(
        Object.entries(s.workflow ?? {}).map(([from, tos]) => [from, tos ?? []]),
      ),
      templates: (s.templates ?? []).map(toTemplate),
      templateDefaults: s.templateDefaults ?? {},
      projectRules: Object.fromEntries(
        Object.entries(s.projectRules ?? {}).map(([p, r]) => [p, toProjectRules(r)]),
      ),
    },
  };
}

const toUsage = (u: ApiUsage): WorkspaceUsage => ({
  items: u.items,
  firstNumbers: u.firstNumbers ?? [],
  values: u.values ?? {},
  properties: u.properties ?? {},
});

const toTemplateBody = (input: TemplateInput) => input as ApiTemplateInput;

// API の列挙(きっかけ・すること)は core の型と同じ文字列。scope は空ならワークスペース全体(API は空を省く)。
const toRule = (r: ApiRule): AutomationRule => r;

// 入力は 6 項目だけを送る。API の入力は追加の項目(id・enabled)を断る。
const toRuleBody = ruleForm;

export function createWorkspacesPort(): WorkspacesPort {
  return {
    get: (s, w) => call(async () => toDetail(s, await getWorkspace(s, w))),
    usage: (s, w) => call(async () => toUsage(await getWorkspaceUsage(s, w))),
    create: (s, name, prefix) =>
      call(async () => {
        const w = await createWorkspace(s, { name, prefix });
        return { id: w.id, storeId: s, name: w.name, prefix: w.prefix };
      }),
    rename: (s, w, name) => call(() => renameWorkspace(s, w, { name })),
    changePrefix: (s, w, prefix) => call(() => changeWorkspacePrefix(s, w, { prefix })),
    move: (s, w, direction) => call(() => moveWorkspace(s, w, { direction })),
    addProperty: (s, w, input) =>
      call(async () =>
        toProperty(
          await addWorkspaceProperty(s, w, {
            ...input,
            type: input.type as AddWorkspaceProperty["type"],
          }),
        ),
      ),
    renameProperty: (s, w, id, name) => call(() => renameWorkspaceProperty(s, w, id, { name })),
    setPropertyKinds: (s, w, id, kinds) =>
      call(() => setWorkspacePropertyKinds(s, w, id, { kinds })),
    setPropertyHidden: (s, w, id, hidden) =>
      call(() => setWorkspacePropertyHidden(s, w, id, { hidden })),
    moveProperty: (s, w, id, direction) =>
      call(() => moveWorkspaceProperty(s, w, id, { direction })),
    deleteProperty: (s, w, id) => call(() => deleteWorkspaceProperty(s, w, id)),
    addValue: (s, w, list, name) =>
      call(async () => toValue(await addWorkspaceValue(s, w, list, { name }))),
    renameValue: (s, w, list, id, name) =>
      call(() => renameWorkspaceValue(s, w, list, id, { name })),
    setValueColor: (s, w, list, id, color) =>
      call(() =>
        setWorkspaceValueColor(s, w, list, id, { color: color as SetWorkspaceValueColorColor }),
      ),
    setStatusCategory: (s, w, id, category) =>
      call(() => setWorkspaceStatusCategory(s, w, id, { category })),
    moveValue: (s, w, list, id, direction) =>
      call(() => moveWorkspaceValue(s, w, list, id, { direction })),
    deleteValue: (s, w, list, id, replacement) =>
      call(() => deleteWorkspaceValue(s, w, list, id, replacement === "" ? {} : { replacement })),
    setInitialStatus: (s, w, value) => call(() => setWorkspaceInitialStatus(s, w, { value })),
    setInitialPriority: (s, w, value) => call(() => setWorkspaceInitialPriority(s, w, { value })),
    setCategoryDefault: (s, w, category, value) =>
      call(() => setWorkspaceCategoryDefault(s, w, category, { value })),
    setTransition: (s, w, from, to, allowed) =>
      call(() => setWorkspaceTransition(s, w, from, to, { allowed })),
    createTemplate: (s, w, input) =>
      call(async () => toTemplate(await createWorkspaceTemplate(s, w, toTemplateBody(input)))),
    updateTemplate: (s, w, id, input) =>
      call(() => updateWorkspaceTemplate(s, w, id, toTemplateBody(input))),
    deleteTemplate: (s, w, id) => call(() => deleteWorkspaceTemplate(s, w, id)),
    setTemplateDefault: (s, w, kind, template) =>
      call(() =>
        setWorkspaceTemplateDefault(
          s,
          w,
          kind as Parameters<typeof setWorkspaceTemplateDefault>[2],
          {
            template,
          },
        ),
      ),
    saveItemAsTemplate: (s, item, input) =>
      call(async () => toTemplate(await saveItemAsTemplate(s, item, input))),
    automation: (s, w) =>
      call(async () => {
        const v = await getWorkspaceAutomation(s, w);
        return {
          rules: (v.rules ?? []).map(toRule),
          recent: (v.recent ?? []).map((r) => ({ ...r, at: new Date(r.at) })),
        };
      }),
    createRule: (s, w, input) =>
      call(async () => toRule(await createAutomationRule(s, w, toRuleBody(input)))),
    updateRule: (s, w, id, input) => call(() => updateAutomationRule(s, w, id, toRuleBody(input))),
    deleteRule: (s, w, id) => call(() => deleteAutomationRule(s, w, id)),
    setRuleEnabled: (s, w, id, enabled) =>
      call(() => setAutomationRuleEnabled(s, w, id, { enabled })),
  };
}
