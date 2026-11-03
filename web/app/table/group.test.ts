import { describe, expect, it } from "vitest";
import type { TaskView } from "../../core/api/client";
import { schema as base, task } from "../tree/fixtures";
import type { SortCtx } from "./columns";
import { groupTasks, windowWithHeld } from "./group";

const parents: Record<string, TaskView> = {
  E1: task("E1", "epic", null, {
    id: "REL-1",
    title: "API v2 への移行",
    progress: { done: 1, total: 5 },
  }),
  E8: task("E8", "epic", null, { id: "REL-8", title: "リリース作業" }),
};
const ctx: SortCtx = {
  schemaOf: () => base,
  taskOf: (u) => parents[u],
  storeOrder: ["shared"],
};

const shape = (items: ReturnType<typeof groupTasks>) =>
  items.map((i) => (i.type === "group" ? `# ${i.label} (${i.count})` : i.task.ulid));

describe("groupTasks", () => {
  const tasks = [
    task("T1", "task", "E1", {
      status: "doing",
      assignees: ["koki"],
      project: { ulid: "P", id: "REL-15", title: "リリース" },
    }),
    task("T2", "task", "E8", { status: "todo", assignees: ["koki", "tanaka"] }),
    task("T3", "task", "E1", { status: "done" }),
    task("T4", "task", null, { status: "todo", assignees: ["abe"] }),
  ];

  it("is the rows as they are with no group", () => {
    expect(shape(groupTasks(tasks, "none", ctx))).toEqual(["T1", "T2", "T3", "T4"]);
  });

  it("groups by parent in the order they appear, with the progress of the parent, and none last", () => {
    const items = groupTasks(tasks, "parent", ctx);
    expect(shape(items)).toEqual([
      "# API v2 への移行 (2)",
      "T1",
      "T3",
      "# リリース作業 (1)",
      "T2",
      "# 親なし (1)",
      "T4",
    ]);
    expect(items[0]).toMatchObject({ id: "REL-1", ulid: "E1", progress: { done: 1, total: 5 } });
    expect(items[3]).toMatchObject({ id: "REL-8", progress: undefined });
  });

  it("names a parent that is not in the cache", () => {
    const items = groupTasks([task("T9", "task", "GONE")], "parent", ctx);
    expect(shape(items)).toEqual(["# 不明な親 (1)", "T9"]);
  });

  it("tells a parent on its way from one that is not found", () => {
    const wait = { ...ctx, loading: () => true };
    expect(shape(groupTasks([task("T9", "task", "GONE")], "parent", wait))).toEqual([
      "# 読み込み中… (1)",
      "T9",
    ]);
  });

  it("groups by status in the order of sort:status, with the label of the schema", () => {
    expect(shape(groupTasks(tasks, "status", ctx))).toEqual([
      "# todo (2)",
      "T2",
      "T4",
      "# doing (1)",
      "T1",
      "# done (1)",
      "T3",
    ]);
  });

  it("groups by assignee by name and puts a task in the group of each assignee", () => {
    const items = groupTasks(tasks, "assignee", ctx);
    expect(shape(items)).toEqual([
      "# abe (1)",
      "T4",
      "# koki (2)",
      "T1",
      "T2",
      "# tanaka (1)",
      "T2",
      "# 未割り当て (1)",
      "T3",
    ]);
    const keys = items.map((i) => i.key);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("groups by project, the tasks outside any project last", () => {
    expect(shape(groupTasks(tasks, "project", ctx))).toEqual([
      "# リリース (1)",
      "T1",
      "# プロジェクトなし (3)",
      "T2",
      "T3",
      "T4",
    ]);
  });
});

describe("windowWithHeld", () => {
  const range = { startIndex: 20, endIndex: 25, overscan: 2, count: 100 };

  it("is the window with its overscan, and the rows that are held", () => {
    expect(windowWithHeld(range, [])).toEqual([18, 19, 20, 21, 22, 23, 24, 25, 26, 27]);
    expect(windowWithHeld(range, [90, 3, 22])).toEqual([
      3, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 90,
    ]);
  });

  it("stays inside the list", () => {
    expect(windowWithHeld({ startIndex: 0, endIndex: 2, overscan: 5, count: 4 }, [9])).toEqual([
      0, 1, 2, 3,
    ]);
  });
});
