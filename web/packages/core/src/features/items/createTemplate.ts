import type { Kind, PropertyInput } from "../../model/item";
import type { Member } from "../../model/member";
import type { WorkspaceSettings } from "../../model/workspace";
import type { Option } from "../projects/createDialog";
import {
  applyTemplate,
  defaultTemplate,
  type StructureRow,
  templateStructure,
  usableTemplates,
} from "../workspaces/templateApply";
import type { Values } from "./createValues";
import { visibleProperties } from "./properties";

// 作成ダイアログのテンプレート(02「ダイアログの中での連動」、03「適用するときの規則」)。説明の当てはめは
// A4(A3a の決定 6)。使えるテンプレートはワークスペースの設定だけで決める(プロジェクトの絞り込みは B5)。

export interface TemplateForm {
  kind: Kind;
  title: string;
  values: Values;
  // templateId は、当てはめているテンプレート。空は「なし」。
  templateId: string;
}

const find = (settings: WorkspaceSettings, id: string) =>
  settings.templates.find((t) => t.id === id) ?? null;

// templateOptions は、テンプレートのセレクトの選択肢。使えるものがなければ空(欄を出さない)。
export function templateOptions(settings: WorkspaceSettings, kind: Kind): Option[] {
  const list = usableTemplates(settings, kind);
  if (list.length === 0) return [];
  return [{ value: "", label: "なし" }, ...list.map((t) => ({ value: t.id, label: t.name }))];
}

// toInput は、テンプレートの値を入力欄の値にする。数は文字列にする(A3a の決定 12、A3b-13)。
const toInput = (v: unknown): PropertyInput =>
  typeof v === "number" ? String(v) : (v as PropertyInput);

// applyToForm は、templateId のテンプレートを当てはめた入力。空なら「なし」にする(入力は残す)。
export function applyToForm<F extends TemplateForm>(
  form: F,
  settings: WorkspaceSettings,
  templateId: string,
): F {
  const shown = new Set(visibleProperties(settings, form.kind).map((p) => p.id));
  const out = applyTemplate(
    { title: form.title, description: "", values: {}, template: find(settings, form.templateId) },
    find(settings, templateId),
    "select",
    (id) => shown.has(id),
  );
  const values = { ...form.values };
  for (const [id, v] of Object.entries(out.values)) values[id] = toInput(v);
  return { ...form, title: out.title, values, templateId: out.template?.id ?? "" };
}

// applyDefault は、いまのテンプレートを「なし」にしてから、その種類の既定を当てはめる(A3b-14)。
export function applyDefault<F extends TemplateForm>(form: F, settings: WorkspaceSettings): F {
  const none = { ...form, templateId: "" };
  return applyToForm(none, settings, defaultTemplate(settings, form.kind)?.id ?? "");
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
