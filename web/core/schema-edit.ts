import type { SchemaView } from "./schema";

/** The colors a status can take. value is the name in schema.yaml. */
export const STATUS_COLORS: { value: string; label: string; token: string }[] = [
  { value: "gray", label: "グレー", token: "var(--muted)" },
  { value: "blue", label: "ブルー", token: "var(--accent)" },
  { value: "green", label: "グリーン", token: "var(--success)" },
  { value: "amber", label: "アンバー", token: "var(--amber)" },
  { value: "red", label: "レッド", token: "var(--danger)" },
  { value: "teal", label: "ティール", token: "var(--store-teal-dot)" },
  { value: "orange", label: "オレンジ", token: "var(--store-orange-dot)" },
  { value: "purple", label: "パープル", token: "var(--store-purple-dot)" },
  { value: "sky", label: "スカイ", token: "var(--store-sky-dot)" },
  { value: "pink", label: "ピンク", token: "var(--store-pink-dot)" },
  { value: "olive", label: "オリーブ", token: "var(--store-olive-dot)" },
  { value: "brown", label: "ブラウン", token: "var(--store-brown-dot)" },
];

export const KIND_ICONS: { value: string; label: string }[] = [
  { value: "project", label: "プロジェクト" },
  { value: "epic", label: "エピック" },
  { value: "story", label: "ストーリー" },
  { value: "task", label: "タスク" },
  { value: "bug", label: "バグ" },
];

/** The color a status of the schema is given, if any. */
export const statusColorOf = (schema: SchemaView | undefined, name: string): string | undefined =>
  schema?.statuses?.find((s) => s.name === name)?.color;

/** The icon a kind of the schema is given, if any. */
export const kindIconOf = (schema: SchemaView | undefined, name: string): string | undefined =>
  schema?.kinds?.find((k) => k.name === name)?.icon;

/** The color a kind of the schema is given, if any. */
export const kindColorOf = (schema: SchemaView | undefined, name: string): string | undefined =>
  schema?.kinds?.find((k) => k.name === name)?.color;

/** The role of a kind of the schema: work or container. */
export const kindRoleOf = (schema: SchemaView | undefined, name: string): string | undefined =>
  schema?.kinds?.find((k) => k.name === name)?.role;

/** The props of the schema in the order of schema.yaml (by name when the order is not given). */
export function propNames(schema: SchemaView): string[] {
  const ordered = [...new Set(schema.prop_order ?? [])].filter((n) => n in schema.props);
  const rest = Object.keys(schema.props)
    .filter((n) => !ordered.includes(n))
    .sort();
  return [...ordered, ...rest];
}

/** The color to draw: a token for a palette name, the value for another plain color, undefined otherwise. */
export function colorToken(color: string | undefined): string | undefined {
  if (!color) return undefined;
  const named = STATUS_COLORS.find((c) => c.value === color);
  if (named) return named.token;
  return /^(#[0-9a-f]{3,8}|[a-z]+|var\(--[a-z0-9-]+\))$/i.test(color) ? color : undefined;
}

type Lists = Record<string, string[] | null> | undefined;
type Workflow = { transitions?: Lists; required?: Lists } | undefined;

/** One place a removal takes the definition out of. project is the ULID of an override's project. */
export type Reference = { text: string; project?: string };

function workflowRefs(
  wf: Workflow,
  target: "status" | "prop",
  name: string,
  prefix: string,
): string[] {
  const out: string[] = [];
  if (target === "status") {
    for (const [from, tos] of Object.entries(wf?.transitions ?? {})) {
      if (from !== name && tos?.includes(name)) out.push(`${prefix}${from} から移せる先`);
    }
    if (wf?.transitions && name in wf.transitions) out.push(`${prefix}${name} からの遷移の制約`);
    if (wf?.required && name in wf.required) out.push(`${prefix}${name} に入るときの必須の項目`);
  } else {
    for (const [status, fields] of Object.entries(wf?.required ?? {})) {
      if (fields?.includes(name)) out.push(`${prefix}${status} に入るときの必須の項目`);
    }
  }
  return out;
}

/** Where a removal takes the definition out of, one line each, in Japanese. */
export function referencesOf(
  schema: SchemaView,
  target: "status" | "kind" | "prop" | "value",
  name: string,
): Reference[] {
  const out: Reference[] = [];
  if (target === "status" || target === "prop") {
    for (const text of workflowRefs(schema.workflow, target, name, "ワークフロー: "))
      out.push({ text });
  }
  if (target === "kind") {
    for (const k of schema.kinds ?? []) {
      if (k.parents?.includes(name)) out.push({ text: `種類 ${k.name} の置ける親` });
    }
  }
  for (const s of schema.scoped ?? []) {
    const project = s.where.replace(/^project:/, "");
    const add = (text: string) => out.push({ text: `上書き: ${text}`, project });
    if (target === "status") {
      if (s.statuses?.includes(name)) add("表示するステータス");
      for (const text of workflowRefs(s.workflow, "status", name, "ワークフロー: ")) add(text);
    }
    if (target === "prop") {
      for (const text of workflowRefs(s.workflow, "prop", name, "ワークフロー: ")) add(text);
    }
    if (target === "kind" && s.templates?.default && name in s.templates.default)
      add("既定のテンプレート");
  }
  return out;
}

/** Kinds whose only parent is name (a removal of name is refused). */
export function orphanedBy(schema: SchemaView, kind: string): string[] {
  return (schema.kinds ?? [])
    .filter(
      (k) =>
        k.name !== kind && k.parents && k.parents.length > 0 && k.parents.every((p) => p === kind),
    )
    .map((k) => k.name);
}
