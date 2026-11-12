import type { Member } from "../../model/member";
import {
  type ChildKind,
  stepNone,
  stepParent,
  type TemplateChild,
  type TemplateInput,
  type TemplateStep,
  type WorkspaceProperty,
  type WorkspaceSettings,
  type WorkspaceTemplate,
} from "../../model/workspace";

// テンプレートの編集ダイアログ(03「一覧と編集」)。

export interface StepForm {
  id: string;
  title: string;
  assignee: string;
}

export interface ChildForm {
  id: string;
  kind: ChildKind;
  title: string;
  description: string;
  values: Record<string, string | string[]>;
  steps: StepForm[];
}

// 値の欄の入力は文字(選択式は ID、数は文字、チェックは "on"・"off"、複数は配列)。空は "" か []。
export interface TemplateForm {
  id: string; // 新規は ""
  name: string;
  kind: string;
  title: string;
  description: string;
  values: Record<string, string | string[]>;
  children: ChildForm[];
  steps: StepForm[];
}

type Option = { value: string; label: string };

export interface ValueField {
  propertyId: string;
  label: string;
  input: "select" | "multi" | "text" | "number" | "date" | "url";
  options: Option[]; // select・multi のとき
}

// 子に選べる種類(03「持てるもの」)。
export const childKinds: ChildKind[] = ["Story", "Task", "Bug"];

export function emptyTemplateForm(): TemplateForm {
  return {
    id: "",
    name: "",
    kind: "Task",
    title: "",
    description: "",
    values: {},
    children: [],
    steps: [],
  };
}

function formValue(v: unknown): string | string[] {
  if (Array.isArray(v)) return v.map(String);
  if (typeof v === "boolean") return v ? "on" : "off";
  if (v === null || v === undefined) return "";
  return String(v);
}

function formValues(vs: Record<string, unknown>): Record<string, string | string[]> {
  return Object.fromEntries(Object.entries(vs).map(([k, v]) => [k, formValue(v)]));
}

const formSteps = (ss: TemplateStep[]): StepForm[] => ss.map((s) => ({ ...s }));

export function formOfTemplate(t: WorkspaceTemplate): TemplateForm {
  return {
    id: t.id,
    name: t.name,
    kind: t.kind,
    title: t.title,
    description: t.description,
    values: formValues(t.values),
    children: t.children.map((c) => ({
      id: c.id,
      kind: c.kind,
      title: c.title,
      description: c.description,
      values: formValues(c.values),
      steps: formSteps(c.steps),
    })),
    steps: formSteps(t.steps),
  };
}

// holdsValue は、テンプレートに値を持てるプロパティか(Go の HoldsTemplateValue と同じ)。
function holdsValue(p: WorkspaceProperty): boolean {
  return !p.builtin || p.key === "priority" || p.key === "assignee" || p.key === "systems";
}

// 使う種類が空なら、すべての種類で使う(properties の表示と同じ)。
const usedBy = (p: WorkspaceProperty, kind: string) =>
  p.kinds.length === 0 || p.kinds.includes(kind);

const noChoice: Option = { value: "", label: "指定しない" };
const toOptions = (vs: { id: string; name: string }[]): Option[] =>
  vs.map((v) => ({ value: v.id, label: v.name }));
const memberOptions = (members: Member[]): Option[] =>
  members.map((m) => ({ value: m.login, label: m.name }));

function field(p: WorkspaceProperty, s: WorkspaceSettings, members: Member[]): ValueField {
  const f = (input: ValueField["input"], options: Option[] = []): ValueField => ({
    propertyId: p.id,
    label: p.name,
    input,
    options,
  });
  if (p.builtin && p.key === "priority") return f("select", [noChoice, ...toOptions(s.priorities)]);
  if (p.builtin && p.key === "assignee")
    return f("select", [{ value: "", label: "未割り当て" }, ...memberOptions(members)]);
  if (p.builtin && p.key === "systems") return f("multi", toOptions(s.systems));
  switch (p.type) {
    case "user":
      return f("select", [noChoice, ...memberOptions(members)]);
    case "single":
      return f("select", [noChoice, ...toOptions(p.choices)]);
    case "multi":
      return f("multi", toOptions(p.choices));
    case "check":
      return f("select", [
        noChoice,
        { value: "on", label: "オン" },
        { value: "off", label: "オフ" },
      ]);
    case "number":
      return f("number");
    case "date":
      return f("date");
    case "url":
      return f("url");
    default:
      return f("text");
  }
}

