import type { SchemaView } from "./schema";

export type Category = "todo" | "doing" | "done" | "canceled";

// The categories in the order sort:status puts them (internal/app/domain/schema Categories), which is also the
// order the workspace band, the kanban columns and the table groups show them in.
export const CATEGORIES: readonly Category[] = ["todo", "doing", "done", "canceled"];

export const CATEGORY_LABEL: Record<Category, string> = {
  todo: "未着手",
  doing: "進行中",
  done: "完了",
  canceled: "中止",
};

// categoryOf is the category of a status. A status the schema does not list (the task has a problem
// that says so), or a schema that is not read yet, counts as todo, so that the categories always add
// up to the total.
export function categoryOf(schema: SchemaView | undefined, status: string): Category {
  const c = schema?.statuses?.find((s) => s.name === status)?.category;
  return c === "doing" || c === "done" || c === "canceled" ? c : "todo";
}
