import { describe, expect, it } from "vitest";
import type { ProjectRules, WorkspaceSettings, WorkspaceTemplate } from "../../model/workspace";
import { rulesHeading, rulesProjects, templateDeleteLines } from "./projectRules";
import { P, review, rules, sample } from "./projectRules.fixtures.test-util";
import { defaultTemplate, usableTemplates } from "./templateApply";

const tpl = (id: string, kind: string): WorkspaceTemplate =>
  ({
    id,
    name: id,
    kind,
    title: "",
    description: "",
    values: {},
    steps: [],
    children: [],
  }) as unknown as WorkspaceTemplate;

describe("usable and default templates", () => {
  const base = (r: Partial<ProjectRules>, td: Record<string, string> = { Task: "T2" }) =>
    sample(r, {
      templates: [tpl("T1", "Task"), tpl("T2", "Task"), tpl("T3", "Bug")],
      templateDefaults: td,
    });
  const ids = (s: WorkspaceSettings) => usableTemplates(s, "Task", P).map((t) => t.id);

  it("follows the workspace without a project or rules", () => {
    expect(usableTemplates(base({}), "Task").map((t) => t.id)).toEqual(["T1", "T2"]);
    expect(ids(base({}))).toEqual(["T1", "T2"]);
    expect(defaultTemplate(base({}), "Task", P)?.id).toBe("T2");
  });

  it("narrows to the project templates and drops an unusable workspace default", () => {
    const s = base({ templates: ["T1"] });
    expect(ids(s)).toEqual(["T1"]);
    expect(defaultTemplate(s, "Task", P)).toBeNull();
  });

  it("uses the project default when usable", () => {
    const s = base({ templates: ["T1"], templateDefaults: { Task: "T1" } });
    expect(defaultTemplate(s, "Task", P)?.id).toBe("T1");
  });

  it("gives none for an unusable own default, without falling back", () => {
    expect(defaultTemplate(base({ templateDefaults: { Task: "T3" } }), "Task", P)).toBeNull();
  });

  it("gives none for @none", () => {
    expect(defaultTemplate(base({ templateDefaults: { Task: "@none" } }), "Task", P)).toBeNull();
  });

  it("uses all when only deleted templates are listed", () => {
    expect(ids(base({ templates: ["Gone"] }))).toEqual(["T1", "T2"]);
  });
});

describe("template delete lines", () => {
  const projects = [
    { id: "pa", display: "HH-P1", title: "A" },
    { id: "pb", display: "HH-P2", title: "B" },
    { id: "pc", display: "HH-P3", title: "C" },
    { id: "pe", display: "HH-P5", title: "E" },
  ];
  const s = (set: Record<string, Partial<ProjectRules>>): WorkspaceSettings => ({
    ...sample(),
    templates: [tpl("T1", "Task"), tpl("T2", "Task")],
    projectRules: Object.fromEntries(Object.entries(set).map(([k, v]) => [k, rules(v)])),
  });

  it("counts projects and lists those that become unlimited", () => {
    const set = s({
      pc: { templates: ["T1", "T2"] },
      pb: { templates: ["T1"] },
      pa: { templateDefaults: { Task: "T1" } },
      pe: { templates: ["T1", "Gone"] }, // Gone は削除済みのテンプレートの残り
      pf: { templateDefaults: { Task: "@none" } },
    });
    expect(
      templateDeleteLines(set, "T1", [...projects, { id: "pf", display: "HH-P6", title: "F" }]),
    ).toEqual([
      "4 件のプロジェクト別のルールからも削除されます。",
      "HH-P2・HH-P5 は、すべてのテンプレートを使えるようになります。",
    ]);
  });

  it("is empty when no rule changes and skips deleted projects", () => {
    expect(templateDeleteLines(s({ pc: { templates: ["T2"] } }), "T1", projects)).toEqual([]);
    expect(templateDeleteLines(s({ pz: { templates: ["T1"] } }), "T1", projects)).toEqual([]);
  });

  it("gives only the count line when nobody becomes unlimited", () => {
    const set = s({ pa: { templateDefaults: { Task: "T1" } } });
    expect(templateDeleteLines(set, "T1", projects)).toEqual([
      "1 件のプロジェクト別のルールからも削除されます。",
    ]);
  });
});

describe("rules projects and heading", () => {
  const projects = [
    { id: "pa", display: "HH-P1", title: "ログイン" },
    { id: "pb", display: "HH-P2", title: "決済" },
    { id: "pc", display: "HH-P3", title: "空" },
  ];
  const s: WorkspaceSettings = {
    ...sample(),
    projectRules: {
      pa: rules({ hiddenStatuses: [review] }),
      pb: rules({ hiddenStatuses: [review], templateDefaults: { Task: "T1" } }),
      pc: rules({}),
      pz: rules({ hiddenStatuses: [review] }),
    },
  };

  it("lists the projects having rules in the section, without empty or deleted ones", () => {
    expect(rulesProjects(s, "statuses", projects)).toEqual({
      projects: projects.slice(0, 2),
      text: "プロジェクト別のルールがあるプロジェクト: HH-P1 ログイン・HH-P2 決済(2 件)",
    });
    expect(rulesProjects(s, "templates", projects)?.projects.map((p) => p.id)).toEqual(["pb"]);
    expect(rulesProjects(s, "workflow", projects)).toBeNull();
  });

  it("reads the automation section from disabledRules", () => {
    const a = { ...s, projectRules: { pa: rules({ disabledRules: ["r1"] }) } };
    expect(rulesProjects(a, "automation", projects)?.projects.map((p) => p.id)).toEqual(["pa"]);
  });

  it("says the heading", () => {
    expect(rulesHeading("Web")).toBe(
      "このプロジェクトだけのルールです。ステータスや種類の定義は Web のままです。",
    );
  });
});
