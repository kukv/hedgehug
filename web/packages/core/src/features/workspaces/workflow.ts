import type { Category, WorkspaceSettings } from "../../model/workspace";
import { allowedIn, projectTargets, rulesOf, usesIn, workspaceAllowed } from "./projectRules";

// ワークフローのタブ(03「遷移の制約」)。

export const workflowHelp =
  "行に表示されているステータスから、チェックしたステータスにだけ遷移できるよう制御することができます。";

export interface WorkflowCell {
  to: string;
  name: string;
  self: boolean;
  checked: boolean;
}

export interface WorkflowRow {
  from: string;
  name: string;
  color: string;
  category: Category;
  cells: WorkflowCell[];
  note: "どこへでも" | "制限あり";
}

export function workflowRows(settings: WorkspaceSettings): WorkflowRow[] {
  return settings.statuses.map((v) => {
    const ok = workspaceAllowed(settings, v.id);
    return {
      from: v.id,
      name: v.name,
      color: v.color,
      category: v.category ?? "todo",
      cells: settings.statuses.map((t) => ({
        to: t.id,
        name: t.name,
        self: t.id === v.id,
        checked: ok.includes(t.id),
      })),
      note: ok.length > 0 ? "制限あり" : "どこへでも",
    };
  });
}

export const sameAsWorkspace = "ワークスペースと同じ";
const forbidden = "ワークスペースの制約で変更できません";

export interface ProjectWorkflowCell extends WorkflowCell {
  // ワークスペースの制約が許さないセルは、チェックなしで無効にし、理由を添える
  disabled?: boolean;
  reason?: string;
}

export interface ProjectWorkflowRow extends Omit<WorkflowRow, "cells" | "note"> {
  cells: ProjectWorkflowCell[];
  note: typeof sameAsWorkspace | "制限あり";
  // 両方が許す To がないときの添え書き
  blocked?: string;
}

// projectWorkflowRows は、プロジェクトの「ワークフローの制約」の表(03「プロジェクトの遷移の制約」)。
// 使うステータスだけを並べ、チェックはプロジェクトのもの。
export function projectWorkflowRows(
  settings: WorkspaceSettings,
  projectId: string,
): ProjectWorkflowRow[] {
  const r = rulesOf(settings, projectId);
  const used = settings.statuses.filter((v) => usesIn(settings, r, v.id));
  return used.map((v) => {
    const ws = workspaceAllowed(settings, v.id);
    const checks = projectTargets(settings, r, v.id);
    const { to, restricted } = allowedIn(settings, r, v.id);
    return {
      from: v.id,
      name: v.name,
      color: v.color,
      category: v.category ?? "todo",
      cells: used.map((t): ProjectWorkflowCell => {
        const blocked = ws.length > 0 && t.id !== v.id && !ws.includes(t.id);
        return {
          to: t.id,
          name: t.name,
          self: t.id === v.id,
          checked: checks.includes(t.id),
          ...(blocked ? { disabled: true, reason: forbidden } : {}),
        };
      }),
      note: checks.length > 0 ? "制限あり" : sameAsWorkspace,
      ...(restricted && to.length === 0 ? { blocked: "変更できる先がありません" } : {}),
    };
  });
}

// Go の CheckTransition と同じ判定と文言。通るなら null。project があれば、そのプロジェクトのルールも
// 重ねる(B5a-6)。
export function transitionRefusal(
  settings: WorkspaceSettings,
  from: string,
  to: string,
  project?: string,
): string | null {
  // いまのステータスが一覧にない(削除された値のまま)ときは検査しない(02「ステータスの変更」、A3b-2)。
  if (from === "" || from === to || !settings.statuses.some((v) => v.id === from)) return null;
  const rules = project === undefined ? undefined : rulesOf(settings, project);
  const { to: ok, restricted } = rules
    ? allowedIn(settings, rules, from)
    : {
        to: workspaceAllowed(settings, from),
        restricted: workspaceAllowed(settings, from).length > 0,
      };
  if (!restricted || ok.includes(to)) return null;
  const name = (id: string) => settings.statuses.find((v) => v.id === id)?.name ?? "";
  const tail = ok.length > 0 ? `${ok.map(name).join("・")} のみ` : "変更できる先がありません";
  return `ワークフローの制約: ${name(from)} から ${name(to)} には変更できません(${tail})`;
}
