import { describe, expect, it } from "vitest";
import type { SchemaView } from "./schema";
import { compareStatusPlace, statusPlace, statusRank } from "./status-order";

const schema = {
  statuses: [
    { name: "backlog", category: "todo" },
    { name: "wip", category: "doing" },
    { name: "done", category: "done" },
    { name: "dropped", category: "canceled" },
    { name: "odd", category: "weird" },
  ],
} as unknown as SchemaView;

describe("statusPlace", () => {
  it("is the category, the place in the schema and the place of the store", () => {
    expect(statusPlace(schema, "wip", 2)).toEqual({ category: 1, pos: 1, store: 2 });
  });

  it("puts a status the schema lacks, a schema not read and a category not known after the rest", () => {
    const last = Number.MAX_SAFE_INTEGER;
    expect(statusPlace(schema, "gone", 0)).toEqual({ category: 4, pos: last, store: 0 });
    expect(statusPlace(undefined, "wip", 0)).toEqual({ category: 4, pos: last, store: 0 });
    expect(statusPlace(schema, "odd", 0).category).toBe(4);
    expect(statusPlace(schema, "wip", -1).store).toBe(last);
  });
});

describe("compareStatusPlace", () => {
  it("compares the category first, then the place in the schema, then the store", () => {
    const at = (s: string, store: number) => statusPlace(schema, s, store);
    expect(compareStatusPlace(at("backlog", 5), at("wip", 0))).toBeLessThan(0);
    expect(compareStatusPlace(at("done", 0), at("wip", 0))).toBeGreaterThan(0);
    expect(compareStatusPlace(at("wip", 0), at("wip", 1))).toBeLessThan(0);
    expect(compareStatusPlace(at("wip", 1), at("wip", 1))).toBe(0);
  });
});

describe("statusRank", () => {
  it("orders as the places do", () => {
    const rank = (s: string, store: number) => statusRank(schema, s, store);
    expect(rank("backlog", 3)).toBeLessThan(rank("wip", 0));
    expect(rank("wip", 0)).toBeLessThan(rank("wip", 1));
    expect(rank("dropped", 0)).toBeLessThan(rank("gone", 0));
  });
});
