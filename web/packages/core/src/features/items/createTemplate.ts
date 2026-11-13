import type { Kind, PropertyInput } from "../../model/item";
import type { Member } from "../../model/member";
import type { WorkspaceSettings } from "../../model/workspace";
import type { Option } from "../projects/createDialog";
import { usesStatus } from "../workspaces/projectRules";
import {
  type ApplyMode,
  applyTemplate,
  defaultTemplate,
  nextDescription,
  type StructureRow,
  templateStructure,
  usableTemplates,
} from "../workspaces/templateApply";
import type { Values } from "./createValues";
import { visibleProperties } from "./properties";

// 作成ダイアログのテンプレート(02「ダイアログの中での連動」、03「適用するときの規則」)。説明の当てはめは
// A4a-6。使えるテンプレートは、project があればそのプロジェクトの絞り込みを重ねて決める(B5a-9)。

export interface TemplateForm {
  kind: Kind;
  title: string;
  values: Values;
  // templateId は、当てはめているテンプレート。空は「なし」。
  templateId: string;
  // description は説明の欄。templateDescription は、いま当てはめているテンプレートの説明(当てはめていなければ "")。
  // 説明が "" かこれと一字一句同じなら、テンプレート由来とみなす(02「開き方と初期値」)。ワークスペースを変えると
  // 前のテンプレートは新しい設定にないので、設定から引かず、フォームが持つ。
  description: string;
  templateDescription: string;
}

const find = (settings: WorkspaceSettings, id: string) =>
  settings.templates.find((t) => t.id === id) ?? null;

// templateOptions は、テンプレートのセレクトの選択肢。使えるものがなければ空(欄を出さない)。
export function templateOptions(
  settings: WorkspaceSettings,
  kind: Kind,
  project?: string,
): Option[] {
  const list = usableTemplates(settings, kind, project);
  if (list.length === 0) return [];
  return [{ value: "", label: "なし" }, ...list.map((t) => ({ value: t.id, label: t.name }))];
}

// toInput は、テンプレートの値を入力欄の値にする。数は文字列にする(A3a の決定 12、A3b-13)。
const toInput = (v: unknown): PropertyInput =>
  typeof v === "number" ? String(v) : (v as PropertyInput);

// applyToForm は、templateId のテンプレートを当てはめた入力。空なら「なし」にする(入力は残す)。
// mode は、説明の扱い: "select" はセレクトで選んだとき(置き換える)、"auto" は開いたとき・種類やワークスペースを
// 変えたとき(空かテンプレート由来のときだけ入れる)。
export function applyToForm<F extends TemplateForm>(
  form: F,
  settings: WorkspaceSettings,
  templateId: string,
  mode: ApplyMode = "select",
): F {
  const shown = new Set(visibleProperties(settings, form.kind).map((p) => p.id));
  const next = find(settings, templateId);
  const out = applyTemplate(
    { title: form.title, description: "", values: {}, template: null },
    next,
    mode,
    (id) => shown.has(id),
  );
  const values = { ...form.values };
  for (const [id, v] of Object.entries(out.values)) values[id] = toInput(v);
  return {
    ...form,
    title: out.title,
    values,
    templateId: next?.id ?? "",
    description: nextDescription(form.description, form.templateDescription, next, mode),
    templateDescription: next?.description ?? "",
  };
}

// applyDefault は、その種類の既定のテンプレートを当てはめる(A3b-14。説明は自動の扱い。A4a-6)。
export function applyDefault<F extends TemplateForm>(
  form: F,
  settings: WorkspaceSettings,
  project?: string,
): F {
  return applyToForm(
    form,
    settings,
    defaultTemplate(settings, form.kind, project)?.id ?? "",
    "auto",
  );
}

// narrowToProject は、プロジェクトを変えたあとの入力(B5a-21、02「プロジェクトを変えたとき」)。新しいプロジェクトで
// 使わないステータスは初期ステータスに戻す。使えないテンプレートは「なし」に戻し、そのプロジェクトのいまの種類の
// 既定を当てはめる(説明は自動の扱い: テンプレート由来なら入れ替え、利用者が書いたものは残す)。使えるものはそのまま。
export function narrowToProject<F extends TemplateForm & { projectId: string }>(
  form: F,
  settings: WorkspaceSettings,
): F {
  let out = form;
  const status = out.values.status;
  if (typeof status === "string" && !usesStatus(settings, out.projectId, status)) {
    out = { ...out, values: { ...out.values, status: settings.initialStatus } };
  }
  const usable = usableTemplates(settings, out.kind, out.projectId);
  if (out.templateId !== "" && !usable.some((t) => t.id === out.templateId)) {
    out = applyDefault(out, settings, out.projectId);
  }
  return out;
}

// structureOf は、「一緒に作るもの」。「親と同じ」の担当者には、いまの入力の担当者を出す(A3b-15)。
export function structureOf(
  settings: WorkspaceSettings,
  form: TemplateForm,
  members: Member[],
): { heading: string; rows: StructureRow[] } | null {
  const t = find(settings, form.templateId);
  if (!t) return null;
  const assignee = typeof form.values.assignee === "string" ? form.values.assignee : "";
  return templateStructure(t, assignee, members);
}

// othersOf は、一緒に作る子と Step の数(トーストの「ほか N 件」。A3b-16)。
export function othersOf(settings: WorkspaceSettings, templateId: string): number {
  const t = find(settings, templateId);
  if (!t) return 0;
  return t.children.reduce((n, c) => n + 1 + c.steps.length, t.steps.length);
}
