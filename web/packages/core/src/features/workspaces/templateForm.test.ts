import { describe, expect, it } from "vitest";
import type { WorkspaceProperty, WorkspaceTemplate } from "../../model/workspace";
import { defaultDetail } from "./fixtures.test-util";
import {
  emptyTemplateForm,
  formOfTemplate,
  kindChangeNote,
  stepAssigneeOptions,
  structureOf,
  type TemplateForm,
  toTemplateInput,
  valueFields,
} from "./templateForm";

const settings = defaultDetail("s1", "w1").settings;
const members = [{ login: "taro", name: "田中" }];

const custom = (id: string, type: WorkspaceProperty["type"], kinds: string[] = []) => ({
  id,
  key: id,
  name: id,
  type,
  builtin: false,
  kinds,
  hidden: false,
  choices: [{ id: `${id}-a`, name: "A", color: "gray" }],
});

describe("value fields", () => {
  // 既定の設定の並びは、ステータス・優先度・担当者・開始日・終了日・見積もり時間・関連システム・メモ。
  it("shows priority, assignee and systems with their empty choices", () => {
    const fields = valueFields(settings, "Task", members);
    expect(fields.map((f) => f.label)).toEqual(["優先度", "担当者", "関連システム"]);
    expect(fields[0]?.options[0]).toEqual({ value: "", label: "指定しない" });
    expect(fields[1]?.options[0]).toEqual({ value: "", label: "未割り当て" });
    expect(fields[1]?.options[1]).toEqual({ value: "taro", label: "田中" });
    expect(fields[2]?.input).toBe("multi");
  });

  it("hides hidden properties and those the kind does not use", () => {
    const s = {
      ...settings,
      properties: settings.properties.map((p) =>
        p.key === "priority"
          ? { ...p, hidden: true }
          : p.key === "assignee"
            ? { ...p, kinds: ["Epic"] }
            : p,
      ),
    };
    expect(valueFields(s, "Task", members).map((f) => f.label)).toEqual(["関連システム"]);
  });

  it("shows added properties by their type", () => {
    const s = {
      ...settings,
      properties: [
        ...settings.properties,
        custom("c", "check"),
        custom("u", "user"),
        custom("o", "single"),
        custom("m", "multi"),
        custom("n", "number"),
        custom("d", "date", ["Bug"]),
      ],
    };
    const fields = valueFields(s, "Task", members).slice(3);
    expect(fields.map((f) => f.input)).toEqual(["select", "select", "select", "multi", "number"]);
    expect(fields[0]?.options).toEqual([
      { value: "", label: "指定しない" },
      { value: "on", label: "オン" },
      { value: "off", label: "オフ" },
    ]);
    expect(fields[1]?.options).toEqual([
      { value: "", label: "指定しない" },
      { value: "taro", label: "田中" },
    ]);
    expect(fields[2]?.options).toEqual([
      { value: "", label: "指定しない" },
      { value: "o-a", label: "A" },
    ]);
    expect(fields[3]?.options).toEqual([{ value: "m-a", label: "A" }]);
  });
});

describe("step assignee options", () => {
  it("starts with 親と同じ and 未割り当て", () => {
    expect(stepAssigneeOptions(members)).toEqual([
      { value: "@parent", label: "親と同じ" },
      { value: "@none", label: "未割り当て" },
      { value: "taro", label: "田中" },
    ]);
  });
});

describe("structure", () => {
  it("gives children to Epic, steps to Story, Task and Bug, and nothing to Step", () => {
    expect(["Epic", "Story", "Task", "Bug", "Step"].map(structureOf)).toEqual([
      "children",
      "steps",
      "steps",
      "steps",
      null,
    ]);
  });
});