// valueFields は、その種類で使い、表示中で、テンプレートに持てるプロパティの欄(設定の並び)。
export function valueFields(
  settings: WorkspaceSettings,
  kind: string,
  members: Member[],
): ValueField[] {
  return settings.properties
    .filter((p) => holdsValue(p) && usedBy(p, kind) && !p.hidden)
    .map((p) => field(p, settings, members));
}

// Step の担当者の選択肢。親と同じ・未割り当て・メンバー。
export function stepAssigneeOptions(members: Member[]): Option[] {
  return [
    { value: stepParent, label: "親と同じ" },
    { value: stepNone, label: "未割り当て" },
    ...memberOptions(members),
  ];
}

export type Structure = "children" | "steps" | null;

// structureOf は、種類が持てる構成。Epic は子、Story・Task・Bug は Step、Step はなし。
export function structureOf(kind: string): Structure {
  if (kind === "Epic") return "children";
  if (kind === "Story" || kind === "Task" || kind === "Bug") return "steps";
  return null;
}

// kindChangeNote は、いまの構成を kind で保存したら捨てるものの文(B3-14)。捨てるものがなければ null。
export function kindChangeNote(form: TemplateForm, kind: string): string | null {
  const structure = structureOf(kind);
  let children = 0;
  let steps = 0;
  if (structure !== "children") {
    children = form.children.length;
    steps += form.children.reduce((n, c) => n + c.steps.length, 0);
  }
  if (structure !== "steps") steps += form.steps.length;
  const parts: string[] = [];
  if (children > 0) parts.push(`子 ${children} 件`);
  if (steps > 0) parts.push(`Step ${steps} 件`);
  return parts.length === 0 ? null : `${parts.join("・")}は、この種類では保存されません`;
}

function inputValue(
  settings: WorkspaceSettings,
  f: ValueField,
  v: string | string[] | undefined,
): unknown {
  if (v === undefined || v === "" || (Array.isArray(v) && v.length === 0)) return null;
  if (Array.isArray(v)) return v;
  if (f.input === "number") {
    // 前後の空白を除いて数にする。数でなければ値なし。
    const t = v.trim();
    const n = Number(t);
    return t === "" || !Number.isFinite(n) ? null : n;
  }
  const check = settings.properties.find((p) => p.id === f.propertyId)?.type === "check";
  return check ? v === "on" : v;
}

// inputValues は、kind の種類で画面に出す欄だけを送る形にする。空は null。
function inputValues(
  settings: WorkspaceSettings,
  kind: string,
  values: Record<string, string | string[]>,
): Record<string, unknown> {
  return Object.fromEntries(
    valueFields(settings, kind, []).map((f) => [
      f.propertyId,
      inputValue(settings, f, values[f.propertyId]),
    ]),
  );
}

const keep = (title: string) => title.trim() !== "";
const inputSteps = (ss: StepForm[]): TemplateStep[] =>
  ss.filter((s) => keep(s.title)).map((s) => ({ ...s }));

// toTemplateInput は、送る形。画面に出した欄だけを値として送り、空は null。ほかの値は送らない(B3-9)。
export function toTemplateInput(form: TemplateForm, settings: WorkspaceSettings): TemplateInput {
  const structure = structureOf(form.kind);
  const children: TemplateChild[] =
    structure === "children"
      ? form.children
          .filter((c) => keep(c.title))
          .map((c) => ({
            id: c.id,
            kind: c.kind,
            title: c.title,
            description: c.description,
            values: inputValues(settings, c.kind, c.values),
            steps: inputSteps(c.steps),
          }))
      : [];
  return {
    name: form.name,
    kind: form.kind,
    title: form.title,
    description: form.description,
    values: inputValues(settings, form.kind, form.values),
    children,
    steps: structure === "steps" ? inputSteps(form.steps) : [],
  };
}
