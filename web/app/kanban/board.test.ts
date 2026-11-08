import { describe, expect, it } from "vitest";
import type { SchemaView } from "../../core/schema";
import { task } from "../tree/fixtures";
import { autoHints, buildBoard, columnKeyOf, NO_LANE } from "./board";

const sc = (statuses: { name: string; category: string; label?: string }[]) =>
  ({
    statuses,
    kinds: [],
    props: {},
    scoped: [],
    workflow: { transitions: {}, required: {} },
  }) as unknown as SchemaView;

// shared: backlog, todo, doing, review, done, canceled
const shared = sc([
  { name: "backlog", category: "todo", label: "バックログ" },
  { name: "todo", category: "todo", label: "未着手" },
  { name: "doing", category: "doing", label: "進行中" },
  { name: "review", category: "doing", label: "レビュー" },
  { name: "done", category: "done", label: "完了" },
  { name: "canceled", category: "canceled", label: "中止" },
]);
// work: todo, wip, shipped, done (its "todo" has another label)
const work = sc([
  { name: "todo", category: "todo", label: "ToDo" },
  { name: "wip", category: "doing" },
  { name: "shipped", category: "done" },
  { name: "done", category: "done" },
  { name: "dropped", category: "canceled" },
]);

const opts = (over: Partial<Parameters<typeof buildBoard>[1]> = {}) => ({
  mode: "status" as const,
  swimlane: "none" as const,
  schemas: new Map([["shared", shared]]),
  storeOrder: ["shared", "work"],
  ...over,
});
const keys = (b: ReturnType<typeof buildBoard>) => b.columns.map((c) => c.key);
const ulids = (list: { ulid: string }[]) => list.map((t) => t.ulid);

describe("buildBoard: status columns", () => {
  it("makes a column of every status of the store, in the order of the schema, but the canceled one has to have tasks", () => {
    const b = buildBoard([task("A", "task", null, { status: "todo" })], opts({ store: "shared" }));
    expect(keys(b)).toEqual(["backlog", "todo", "doing", "review", "done"]);
    expect(b.columns[1]).toMatchObject({
      label: "未着手",
      category: "todo",
      status: "todo",
      unknown: false,
    });
  });

  it("shows the canceled column when a task is in it", () => {
    const b = buildBoard([task("A", "task", null, { status: "canceled" })], opts());
    expect(keys(b)).toContain("canceled");
  });

  it("joins the statuses of the stores of the tasks: by category, then position, then store", () => {
    const tasks = [
      task("A", "task", null, { store: "shared", status: "todo" }),
      task("B", "task", null, { store: "work", status: "wip" }),
    ];
    const b = buildBoard(
      tasks,
      opts({
        schemas: new Map([
          ["shared", shared],
          ["work", work],
        ]),
      }),
    );
    // As sort:status does: the category, then the place in the schema (shared: backlog 0, todo 1,
    // doing 2, review 3, done 4; work: todo 0, wip 1, shipped 2), then the order of the stores.
    // A name two stores share is one column, placed by the first store.
    expect(keys(b)).toEqual(["backlog", "todo", "wip", "doing", "review", "shipped", "done"]);
    // the label is the first store's
    expect(b.columns.find((c) => c.key === "todo")?.label).toBe("未着手");
  });

  it("carries the color of a status only when one store gives the column", () => {
    const withColor = (color: string) =>
      sc([
        { name: "todo", category: "todo", color } as never,
        { name: "wip", category: "doing", color } as never,
      ]);
    const tasks = [
      task("A", "task", null, { store: "shared", status: "todo" }),
      task("B", "task", null, { store: "work", status: "wip" }),
    ];
    const one = buildBoard(tasks, opts({ schemas: new Map([["shared", withColor("pink")]]) }));
    expect(one.columns.find((c) => c.key === "todo")?.color).toBe("pink");
    const two = buildBoard(
      tasks,
      opts({
        schemas: new Map([
          ["shared", withColor("pink")],
          ["work", withColor("teal")],
        ]),
      }),
    );
    // both stores have todo and wip: no one color to draw
    expect(two.columns.find((c) => c.key === "todo")?.color).toBeUndefined();
    expect(two.columns.find((c) => c.key === "wip")?.color).toBeUndefined();
  });

  it("leaves out a store that has no task, unless it is the store of the screen", () => {
    const schemas = new Map([
      ["shared", shared],
      ["work", work],
    ]);
    const only = buildBoard([task("A", "task", null, { store: "shared" })], opts({ schemas }));
    expect(keys(only)).not.toContain("wip");
    const screen = buildBoard(
      [task("A", "task", null, { store: "shared" })],
      opts({ schemas, store: "work" }),
    );
    expect(keys(screen)).toContain("wip");
  });

  it("keeps to the statuses of the project, and never hides a task", () => {
    const tasks = [
      task("A", "task", null, { status: "todo" }),
      task("B", "task", null, { status: "review" }),
    ];
    const b = buildBoard(
      tasks,
      opts({ store: "shared", scopedStatuses: ["todo", "doing", "done"] }),
    );
    // review has a task, so it stays; backlog is out
    expect(keys(b)).toEqual(["todo", "doing", "review", "done"]);
  });

  it("adds a column for a status no schema lists, at the end", () => {
    const b = buildBoard([task("A", "task", null, { status: "gone" })], opts({ store: "shared" }));
    expect(keys(b).at(-1)).toBe("gone");
    expect(b.columns.at(-1)).toMatchObject({
      label: "gone",
      status: null,
      unknown: true,
      category: "todo",
    });
  });

  it("puts the tasks in the column of their status, in the order they come", () => {
    const tasks = [
      task("A", "task", null, { status: "todo" }),
      task("B", "task", null, { status: "doing" }),
      task("C", "task", null, { status: "todo" }),
    ];
    const b = buildBoard(tasks, opts());
    const lane = b.lanes[0];
    expect(lane?.key).toBe("");
    const at = (k: string) => keys(b).indexOf(k);
    expect(ulids(lane?.cells[at("todo")] ?? [])).toEqual(["A", "C"]);
    expect(ulids(lane?.cells[at("doing")] ?? [])).toEqual(["B"]);
  });
});

