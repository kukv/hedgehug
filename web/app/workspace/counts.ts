import type { TaskView } from "../../core/api/client";
import { type Category, categoryOf } from "../../core/categories";
import type { SchemaView } from "../../core/schema";

export type Counts = {
  total: number;
  projects: number;
  byCategory: Record<Category, number>;
};

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
