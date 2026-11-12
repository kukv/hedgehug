import { describe, expect, it } from "vitest";
import type { WorkspaceTemplate } from "../../model/workspace";
import { defaultDetail } from "./fixtures.test-util";
import { kindDefaults, templateRows, templateSummary } from "./templates";

const tpl = (
  id: string,
  name: string,
  kind: string,
  extra: Partial<WorkspaceTemplate> = {},
): WorkspaceTemplate => ({
  id,
  name,
  kind,
  title: "",
  description: "",
  values: {},
  children: [],
  steps: [],
  ...extra,
});
const step = (title: string) => ({ id: title, title, assignee: "@parent" });

describe("template rows", () => {
  it("counts children and all steps, and hides zero parts", () => {
    const epic = tpl("t1", "リリース作業", "Epic", {
      children: [
        {
          id: "c1",
          kind: "Story",
          title: "QA",
          description: "",
          values: {},
          steps: [step("a"), step("b")],
        },
        { id: "c2", kind: "Task", title: "本番", description: "", values: {}, steps: [] },
      ],
    });
    expect(templateSummary(epic)).toBe("子 2 件・Step 2 件");
    expect(templateSummary(tpl("t2", "調査", "Task", { steps: [step("a")] }))).toBe("Step 1 件");
    expect(templateSummary(tpl("t3", "Step", "Step"))).toBe("");
    const s = { ...defaultDetail("s1", "w1").settings, templates: [epic] };
    expect(templateRows(s)).toEqual([
      { id: "t1", name: "リリース作業", kind: "Epic", summary: "子 2 件・Step 2 件" },
    ]);
  });
});

describe("kind defaults", () => {
  it("offers なし and the templates of the kind", () => {
    const s = {
      ...defaultDetail("s1", "w1").settings,
      templates: [tpl("t1", "調査", "Task"), tpl("t2", "不具合", "Bug"), tpl("t3", "手順", "Task")],
      templateDefaults: { Task: "t3" },
    };
    const ds = kindDefaults(s);
    expect(ds.map((d) => d.kind)).toEqual(["Epic", "Story", "Task", "Bug", "Step"]);
    const task = ds[2];
    expect(task?.value).toBe("t3");
    expect(task?.options).toEqual([
      { value: "", label: "なし" },
      { value: "t1", label: "調査" },
      { value: "t3", label: "手順" },
    ]);
    expect(ds[0]?.value).toBe("");
  });
});
