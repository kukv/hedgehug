import { describe, expect, it } from "vitest";
import type { PlanningOverview } from "../../model/planning";
import type { Context } from "../items/detailContext";
import { settings } from "../items/settings.test-util";
import { cellEditor, groupKeys, rowValues } from "./cellEdit";
import { emptyValues, type SearchColumn, type SearchRow } from "./types";

function row(over: Partial<SearchRow> = {}): SearchRow {
  return {
    storeId: "s",
    storeName: "s",
    storeColor: "",
    statusId: "st1",
    workspaceId: "w",
    projectId: "p",
    itemId: "i",
    display: "HH-1",
    title: "t",
    kind: "Story",
    status: { name: "Todo", category: "todo" },
    assignee: null,
    priority: null,
    end: "",
    points: null,
    oldIdHit: false,
    readOnly: false,
    deleted: null,
    start: "",
    estimate: null,
    systems: [],
    sprint: "",
    release: "",
    project: { display: "HH", title: "P" },
    parent: null,
    created: "",
    updated: "",
    unused: [],
    removed: [],
    custom: {},
    values: emptyValues,
    ...over,
  };
}

const col = (key: string, type = ""): SearchColumn => ({ key, name: key, type, sortable: true });

const plan = {
  sprints: [{ id: "sp1", name: "S1", state: "active" }],
  releases: [{ id: "rl1", name: "R1", state: "open" }],
} as unknown as PlanningOverview;
const ctxOf = (s = settings): Context => ({
  settings: s,
  people: { members: [{ login: "taro", name: "太郎" }], me: "taro" },
  plan,
});
const editable = (e: ReturnType<typeof cellEditor>) => {
  if (e.kind !== "property") throw new Error(`kind: ${e.kind}`);
  return e.row;
};

describe("cellEditor", () => {
  it("display without a context", () => {
    expect(cellEditor(row(), col("priority"), undefined)).toEqual({ kind: "display" });
  });

  it("display for deleted rows, unused cells, and display-only columns", () => {
    const ctx = ctxOf();
    expect(cellEditor(row({ deleted: { daysLeft: 3 } }), col("priority"), ctx)).toEqual({
      kind: "display",
    });
    expect(cellEditor(row({ unused: ["priority"] }), col("priority"), ctx)).toEqual({
      kind: "display",
    });
    expect(cellEditor(row({ unused: ["sprint"] }), col("sprint"), ctx)).toEqual({
      kind: "display",
    });
    for (const k of ["id", "kind", "title", "project", "parent", "created", "updated"]) {
      expect(cellEditor(row(), col(k), ctx)).toEqual({ kind: "display" });
    }
  });

  it("display for a url property", () => {
    const s = {
      ...settings,
      properties: [
        ...settings.properties,
        {
          id: "F3",
          key: "F3",
          name: "Link",
          type: "url" as const,
          builtin: false,
          kinds: ["Story"],
          hidden: false,
          choices: [],
        },
      ],
    };
    expect(cellEditor(row(), col("p:f3", "url"), ctxOf(s))).toEqual({ kind: "display" });
  });

  it("maps builtin columns", () => {
    const ctx = ctxOf();
    const r = row({ values: { ...emptyValues, priority: "pr1", systems: ["sy2"] } });
    const pr = editable(cellEditor(r, col("priority"), ctx));
    expect(pr.property.key).toBe("priority");
    expect(pr.value).toBe(r.values.priority);
    expect(pr.options.some((o) => o.label === "なし")).toBe(false);
    const sy = editable(cellEditor(r, col("system"), ctx));
    expect(sy.property.key).toBe("systems");
    expect(sy.value).toEqual(r.values.systems);
    const as = editable(cellEditor(r, col("assignee"), ctx));
    expect(as.property.type).toBe("user");
    expect(as.options[0]?.label).toBe("未割り当て");
  });

  it("maps a custom column by lower-cased key, with the row's own type", () => {
    const s = {
      ...settings,
      properties: settings.properties.map((p) =>
        p.id === "F2" ? { ...p, name: "Due", key: "Due" } : p,
      ),
    };
    const r = row({ values: { ...emptyValues, fields: { F2: "hello" } } });
    const e = editable(cellEditor(r, col("p:due", "date"), ctxOf(s)));
    expect(e.property.type).toBe("text");
    expect(e.value).toBe("hello");
  });

  it("edits a hidden builtin property", () => {
    const s = {
      ...settings,
      properties: settings.properties.map((p) =>
        p.id === "priority" ? { ...p, hidden: true } : p,
      ),
    };
    expect(cellEditor(row(), col("priority"), ctxOf(s)).kind).toBe("property");
  });

  it("plan cells", () => {
    const ctx = ctxOf();
    const sp = cellEditor(row({ values: { ...emptyValues, sprint: "sp1" } }), col("sprint"), ctx);
    if (sp.kind !== "plan") throw new Error("kind");
    expect(sp.key).toBe("sprint");
    expect(sp.options[0]).toEqual({ value: "", label: "バックログ" });
    expect(sp.value).toBe("sp1");
    const rl = cellEditor(row(), col("release"), ctx);
    if (rl.kind !== "plan") throw new Error("kind");
    expect(rl.options[0]).toEqual({ value: "", label: "なし" });
    expect(rl.value).toBe("");
  });
});

describe("rowValues", () => {
  it("rowValues reads ids", () => {
    const r = row({
      statusId: "st2",
      assignee: { login: "taro", name: "太郎" },
      start: "2026-01-01",
      points: 3,
      estimate: 1.5,
      values: { ...emptyValues, priority: "pr1", systems: ["sy1"], choices: { F1: ["C1"] } },
    });
    expect(rowValues(r)).toMatchObject({
      status: "st2",
      assignee: "taro",
      priority: "pr1",
      start: "2026-01-01",
      points: 3,
      estimate: 1.5,
      systems: ["sy1"],
      choices: { F1: ["C1"] },
    });
  });
});

describe("groupKeys", () => {
  it("groupKeys numbers repeats", () => {
    const g = (label: string) => ({ label, rows: [] });
    expect(groupKeys([g("A"), g("B"), g("A")])).toEqual(["A", "B", "A#2"]);
  });
});
