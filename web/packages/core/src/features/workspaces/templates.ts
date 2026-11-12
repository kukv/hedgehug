import type { TemplateInput, WorkspaceSettings } from "../../model/workspace";
import { propertyKinds } from "./properties";

// テンプレートのタブ(03「テンプレート」)。

export const templatesEmpty = "テンプレートはありません";
export const templateCreated = "テンプレートを作成しました";
export const templateDeleted = "テンプレートを削除しました";
export const deleteTemplateTitle = "テンプレートを削除";
export const deleteTemplateBody = (name: string) =>
  `「${name}」を削除します。作ったアイテムには影響しません。`;

// 構成の件数「子 3 件・Step 8 件」。ないものは出さない。どちらもなければ ""。
export function templateSummary(t: Pick<TemplateInput, "children" | "steps">): string {
  const steps = t.steps.length + t.children.reduce((n, c) => n + c.steps.length, 0);
  const parts: string[] = [];
  if (t.children.length > 0) parts.push(`子 ${t.children.length} 件`);
  if (steps > 0) parts.push(`Step ${steps} 件`);
  return parts.join("・");
}

export interface TemplateRow {
  id: string;
  name: string;
  kind: string;
  summary: string;
}

export function templateRows(settings: WorkspaceSettings): TemplateRow[] {
  return settings.templates.map((t) => ({
    id: t.id,
    name: t.name,
    kind: t.kind,
    summary: templateSummary(t),
  }));
}

export interface KindDefault {
  kind: string;
  value: string;
  options: { value: string; label: string }[];
}

// 5 つの種類の既定のセレクト。候補は「なし」とその種類のテンプレート。
export function kindDefaults(settings: WorkspaceSettings): KindDefault[] {
  return propertyKinds.map((kind) => ({
    kind,
    value: settings.templateDefaults[kind] ?? "",
    options: [
      { value: "", label: "なし" },
      ...settings.templates
        .filter((t) => t.kind === kind)
        .map((t) => ({ value: t.id, label: t.name })),
    ],
  }));
}
