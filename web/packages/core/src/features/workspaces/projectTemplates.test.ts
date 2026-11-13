import { describe, expect, it } from "vitest";
import type { WorkspaceTemplate } from "../../model/workspace";
import { P, sample } from "./projectRules.fixtures.test-util";
import { projectTemplateView } from "./projectTemplates";

const tpl = (id: string, kind: string): WorkspaceTemplate =>
  ({
    id,
    name: `N${id}`,
    kind,
    title: "",
    description: "",
    values: {},
    steps: [],
    children: [],
  }) as unknown as WorkspaceTemplate;
const ws = { templates: [tpl("T1", "Task"), tpl("T2", "Task"), tpl("E1", "Epic")] };
const view = (r = {}, w = {}) => projectTemplateView(sample(r, { ...ws, ...w }), P);
const task = (v: ReturnType<typeof view>) => v.defaults.find((d) => d.kind === "Task");

describe("projectTemplateView", () => {
  it("lists every template unchecked, and says all are usable, when none is chosen", () => {
    const v = view();
    expect(v.uses.map((u) => [u.id, u.checked])).toEqual([
      ["T1", false],
      ["T2", false],
      ["E1", false],
    ]);
    expect(v.allUsable).toBe(true);
  });
  it("checks the chosen templates", () => {
    const v = view({ templates: ["T2"] });
    expect(v.uses.map((u) => u.checked)).toEqual([false, true, false]);
    expect(v.allUsable).toBe(false);
  });
  it("offers the workspace default's name, 'none' and the usable templates", () => {
    const d = task(view({}, { templateDefaults: { Task: "T1" } }));
    expect(d?.options).toEqual([
      { value: "", label: "ワークスペースと同じ(NT1)" },
      { value: "@none", label: "なし" },
      { value: "T1", label: "NT1" },
      { value: "T2", label: "NT2" },
    ]);
    expect(d?.value).toBe("");
  });
  it("says 'none' for the workspace default when it is not usable here", () => {
    const d = task(view({ templates: ["T2"] }, { templateDefaults: { Task: "T1" } }));
    expect(d?.options[0]?.label).toBe("ワークスペースと同じ(なし)");
    expect(d?.options.map((o) => o.value)).toEqual(["", "@none", "T2"]);
  });
  it("shows the project's own default, or 'none' when it is not usable", () => {
    expect(task(view({ templateDefaults: { Task: "T2" } }))?.value).toBe("T2");
    expect(task(view({ templateDefaults: { Task: "@none" } }))?.value).toBe("@none");
    expect(task(view({ templates: ["T1"], templateDefaults: { Task: "T2" } }))?.value).toBe(
      "@none",
    );
  });
  it("has the five kinds", () => {
    expect(view().defaults.map((d) => d.kind)).toEqual(["Epic", "Story", "Task", "Bug", "Step"]);
  });
});