describe("buildBoard: category columns", () => {
  it("makes the three categories, and the canceled one when a task is in it", () => {
    const b = buildBoard([task("A", "task", null, { status: "wip", store: "work" })], {
      ...opts({ schemas: new Map([["work", work]]) }),
      mode: "category",
    });
    expect(keys(b)).toEqual(["todo", "doing", "done"]);
    expect(b.columns.map((c) => c.label)).toEqual(["未着手", "進行中", "完了"]);
    expect(b.columns.every((c) => c.status === null)).toBe(true);
    expect(ulids(b.lanes[0]?.cells[1] ?? [])).toEqual(["A"]);
    const c = buildBoard([task("A", "task", null, { status: "dropped", store: "work" })], {
      ...opts({ schemas: new Map([["work", work]]) }),
      mode: "category",
    });
    expect(keys(c)).toEqual(["todo", "doing", "done", "canceled"]);
  });

  it("puts a task of a store without a schema in todo", () => {
    const b = buildBoard([task("A", "task", null, { status: "wip", store: "work" })], {
      ...opts(),
      mode: "category",
    });
    expect(ulids(b.lanes[0]?.cells[0] ?? [])).toEqual(["A"]);
  });
});

describe("buildBoard: swimlanes", () => {
  it("puts a task with two assignees in the row of each, and the unassigned last", () => {
    const tasks = [
      task("A", "task", null, { assignees: ["gh/sato"] }),
      task("B", "task", null, { assignees: [] }),
      task("C", "task", null, { assignees: ["gh/koki", "gh/sato"] }),
    ];
    const b = buildBoard(tasks, opts({ swimlane: "assignee" }));
    expect(b.lanes.map((l) => l.key)).toEqual(["gh/koki", "gh/sato", NO_LANE]);
    expect(b.lanes.map((l) => l.label)).toEqual(["koki", "sato", "未割り当て"]);
    const todo = keys(b).indexOf("todo");
    expect(ulids(b.lanes[0]?.cells[todo] ?? [])).toEqual(["C"]);
    expect(ulids(b.lanes[1]?.cells[todo] ?? [])).toEqual(["A", "C"]);
    expect(ulids(b.lanes[2]?.cells[todo] ?? [])).toEqual(["B"]);
  });

  it("sorts the assignee rows by the name shown, whatever the host", () => {
    const tasks = [
      task("A", "task", null, { assignees: ["github.com/zed"] }),
      task("B", "task", null, { assignees: ["gitlab.com/amy"] }),
    ];
    const b = buildBoard(tasks, opts({ swimlane: "assignee" }));
    expect(b.lanes.map((l) => l.label)).toEqual(["amy", "zed"]);
  });

  it("makes a row of each project in the order they come, and the tasks outside last", () => {
    const p2 = { ulid: "P2", id: "REL-20", title: "二つ目" };
    const p1 = { ulid: "P1", id: "REL-15", title: "一つ目" };
    const tasks = [
      task("A", "task", null, {}),
      task("B", "task", null, { project: p2 }),
      task("C", "task", null, { project: p1 }),
      task("D", "task", null, { project: p2 }),
    ];
    const b = buildBoard(tasks, opts({ swimlane: "project" }));
    expect(b.lanes.map((l) => l.key)).toEqual(["P2", "P1", NO_LANE]);
    expect(b.lanes.map((l) => l.label)).toEqual([
      "REL-20 二つ目",
      "REL-15 一つ目",
      "プロジェクトなし",
    ]);
    const todo = keys(b).indexOf("todo");
    expect(ulids(b.lanes[0]?.cells[todo] ?? [])).toEqual(["B", "D"]);
  });

  it("has one row without a name when there are no swimlanes, and none for no tasks", () => {
    const b = buildBoard([], opts({ swimlane: "assignee" }));
    expect(b.lanes).toEqual([]);
    const one = buildBoard([task("A", "task", null, {})], opts());
    expect(one.lanes).toHaveLength(1);
    expect(one.lanes[0]).toMatchObject({ key: "", label: "" });
  });
});

