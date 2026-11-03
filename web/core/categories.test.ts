import { describe, expect, it } from "vitest";
import { schema } from "../app/tree/fixtures";
import { CATEGORIES, CATEGORY_LABEL, categoryOf } from "./categories";

describe("categories", () => {
  it("lists the categories in the order sort:status puts them, each with a label", () => {
    expect(CATEGORIES).toEqual(["todo", "doing", "done", "canceled"]);
    expect(CATEGORIES.map((c) => CATEGORY_LABEL[c])).toEqual(["未着手", "進行中", "完了", "中止"]);
  });

  it("gives the category of a status in the schema", () => {
    expect(categoryOf(schema, "doing")).toBe("doing");
    expect(categoryOf(schema, "canceled")).toBe("canceled");
  });

  it("counts a status the schema does not list, or a schema not read, as todo", () => {
    expect(categoryOf(schema, "gone")).toBe("todo");
    expect(categoryOf(undefined, "doing")).toBe("todo");
  });
});
