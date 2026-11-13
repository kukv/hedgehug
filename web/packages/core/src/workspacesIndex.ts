// ワークスペースの設定・テンプレート・作成ダイアログの core の export。index.ts が全部を再公開する
// (index.ts は 300 行の上限なので、ここに分けた。A4b の Task 9)。

export {
  createWorkspaceDialog,
  prefixPreviewHelp,
  type WorkspaceDialog,
  type WorkspaceDialogState,
  type WorkspaceForm,
} from "./features/workspaces/createDialog";
export { prefixConfirm, prefixExamples, prefixHelp } from "./features/workspaces/general";
export {
  addableTypes,
  deletePropertyBody,
  keyHint,
  newProperty,
  type PropertyRow,
  propertiesHelp,
  propertyKinds,
  propertyRows,
  toggleKind,
  typeLabels,
} from "./features/workspaces/properties";
export {
  createSaveAsTemplate,
  type SaveAsTemplate,
  type SaveAsTemplateModel,
  saveAsTemplateDialog,
  saveAsTemplateLabel,
} from "./features/workspaces/saveAsTemplate";
export {
  createWorkspaceSettings,
  type SettingsState,
  type WorkspaceSettingsScreen,
  type WorkspaceSettingsTab,
  workspaceSettingsTabs,
} from "./features/workspaces/settings";
export {
  canMoveWorkspace,
  createWorkspaceMover,
  type WorkspaceMover,
} from "./features/workspaces/sidebarMenu";
export {
  type ApplyMode,
  type ApplyState,
  applyTemplate,
  defaultTemplate,
  type StructureRow,
  templateStructure,
  usableTemplates,
} from "./features/workspaces/templateApply";
export {
  type ChildForm,
  childKinds,
  emptyTemplateForm,
  formOfTemplate,
  kindChangeNote,
  type StepForm,
  type Structure,
  stepAssigneeOptions,
  structureOf,
  type TemplateForm,
  toTemplateInput,
  type ValueField,
  valueFields,
} from "./features/workspaces/templateForm";
export {
  deleteTemplateBody,
  deleteTemplateTitle,
  type KindDefault,
  kindDefaults,
  type TemplateRow,
  templateCreated,
  templateDeleted,
  templateRows,
  templateSummary,
  templatesEmpty,
} from "./features/workspaces/templates";
export {
  categoryLabels,
  colors,
  type DeleteDialogModel,
  deleteDialog,
  statusHelp,
  type ValueRow,
  valueListLabel,
  valueRows,
  valuesHelp,
} from "./features/workspaces/values";
export {
  transitionRefusal,
  type WorkflowCell,
  type WorkflowRow,
  workflowHelp,
  workflowRows,
} from "./features/workspaces/workflow";