describe("columnKeyOf", () => {
  it("is the status, or the category of the status in the schema of the task", () => {
    const t = task("A", "task", null, { status: "wip", store: "work" });
    expect(columnKeyOf(t, "status", work)).toBe("wip");
    expect(columnKeyOf(t, "category", work)).toBe("doing");
    expect(columnKeyOf(t, "category", undefined)).toBe("todo");
  });
});

describe("autoHints", () => {
  const schema = sc([
    { name: "todo", category: "todo" },
    { name: "in_progress", category: "doing" },
    { name: "done", category: "done" },
  ]);
  const opened = {
    id: "a",
    on: "pr.opened",
    status: "in_progress",
    enabled: true,
    disabled_in: [],
  };
  const merged = {
    id: "b",
    on: "pr.merged",
    category: "done",
    when: "all_linked_prs_merged",
    enabled: true,
    disabled_in: [],
  };

  it("puts the words on the column of the status, or of the default of the category", () => {
    expect([...autoHints([opened, merged], schema, "status", undefined)]).toEqual([
      ["in_progress", "PR を開くと自動でここへ"],
      ["done", "全 PR のマージで自動でここへ"],
    ]);
  });

  it("uses the category for a board by category", () => {
    expect([...autoHints([opened], schema, "category", undefined)]).toEqual([
      ["doing", "PR を開くと自動でここへ"],
    ]);
  });

  it("leaves out a rule that is off, an event with no words and another project's rule", () => {
    const other = { ...opened, project: { ulid: "P2", id: "X-2", title: "x" } };
    const rules = [{ ...opened, enabled: false }, { ...opened, on: "issue.closed" }, other];
    expect(autoHints(rules, schema, "status", "P1").size).toBe(0);
    expect(autoHints([other], schema, "status", "P2").get("in_progress")).toBeDefined();
  });

  it("joins two rules on one column", () => {
    const both = [opened, { ...merged, status: "in_progress", when: undefined }];
    expect(autoHints(both, schema, "status", undefined).get("in_progress")).toBe(
      "PR を開くと自動でここへ · PR のマージで自動でここへ",
    );
  });
});
