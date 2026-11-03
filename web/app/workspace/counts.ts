import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";

export type Category = "todo" | "doing" | "done" | "canceled";

// The order the band and its legend show the categories in.
export const CATEGORIES: readonly Category[] = ["todo", "doing", "done", "canceled"];

export const CATEGORY_LABEL: Record<Category, string> = {
  todo: "未着手",
  doing: "進行中",
  done: "完了",
  canceled: "中止",
};

export type Counts = {
  total: number;
  projects: number;
  byCategory: Record<Category, number>;
};

// categoryOf is the category of a status. A status the schema does not list
// (the task has a problem that says so) counts as todo, so that the categories
// always add up to the total.
export function categoryOf(schema: SchemaView, status: string): Category {
  const c = schema.statuses?.find((s) => s.name === status)?.category;
  return c === "doing" || c === "done" || c === "canceled" ? c : "todo";
}

// countByCategory counts the tasks of a workspace as the band shows them:
// all of them (projects too), by the category of their status. projects is how many
// of them are of the project kind, which is part of total.
export function countByCategory(tasks: readonly TaskView[], schema: SchemaView): Counts {
  const byCategory: Record<Category, number> = { todo: 0, doing: 0, done: 0, canceled: 0 };
  let projects = 0;
  for (const t of tasks) {
    byCategory[categoryOf(schema, t.status)]++;
    if (t.kind === "project") projects++;
  }
  return { total: tasks.length, projects, byCategory };
}

const byRank = (a: TaskView, b: TaskView): number =>
  a.rank < b.rank ? -1 : a.rank > b.rank ? 1 : a.ulid < b.ulid ? -1 : a.ulid > b.ulid ? 1 : 0;

// projectsOf lists the projects, in rank order.
export function projectsOf(tasks: readonly TaskView[]): TaskView[] {
  return tasks.filter((t) => t.kind === "project").sort(byRank);
}

// rootsOf lists the tasks put straight at the root of the workspace, outside
// every project, in rank order.
export function rootsOf(tasks: readonly TaskView[]): TaskView[] {
  return tasks.filter((t) => !t.parent && t.kind !== "project").sort(byRank);
}
