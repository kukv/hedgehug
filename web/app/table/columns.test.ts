import { describe, expect, it } from "vitest";
import type { SchemaView } from "../../core/schema";
import { schema as base, task } from "../tree/fixtures";
import {
  columnCandidates,
  compareKeys,
  patchFieldOf,
  type SortCtx,
  shownColumns,
  sortKey,
  toggleColumn,
} from "./columns";

const withProps = (props: Record<string, unknown>): SchemaView =>
  ({ ...base, props }) as unknown as SchemaView;

const shared = withProps({
  due: { type: "date", label: "期限" },
  points: { type: "number" },
  area: { type: "multi_enum", values: ["backend", "frontend"] },
  relates: { type: "task_ref", label: "関連" },
  blocked_by: { type: "task_ref" },
  status: { type: "text" },
});
const me = withProps({
  area: { type: "multi_enum", label: "領域", values: ["x"] },
  team: { type: "text", label: "チーム" },
});

const ids = (list: { id: string }[]) => list.map((c) => c.id);

describe("columnCandidates", () => {
  it("lists the builtin columns, then the properties of the schemas by name", () => {
    expect(ids(columnCandidates([shared]))).toEqual([
      "id",
      "title",
      "kind",
      "status",
      "assignee",
      "label",
      "parent",
      "created",
      "updated",
      "pr",
      "area",
      "due",
      "points",
      "relates",
    ]);
  });

  it("takes the union of several stores and the label of the first store that has the property", () => {
    const list = columnCandidates([shared, me]);
    expect(ids(list).slice(10)).toEqual(["area", "due", "points", "relates", "team"]);
    expect(list.find((c) => c.id === "area")?.label).toBe("area");
    expect(list.find((c) => c.id === "team")?.label).toBe("チーム");
    expect(list.find((c) => c.id === "due")?.label).toBe("期限");
  });

  it("shows ID, parent, created, updated, the pull requests and task references only", () => {
    const readonly = columnCandidates([shared])
      .filter((c) => c.readonly)
      .map((c) => c.id);
    expect(readonly).toEqual(["id", "parent", "created", "updated", "pr", "relates"]);
  });

  it("has no property column without a schema", () => {
    expect(columnCandidates([])).toHaveLength(10);
  });
});

describe("shownColumns", () => {
  const candidates = columnCandidates([shared]);

  it("keeps the order of the view and leaves out what no schema defines", () => {
    expect(ids(shownColumns(["title", "nope", "due", "id"], candidates))).toEqual([
      "title",
      "due",
      "id",
    ]);
  });

  it("shows a column once", () => {
    expect(ids(shownColumns(["id", "id"], candidates))).toEqual(["id"]);
  });

  it("leaves out a property when the store that defines it is not shown", () => {
    expect(ids(shownColumns(["id", "team"], columnCandidates([shared])))).toEqual(["id"]);
    expect(ids(shownColumns(["id", "team"], columnCandidates([shared, me])))).toEqual([
      "id",
      "team",
    ]);
  });
});

describe("toggleColumn", () => {
  it("adds at the end and takes out, and keeps the names it does not show", () => {
    expect(toggleColumn(["id", "ghost"], "due", true)).toEqual(["id", "ghost", "due"]);
    expect(toggleColumn(["id", "due", "ghost"], "due", false)).toEqual(["id", "ghost"]);
    expect(toggleColumn(["id", "due"], "due", true)).toEqual(["id", "due"]);
  });
});

describe("patchFieldOf", () => {
  const c = columnCandidates([shared]);
  const of = (id: string) => {
    const spec = c.find((x) => x.id === id);
    return spec ? patchFieldOf(spec) : "missing";
  };
  it("names the field of the PATCH", () => {
    expect(of("title")).toBe("title");
    expect(of("kind")).toBe("kind");
    expect(of("status")).toBe("status");
    expect(of("assignee")).toBe("assignees");
    expect(of("label")).toBe("labels");
    expect(of("points")).toBe("props.points");
  });
  it("has none for what is only shown", () => {
    for (const id of ["id", "parent", "created", "updated", "pr", "relates"])
      expect(of(id)).toBeNull();
  });
});

describe("sortKey", () => {
  const parent = task("P", "project", null, { id: "REL-1" });
  const ctx: SortCtx = {
    schemaOf: (store) =>
      store === "shared"
        ? ({
            ...shared,
            props: {
              ...shared.props,
              prio: { type: "enum", values: ["low", "mid", "high"] },
            },
          } as SchemaView)
        : undefined,
    taskOf: (u) => (u === "P" ? parent : undefined),
    storeOrder: ["shared", "me"],
  };

  it("reads the builtin fields", () => {
    const t = task("T1", "task", "P", {
      id: "REL-4",
      title: "",
      assignees: ["koki", "tanaka"],
      labels: ["a", "b"],
      created: "2026-11-01T00:00:00Z",
    });
    expect(sortKey(t, "id", ctx)).toBe("REL-4");
    expect(sortKey(t, "title", ctx)).toBeUndefined();
    expect(sortKey(t, "assignee", ctx)).toBe("koki");
    expect(sortKey(t, "label", ctx)).toBe("a,b");
    expect(sortKey(t, "parent", ctx)).toBe("REL-1");
    expect(sortKey(t, "created", ctx)).toBe(Date.parse("2026-11-01T00:00:00Z"));
    expect(sortKey(t, "@store", ctx)).toBe(0);
  });

  it("reads a property by its type", () => {
    const t = task("T1", "task", null, {
      props: { points: 5, done: true, prio: "high", note: "x", empty: "", area: ["a", "b"] },
    });
    expect(sortKey(t, "points", ctx)).toBe(5);
    expect(sortKey(t, "done", ctx)).toBe(1);
    expect(sortKey(t, "prio", ctx)).toBe(2);
    expect(sortKey(t, "note", ctx)).toBe("x");
    expect(sortKey(t, "empty", ctx)).toBeUndefined();
    expect(sortKey(t, "area", ctx)).toBe("a,b");
    expect(sortKey(t, "missing", ctx)).toBeUndefined();
  });

  it("sorts the status by its place in sort:status", () => {
    const t = (status: string, store = "shared") => task("T", "task", null, { status, store });
    const key = (x: ReturnType<typeof t>) => sortKey(x, "status", ctx) as number;
    expect(key(t("backlog"))).toBeLessThan(key(t("todo")));
    expect(key(t("doing"))).toBeLessThan(key(t("done")));
    expect(key(t("todo", "me"))).toBeGreaterThan(key(t("canceled")));
  });
});

describe("compareKeys", () => {
  it("compares numbers as numbers and texts naturally, numbers first", () => {
    expect(compareKeys(2, 10)).toBeLessThan(0);
    expect(compareKeys("REL-2", "REL-10")).toBeLessThan(0);
    expect(compareKeys(5, "a")).toBeLessThan(0);
    expect(compareKeys("a", 5)).toBeGreaterThan(0);
    expect(compareKeys("x", "x")).toBe(0);
  });
});
