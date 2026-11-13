import type { Member } from "../../model/member";
import type { WorkspaceSettings, WorkspaceTemplate } from "../../model/workspace";
import { stepNone, stepParent } from "../../model/workspace";
import { templateSummary } from "./templates";

// 作成ダイアログでのテンプレートの適用(03「適用するときの規則」、02「ダイアログの中での連動」)。
// 作成ダイアログへのはめ込みは A3b(B3-18)。

export function usableTemplates(settings: WorkspaceSettings, kind: string): WorkspaceTemplate[] {
  return settings.templates.filter((t) => t.kind === kind);
}

export function defaultTemplate(
  settings: WorkspaceSettings,
  kind: string,
): WorkspaceTemplate | null {
  const id = settings.templateDefaults[kind];
  return usableTemplates(settings, kind).find((t) => t.id === id) ?? null;
}

export interface ApplyState {
  title: string;
  description: string;
  values: Record<string, unknown>;
  template: WorkspaceTemplate | null;
}

export type ApplyMode = "auto" | "select";

// nextDescription は、テンプレートを当てはめたあとの説明。applied は、いま当てはめているテンプレートの説明
// (当てはめていなければ "")。説明が空か applied と一字一句同じなら、テンプレート由来とみなす。
export function nextDescription(
  current: string,
  applied: string,
  next: WorkspaceTemplate | null,
  mode: ApplyMode,
): string {
  const derived = current === "" || current === applied;
  if (next === null) return derived ? "" : current;
  return mode === "select" || derived ? next.description : current;
}

export function applyTemplate(
  state: ApplyState,
  next: WorkspaceTemplate | null,
  mode: ApplyMode,
  visible: (propertyId: string) => boolean,
): ApplyState {
  const values = { ...state.values };
  for (const [id, v] of Object.entries(next?.values ?? {})) {
    if (visible(id)) values[id] = v;
  }
  return {
    title: state.title.trim() === "" && next !== null ? next.title : state.title,
    description: nextDescription(state.description, state.template?.description ?? "", next, mode),
    values,
    template: next,
  };
}

export interface StructureRow {
  depth: 0 | 1;
  kind: string;
  title: string;
  assignee: string;
}

export function templateStructure(
  t: WorkspaceTemplate,
  parentAssignee: string,
  members: Member[],
): { heading: string; rows: StructureRow[] } | null {
  const summary = templateSummary(t);
  if (summary === "") return null;
  const name = (login: string) =>
    login === "" ? "未割り当て" : (members.find((m) => m.login === login)?.name ?? login);
  const stepRows = (
    steps: WorkspaceTemplate["steps"],
    parent: string,
    depth: 0 | 1,
  ): StructureRow[] =>
    steps.map((s) => ({
      depth,
      kind: "Step",
      title: s.title,
      assignee: name(
        s.assignee === stepParent ? parent : s.assignee === stepNone ? "" : s.assignee,
      ),
    }));
  const rows: StructureRow[] = [];
  for (const c of t.children) {
    const a = typeof c.values.assignee === "string" ? c.values.assignee : "";
    rows.push({ depth: 0, kind: c.kind, title: c.title, assignee: name(a) });
    rows.push(...stepRows(c.steps, a, 1));
  }
  rows.push(...stepRows(t.steps, parentAssignee, 0));
  return { heading: `一緒に作るもの: ${summary}`, rows };
}
