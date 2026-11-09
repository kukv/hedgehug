import { describe, expect, it } from "vitest";
import type { ShowView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import { tv } from "../../core/testing";
import { task } from "../tree/fixtures";
import type { Column } from "./board";
import {
  canTransition,
  decideDrop,
  requiredFields,
  requiredGap,
  requiredLabel,
  scopedStatusesOf,
} from "./drop";

const schema = {
  statuses: [
    { name: "todo", category: "todo", default: true, label: "未着手" },
    { name: "doing", category: "doing", default: true, label: "進行中" },
    { name: "review", category: "doing", label: "レビュー" },
    { name: "done", category: "done", default: true, label: "完了" },
  ],
  kinds: [],
  props: {
    points: { type: "number" },
    area: { type: "text" },
    tags: { type: "multi_enum", values: ["a", "b"] },
  },
  scoped: [
    {
      where: "project:P1",
      statuses: ["todo", "doing", "done"],
      workflow: { transitions: { todo: ["doing"] }, required: { doing: ["area"] } },
    },
  ],
  workflow: {
    transitions: { done: ["todo"], review: [] },
    required: { done: ["points", "assignees"], doing: [] },
  },
} as unknown as SchemaView;

const col = (key: string, over: Partial<Column> = {}): Column => ({
  key,
  label: key,
  category: "todo",
  status: key,
  unknown: false,
  ...over,
});

const base = {
  task: task("A", "task", null, { id: "REL-1", title: "仕事", status: "todo" }),
  sameLane: true,
  swimlane: "none" as const,
  mode: "status" as const,
  schema,
};

describe("a task of a read-only Store", () => {
  it("is not moved, and the reason is the one of the Store", () => {
    const d = decideDrop({ ...base, column: col("doing"), readOnly: "remote に書き込めません" });
    expect(d).toMatchObject({ ok: false, silent: false });
    if (!d.ok) expect(d.reason).toContain("remote に書き込めません");
  });

  it("is refused even on the column it is in", () => {
    const d = decideDrop({ ...base, column: col("todo"), readOnly: "x" });
    expect(d).toMatchObject({ ok: false, silent: false });
  });

  it("is moved as before with no reason", () => {
    expect(decideDrop({ ...base, column: col("doing"), readOnly: null }).ok).toBe(true);
  });
});

describe("workflow of a task", () => {
  it("allows a move the store lists, and any move from a status it does not list", () => {
    expect(canTransition(schema, undefined, "done", "todo")).toBe(true);
    expect(canTransition(schema, undefined, "done", "doing")).toBe(false);
    expect(canTransition(schema, undefined, "todo", "done")).toBe(true);
    expect(canTransition(schema, undefined, "review", "todo")).toBe(false);
    expect(canTransition(schema, undefined, "todo", "todo")).toBe(true);
  });

  it("asks the project too: both have to allow it", () => {
    expect(canTransition(schema, "P1", "todo", "done")).toBe(false);
    expect(canTransition(schema, "P1", "todo", "doing")).toBe(true);
    expect(canTransition(schema, "P2", "todo", "done")).toBe(true);
  });

  it("joins the required fields of the store and of the project", () => {
    expect(requiredFields(schema, undefined, "done")).toEqual(["points", "assignees"]);
    expect(requiredFields(schema, "P1", "doing")).toEqual(["area"]);
    expect(requiredFields(schema, "P1", "todo")).toEqual([]);
  });

  it("reads the statuses a project shows", () => {
    expect(scopedStatusesOf(schema, "P1")).toEqual(["todo", "doing", "done"]);
    expect(scopedStatusesOf(schema, "P2")).toBeUndefined();
    expect(scopedStatusesOf(schema, undefined)).toBeUndefined();
  });
});

describe("requiredGap", () => {
  const t = task("A", "task", null, { props: { points: 0, area: "" }, assignees: [] });

  it("names the empty fields the dialog can take in", () => {
    expect(requiredGap(t, ["points", "area", "assignees", "labels"], schema)).toEqual({
      fill: ["area", "assignees", "labels"],
      elsewhere: [],
    });
  });

  it("counts 0 and false as filled, and an empty list as empty", () => {
    const u = task("B", "task", null, { props: { points: 0, tags: [] }, assignees: ["gh/koki"] });
    expect(requiredGap(u, ["points", "tags", "assignees"], schema).fill).toEqual(["tags"]);
  });

  it("sends the fields only the detail can fill there: the parent, the five relations, and what the API does not show", () => {
    const names = [
      "parent",
      "blocked_by",
      "causes",
      "relates",
      "duplicates",
      "clones",
      "sprint",
      "releases",
      "links",
      "nothing",
    ];
    expect(requiredGap(t, names, schema)).toEqual({ fill: [], elsewhere: names });
  });

  it("leaves out the parent and the relations the task has", () => {
    const ref = [{ ulid: "Z", id: "REL-9", title: "z" }];
    const u = task("B", "task", "P", { blocked_by: ref, clones: ref });
    expect(requiredGap(u, ["parent", "blocked_by", "clones"], schema).elsewhere).toEqual([]);
  });

  it("shows a relation by its display name", () => {
    expect(requiredLabel(schema, "blocked_by")).toBe("Blocked by");
    expect(requiredLabel(schema, "relates")).toBe("Relates to");
  });

  it("reads watchers and repos from the full task, and counts them as lacking without it", () => {
    const full = { watchers: ["gh/koki"], repos: [] } as unknown as ShowView;
    expect(requiredGap(t, ["watchers", "repos"], schema).elsewhere).toEqual(["watchers", "repos"]);
    expect(requiredGap(t, ["watchers", "repos"], schema, full).elsewhere).toEqual(["repos"]);
  });
});

describe("decideDrop", () => {
  it("is silent for the column the task is in", () => {
    expect(decideDrop({ ...base, column: col("todo") })).toMatchObject({ ok: false, silent: true });
  });

  it("moves to the status of the column, with the fields still empty", () => {
    const d = decideDrop({
      ...base,
      task: task("A", "task", null, { status: "doing", props: {}, assignees: [] }),
      column: col("review"),
    });
    expect(d).toEqual({ ok: true, status: "review", fill: [], elsewhere: [] });
    const done = decideDrop({
      ...base,
      task: task("A", "task", null, { status: "doing", props: { points: 3 }, assignees: [] }),
      column: col("done"),
    });
    expect(done).toEqual({ ok: true, status: "done", fill: ["assignees"], elsewhere: [] });
  });

  it("lists the required fields only the detail can fill, next to the ones the dialog can", () => {
    const needy = {
      ...schema,
      workflow: { ...schema.workflow, required: { done: ["points", "parent", "watchers"] } },
    } as unknown as SchemaView;
    const t = task("A", "task", null, { status: "doing", props: {} });
    const d = decideDrop({ ...base, task: t, schema: needy, column: col("done") });
    expect(d).toEqual({
      ok: true,
      status: "done",
      fill: ["points"],
      elsewhere: ["parent", "watchers"],
    });
    const full = { watchers: ["gh/koki"] } as unknown as ShowView;
    const e = decideDrop({ ...base, task: t, schema: needy, column: col("done"), detail: full });
    expect(e).toMatchObject({ elsewhere: ["parent"] });
  });

  it("refuses to cross a swimlane, and says what would change", () => {
    const a = decideDrop({
      ...base,
      column: col("doing"),
      sameLane: false,
      swimlane: "assignee",
    });
    expect(a).toMatchObject({
      ok: false,
      silent: false,
      title: "担当者の行をまたぐ移動はできません",
    });
    const p = decideDrop({ ...base, column: col("todo"), sameLane: false, swimlane: "project" });
    expect(p).toMatchObject({
      ok: false,
      silent: false,
      title: "プロジェクトをまたぐ移動はできません",
    });
  });

  it("refuses a move the workflow does not allow, with the reason", () => {
    const d = decideDrop({
      ...base,
      task: task("A", "task", null, { id: "REL-1", title: "仕事", status: "done" }),
      column: col("doing"),
    });
    expect(d).toMatchObject({
      ok: false,
      silent: false,
      title: "「完了」から「進行中」には移せません",
    });
    expect(d.ok ? "" : d.reason).toContain("REL-1 仕事");
  });

  it("writes the ID of the task as the screen is told to", () => {
    const d = decideDrop({
      ...base,
      task: task("A", "task", null, { id: "REL-1", title: "仕事", status: "done", store: "work" }),
      column: col("doing"),
      idOf: (t) => `${t.store}:${t.id}`,
    });
    expect(d.ok ? "" : d.reason).toContain("work:REL-1 仕事");
  });

  it("uses the workflow of the scope the server names for the task", () => {
    const t = task("A", "task", null, {
      status: "todo",
      project: { ulid: "P1", id: "P-1", title: "p" },
      scope: "P1",
    });
    expect(decideDrop({ ...base, task: t, column: col("done") })).toMatchObject({
      ok: false,
      silent: false,
    });
    expect(decideDrop({ ...base, task: t, column: col("doing") })).toMatchObject({
      ok: true,
      fill: ["area"],
    });
  });

  describe("the scope is the one the server names, not the project", () => {
    const withEpic = {
      ...schema,
      scoped: [
        ...(schema.scoped ?? []),
        { where: "project:E1", workflow: { required: { done: ["labels"] } } },
      ],
    } as unknown as SchemaView;
    const project = { ulid: "P1", id: "P-1", title: "p" };

    it("asks what the override of an epic needs, though the project has another", () => {
      const t = task("A", "task", null, { status: "doing", props: {}, project, scope: "E1" });
      const d = decideDrop({ ...base, task: t, schema: withEpic, column: col("done") });
      expect(d).toMatchObject({ ok: true, fill: ["points", "assignees", "labels"] });
    });

    it("allows the move the epic allows, though the project forbids it", () => {
      const t = task("A", "task", null, { status: "todo", props: {}, project, scope: "E1" });
      const d = decideDrop({ ...base, task: t, schema: withEpic, column: col("done") });
      expect(d).toMatchObject({ ok: true, status: "done" });
    });

    it("applies no override to a task the server gives none", () => {
      const t = task("A", "task", null, { status: "todo", props: {}, project });
      const d = decideDrop({ ...base, task: t, schema: withEpic, column: col("done") });
      expect(d).toMatchObject({ ok: true, status: "done" });
    });
  });

  it("refuses a status the schema of the task does not have", () => {
    const d = decideDrop({ ...base, column: col("shipped") });
    expect(d).toMatchObject({ ok: false, silent: false, title: "「shipped」には移せません" });
  });

  it("refuses a column no schema lists", () => {
    const d = decideDrop({ ...base, column: col("gone", { status: null, unknown: true }) });
    expect(d).toMatchObject({ ok: false, silent: false });
  });

  it("waits for the schema", () => {
    const d = decideDrop({ ...base, schema: undefined, column: col("doing") });
    expect(d).toMatchObject({
      ok: false,
      silent: false,
      title: "少し待ってからやり直してください",
    });
  });

  describe("by category", () => {
    const by = { ...base, mode: "category" as const };

    it("moves to the default status of the category in the schema of the task", () => {
      const d = decideDrop({
        ...by,
        column: col("doing", { category: "doing", status: null }),
      });
      expect(d).toEqual({ ok: true, status: "doing", fill: [], elsewhere: [] });
    });

    it("is silent for the category the task is in", () => {
      const t = task("A", "task", null, { status: "review" });
      const d = decideDrop({
        ...by,
        task: t,
        column: col("doing", { category: "doing", status: null }),
      });
      expect(d).toMatchObject({ ok: false, silent: true });
    });

    it("refuses a category the schema has no status in", () => {
      const d = decideDrop({
        ...by,
        column: col("canceled", { category: "canceled", status: null, label: "中止" }),
      });
      expect(d).toMatchObject({ ok: false, silent: false, title: "「中止」には移せません" });
    });
  });
});

describe("requiredGap of the fields of a plan", () => {
  const schema = { props: {} } as unknown as SchemaView;
  it("reads the sprint, the releases and the links from the task", () => {
    const names = ["sprint", "releases", "links"];
    expect(requiredGap(tv("A"), names, schema).elsewhere).toEqual(names);
    const full = tv("A", "a", {
      sprint: { ulid: "S" },
      releases: [{ ulid: "R" }],
      links: [{ kind: "pr", ref: "x" } as never],
    });
    expect(requiredGap(full, names, schema)).toEqual({ fill: [], elsewhere: [] });
  });
});
