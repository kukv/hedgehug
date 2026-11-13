import type { ProjectRules, WorkspaceSettings } from "../../model/workspace";
import { defaultDetail } from "./fixtures.test-util";

// プロジェクト別のルールのテスト用。st1 未着手(初期・既定)、st2 進行中(既定)、st3 レビュー中(既定でない)、
// st4 完了(既定)、st5 中止(既定)。
export const todo = "st1";
export const doing = "st2";
export const review = "st3";
export const done = "st4";
export const cancel = "st5";
export const P = "p1";

export const rules = (r: Partial<ProjectRules>): ProjectRules => ({
  hiddenStatuses: [],
  workflow: {},
  templates: [],
  templateDefaults: {},
  disabledRules: [],
  ...r,
});

export const sample = (
  r: Partial<ProjectRules> = {},
  ws: Partial<WorkspaceSettings> = {},
): WorkspaceSettings => ({
  ...defaultDetail("s1", "w1").settings,
  ...ws,
  projectRules: { [P]: rules(r) },
});
