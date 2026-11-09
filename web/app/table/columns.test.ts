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

const shared = {
  ...withProps({
    kickoff: { type: "date", label: "着手" },
    effort: { type: "number" },
    area: { type: "multi_enum", values: ["backend", "frontend"] },
    site: { type: "url", label: "サイト" },
    causes: { type: "text" },
    blocked_by: { type: "text" },
    status: { type: "text" },
    due: { type: "date" },
    priority: { type: "text" },
  }),
  priorities: ["low", "mid", "high", "urgent"],
} as SchemaView;
const me = withProps({
  area: { type: "multi_enum", label: "領域", values: ["x"] },
  team: { type: "text", label: "チーム" },
});

const ids = (list: { id: string }[]) => list.map((c) => c.id);

describe("columnCandidates", () => {
  it("offers the properties in the prop_order of the schema", () => {
    const sc = { ...shared, prop_order: ["effort", "kickoff", "area", "site"] } as SchemaView;
    expect(ids(columnCandidates([sc])).slice(-4)).toEqual(["effort", "kickoff", "area", "site"]);
  });

  it("lists the builtin columns, then the properties of the schemas by name", () => {
    expect(ids(columnCandidates([shared]))).toEqual([
      "id",
      "title",
      "kind",
      "status",
      "assignee",
      "label",
      "priority",
      "start",
      "due",
      "estimate",
      "points",
      "parent",
      "created",
      "updated",
      "pr",
      "area",
      "effort",
      "kickoff",
      "site",
    ]);
  });

  it("names the built-in columns as the detail does; a property of the schema cannot take their names", () => {
    const list = columnCandidates([shared]);
    const label = (id: string) => list.find((c) => c.id === id)?.label;
    expect(label("priority")).toBe("優先度");
    expect(label("start")).toBe("開始日");
    expect(label("due")).toBe("期限");
    expect(label("estimate")).toBe("見積もり");
    expect(label("points")).toBe("ポイント");
    expect(label("label")).toBe("ラベル");
    expect(list.filter((c) => c.id === "due")).toHaveLength(1);
    expect(list.filter((c) => c.id === "priority")).toHaveLength(1);
  });

  it("takes the union of several stores and the label of the first store that has the property", () => {
    const list = columnCandidates([shared, me]);
    expect(ids(list).slice(15)).toEqual(["area", "effort", "kickoff", "site", "team"]);
    expect(list.find((c) => c.id === "area")?.label).toBe("area");
    expect(list.find((c) => c.id === "team")?.label).toBe("チーム");
    expect(list.find((c) => c.id === "kickoff")?.label).toBe("着手");
  });

  it("shows ID, parent, created, updated and the pull requests only; a relation's name is not a column", () => {
    const readonly = columnCandidates([shared])
      .filter((c) => c.readonly)
      .map((c) => c.id);
    expect(readonly).toEqual(["id", "parent", "created", "updated", "pr"]);
  });

  it("has no property column without a schema", () => {
    expect(columnCandidates([])).toHaveLength(15);
  });
});

describe("shownColumns", () => {
  const candidates = columnCandidates([shared]);

  it("keeps the order of the view and leaves out what no schema defines", () => {
    expect(ids(shownColumns(["title", "nope", "kickoff", "id"], candidates))).toEqual([
      "title",
      "kickoff",
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
    expect(of("effort")).toBe("props.effort");
  });
  it("is the field of the task for a built-in property, and props for a property of the schema", () => {
    for (const id of ["priority", "start", "due", "estimate", "points"]) expect(of(id)).toBe(id);
    expect(of("kickoff")).toBe("props.kickoff");
  });
  it("has none for what is only shown", () => {
    for (const id of ["id", "parent", "created", "updated", "pr"]) expect(of(id)).toBeNull();
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

  it("reads the built-in properties from the fields of the task", () => {
    const t = task("T1", "task", null, {
      priority: "high",
      start: "2026-11-02",
      due: "2026-11-09",
      estimate: "1h30m",
      points: 0,
    });
    expect(sortKey(t, "priority", ctx)).toBe(2);
    expect(sortKey(t, "start", ctx)).toBe("2026-11-02");
    expect(sortKey(t, "due", ctx)).toBe("2026-11-09");
    expect(sortKey(t, "estimate", ctx)).toBe(90);
    expect(sortKey(t, "points", ctx)).toBe(0);
    // none, and a priority the list lacks goes after the ones it has
    const none = task("T2", "task", null);
    for (const id of ["priority", "start", "due", "estimate", "points"]) {
      expect(sortKey(none, id, ctx)).toBeUndefined();
    }
    expect(sortKey(task("T3", "task", null, { priority: "gone" }), "priority", ctx)).toBe(4);
    expect(sortKey(task("T4", "task", null, { estimate: "45m" }), "estimate", ctx)).toBe(45);
    expect(sortKey(task("T5", "task", null, { estimate: "6h" }), "estimate", ctx)).toBe(360);
  });

  it("reads a property by its type", () => {
    const t = task("T1", "task", null, {
      props: { effort: 5, done: true, prio: "high", note: "x", empty: "", area: ["a", "b"] },
    });
    expect(sortKey(t, "effort", ctx)).toBe(5);
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
