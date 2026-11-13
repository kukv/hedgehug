import type { Category, WorkspaceSettings } from "../../model/workspace";

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

// allowed は、from から変えられる To(ステータスの並び。いまあるものだけ)。空なら制限なし。
function allowed(settings: WorkspaceSettings, from: string): string[] {
  const set = settings.workflow[from] ?? [];
  return settings.statuses.map((v) => v.id).filter((id) => id !== from && set.includes(id));
}

export function workflowRows(settings: WorkspaceSettings): WorkflowRow[] {
  return settings.statuses.map((v) => {
    const ok = allowed(settings, v.id);
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

// Go の CheckTransition と同じ判定と文言。通るなら null。
export function transitionRefusal(
  settings: WorkspaceSettings,
  from: string,
  to: string,
): string | null {
  // いまのステータスが一覧にない(削除された値のまま)ときは検査しない(02「ステータスの変更」、A3b-2)。
  if (from === "" || from === to || !settings.statuses.some((v) => v.id === from)) return null;
  const ok = allowed(settings, from);
  if (ok.length === 0 || ok.includes(to)) return null;
  const name = (id: string) => settings.statuses.find((v) => v.id === id)?.name ?? "";
  return `ワークフローの制約: ${name(from)} から ${name(to)} には変更できません(${ok.map(name).join("・")} のみ)`;
}
