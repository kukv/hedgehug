import type {
  AutomationRule,
  AutomationRuleInput,
  Category,
  NewProperty,
  SaveAsTemplateInput,
  TemplateInput,
  ValueList,
  Workspace,
  WorkspaceAutomation,
  WorkspaceDetail,
  WorkspaceProperty,
  WorkspaceTemplate,
  WorkspaceUsage,
  WorkspaceValue,
} from "../model/workspace";

export type Direction = "up" | "down";

// ワークスペースと設定の口。実装は api パッケージにある。
export interface WorkspacesPort {
  get(storeId: string, workspaceId: string): Promise<WorkspaceDetail>;
  usage(storeId: string, workspaceId: string): Promise<WorkspaceUsage>;
  create(storeId: string, name: string, prefix: string): Promise<Workspace>;
  rename(storeId: string, workspaceId: string, name: string): Promise<void>;
  changePrefix(storeId: string, workspaceId: string, prefix: string): Promise<void>;
  move(storeId: string, workspaceId: string, direction: Direction): Promise<void>;
  addProperty(storeId: string, workspaceId: string, input: NewProperty): Promise<WorkspaceProperty>;
  renameProperty(
    storeId: string,
    workspaceId: string,
    propertyId: string,
    name: string,
  ): Promise<void>;
  setPropertyKinds(
    storeId: string,
    workspaceId: string,
    propertyId: string,
    kinds: string[],
  ): Promise<void>;
  setPropertyHidden(
    storeId: string,
    workspaceId: string,
    propertyId: string,
    hidden: boolean,
  ): Promise<void>;
  moveProperty(
    storeId: string,
    workspaceId: string,
    propertyId: string,
    direction: Direction,
  ): Promise<void>;
  deleteProperty(storeId: string, workspaceId: string, propertyId: string): Promise<void>;
  addValue(
    storeId: string,
    workspaceId: string,
    list: ValueList,
    name: string,
  ): Promise<WorkspaceValue>;
  renameValue(
    storeId: string,
    workspaceId: string,
    list: ValueList,
    valueId: string,
    name: string,
  ): Promise<void>;
  setValueColor(
    storeId: string,
    workspaceId: string,
    list: ValueList,
    valueId: string,
    color: string,
  ): Promise<void>;
  setStatusCategory(
    storeId: string,
    workspaceId: string,
    valueId: string,
    category: Category,
  ): Promise<void>;
  moveValue(
    storeId: string,
    workspaceId: string,
    list: ValueList,
    valueId: string,
    direction: Direction,
  ): Promise<void>;
  // replacement が空なら、ステータス・優先度は初期値、単一選択は「なし」。
  deleteValue(
    storeId: string,
    workspaceId: string,
    list: ValueList,
    valueId: string,
    replacement: string,
  ): Promise<void>;
  setInitialStatus(storeId: string, workspaceId: string, valueId: string): Promise<void>;
  setInitialPriority(storeId: string, workspaceId: string, valueId: string): Promise<void>;
  setCategoryDefault(
    storeId: string,
    workspaceId: string,
    category: Category,
    valueId: string,
  ): Promise<void>;
  setTransition(
    storeId: string,
    workspaceId: string,
    from: string,
    to: string,
    allowed: boolean,
  ): Promise<void>;
  createTemplate(
    storeId: string,
    workspaceId: string,
    input: TemplateInput,
  ): Promise<WorkspaceTemplate>;
  updateTemplate(
    storeId: string,
    workspaceId: string,
    templateId: string,
    input: TemplateInput,
  ): Promise<void>;
  deleteTemplate(storeId: string, workspaceId: string, templateId: string): Promise<void>;
  // templateId が空なら「なし」
  setTemplateDefault(
    storeId: string,
    workspaceId: string,
    kind: string,
    templateId: string,
  ): Promise<void>;
  saveItemAsTemplate(
    storeId: string,
    itemId: string,
    input: SaveAsTemplateInput,
  ): Promise<WorkspaceTemplate>;
  automation(storeId: string, workspaceId: string): Promise<WorkspaceAutomation>;
  createRule(
    storeId: string,
    workspaceId: string,
    input: AutomationRuleInput,
  ): Promise<AutomationRule>;
  updateRule(
    storeId: string,
    workspaceId: string,
    ruleId: string,
    input: AutomationRuleInput,
  ): Promise<void>;
  deleteRule(storeId: string, workspaceId: string, ruleId: string): Promise<void>;
  setRuleEnabled(
    storeId: string,
    workspaceId: string,
    ruleId: string,
    enabled: boolean,
  ): Promise<void>;
}