describe("kind change", () => {
  it("names what is not saved and omits zero parts", () => {
    const f = {
      ...emptyTemplateForm(),
      kind: "Epic",
      children: [
        {
          id: "",
          kind: "Story" as const,
          title: "QA",
          description: "",
          values: {},
          steps: [{ id: "", title: "a", assignee: "@parent" }],
        },
      ],
    };
    expect(kindChangeNote(f, "Task")).toBe("子 1 件・Step 1 件は、この種類では保存されません");
    expect(kindChangeNote(f, "Epic")).toBeNull();
    const t = { ...emptyTemplateForm(), steps: [{ id: "", title: "a", assignee: "@parent" }] };
    expect(kindChangeNote(t, "Step")).toBe("Step 1 件は、この種類では保存されません");
    expect(kindChangeNote(t, "Bug")).toBeNull();
  });
});

describe("input", () => {
  it("starts as a Task", () => {
    expect(emptyTemplateForm().kind).toBe("Task");
  });

  it("sends shown fields, null for empty ones, and drops empty rows", () => {
    const high = settings.priorities[1]?.id as string;
    const f = {
      ...emptyTemplateForm(),
      name: "調査",
      values: { priority: high, assignee: "", systems: [] },
      steps: [
        { id: "", title: "再現", assignee: "@none" },
        { id: "", title: " ", assignee: "@parent" },
      ],
    };
    const input = toTemplateInput(f, settings);
    expect(input.values).toEqual({ priority: high, assignee: null, systems: null });
    expect(input.steps).toEqual([{ id: "", title: "再現", assignee: "@none" }]);
    expect(input.children).toEqual([]);
  });

  it("sends values by type, only the structure the kind holds, and children with their own fields", () => {
    const s = {
      ...settings,
      properties: [...settings.properties, custom("c", "check"), custom("n", "number", ["Epic"])],
    };
    const f: TemplateForm = {
      ...emptyTemplateForm(),
      kind: "Epic",
      values: { c: "off", n: "3", memo: "x" },
      steps: [{ id: "", title: "捨てる", assignee: "@parent" }],
      children: [
        {
          id: "k1",
          kind: "Bug" as const,
          title: "QA",
          description: "d",
          values: { c: "on", n: "5" },
          steps: [],
        },
        { id: "", kind: "Task" as const, title: "  ", description: "", values: {}, steps: [] },
      ],
    };
    const input = toTemplateInput(f, s);
    expect(input.values).toEqual({ priority: null, assignee: null, systems: null, c: false, n: 3 });
    expect(input.steps).toEqual([]);
    expect(input.children).toEqual([
      {
        id: "k1",
        kind: "Bug",
        title: "QA",
        description: "d",
        values: { priority: null, assignee: null, systems: null, c: true },
        steps: [],
      },
    ]);
  });

  it("trims numbers and sends null for what is not a number", () => {
    const s = { ...settings, properties: [...settings.properties, custom("n", "number")] };
    const send = (n: string) =>
      toTemplateInput({ ...emptyTemplateForm(), values: { n } }, s).values.n;
    expect(send(" 3 ")).toBe(3);
    expect(send("1.5")).toBe(1.5);
    expect(send("")).toBeNull();
    expect(send("abc")).toBeNull();
  });

  it("reads a template back into the form", () => {
    const t: WorkspaceTemplate = {
      id: "t1",
      name: "調査",
      kind: "Epic",
      title: "[調査] ",
      description: "## 手順",
      values: { priority: "pr2", systems: ["sy1"], n: 3, c: true },
      children: [
        {
          id: "k1",
          kind: "Story",
          title: "QA",
          description: "",
          values: { c: false },
          steps: [{ id: "s1", title: "確認", assignee: "taro" }],
        },
      ],
      steps: [],
    };
    const f = formOfTemplate(t);
    expect(f).toEqual({
      id: "t1",
      name: "調査",
      kind: "Epic",
      title: "[調査] ",
      description: "## 手順",
      values: { priority: "pr2", systems: ["sy1"], n: "3", c: "on" },
      children: [
        {
          id: "k1",
          kind: "Story",
          title: "QA",
          description: "",
          values: { c: "off" },
          steps: [{ id: "s1", title: "確認", assignee: "taro" }],
        },
      ],
      steps: [],
    });
  });
});
