import { describe, expect, it } from "vitest";
import { schema, task } from "../tree/fixtures";
import { categoryOf, countByCategory, projectsOf, rootsOf } from "./counts";

describe("countByCategory", () => {
  it("counts every task by the category of its status, projects included", () => {
    const tasks = [
      task("P1", "project", null, { status: "doing" }),
      task("T1", "task", "P1", { status: "doing" }),
      task("T2", "task", "P1", { status: "todo" }),
      task("T3", "task", null, { status: "backlog" }),
      task("T4", "task", null, { status: "done" }),
      task("T5", "task", null, { status: "canceled" }),
    ];
    expect(countByCategory(tasks, schema)).toEqual({
      total: 6,
      projects: 1,
      byCategory: { todo: 2, doing: 2, done: 1, canceled: 1 },
    });
  });

  it("counts a status the schema does not list as todo, so that the categories add up", () => {
    const c = countByCategory([task("T1", "task", null, { status: "gone" })], schema);
    expect(c.byCategory).toEqual({ todo: 1, doing: 0, done: 0, canceled: 0 });
    expect(categoryOf(schema, "gone")).toBe("todo");
  });

  it("counts nothing for no tasks", () => {
    expect(countByCategory([], schema)).toEqual({
      total: 0,
      projects: 0,
      byCategory: { todo: 0, doing: 0, done: 0, canceled: 0 },
    });
  });
});

describe("projectsOf and rootsOf", () => {
  const tasks = [
    task("T9", "task", null, { rank: "c" }),
    task("P2", "project", null, { rank: "b" }),
    task("T1", "task", "P2", { rank: "a" }),
    task("P1", "project", null, { rank: "a" }),
    task("E1", "epic", null, { rank: "a" }),
  ];

  it("lists the projects in rank order", () => {
    expect(projectsOf(tasks).map((t) => t.ulid)).toEqual(["P1", "P2"]);
  });

  it("lists the tasks at the root that are not projects, in rank order", () => {
    expect(rootsOf(tasks).map((t) => t.ulid)).toEqual(["E1", "T9"]);
  });

  it("does not change the list it is given", () => {
    const before = tasks.map((t) => t.ulid);
    projectsOf(tasks);
    rootsOf(tasks);
    expect(tasks.map((t) => t.ulid)).toEqual(before);
  });
});
