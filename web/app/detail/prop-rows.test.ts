import { describe, expect, it } from "vitest";
import { builtinRows, propRows } from "./prop-rows";
import { schemaView } from "./testkit";

describe("builtinRows", () => {
  const task = {
    priority: "mid",
    start: null,
    due: "2026-11-07",
    estimate: "1h30m",
    points: 0,
    labels: ["bug"],
  };

  it("are the six built-in properties in the fixed order, below the status of the detail", () => {
    expect(builtinRows(schemaView, task).map((r) => [r.name, r.label, r.editor])).toEqual([
      ["priority", "優先度", "enum"],
      ["start", "開始日", "date"],
      ["due", "期限", "date"],
      ["estimate", "見積もり", "duration"],
      ["points", "ポイント", "number"],
      ["labels", "ラベル", "multi_enum"],
    ]);
  });

  it("take the value from the field of the task, null when it has none, and 0 stays 0", () => {
    const rows = builtinRows(schemaView, task);
    expect(rows.map((r) => r.value)).toEqual(["mid", null, "2026-11-07", "1h30m", 0, ["bug"]]);
    expect(builtinRows(schemaView, {}).map((r) => r.value)).toEqual([
      null,
      null,
      null,
      null,
      null,
      null,
    ]);
  });

  it("choose the priority from the priorities and the labels from the labels of the schema", () => {
    const rows = builtinRows(schemaView, task);
    expect(rows.find((r) => r.name === "priority")).toMatchObject({
      options: ["low", "mid", "high"],
      multi: false,
    });
    expect(rows.find((r) => r.name === "labels")).toMatchObject({
      options: ["bug", "ux"],
      multi: true,
    });
    expect(rows.find((r) => r.name === "due")?.options).toEqual([]);
  });

  it("have no choices for a schema that lists no labels", () => {
    const sc = { ...schemaView, labels: null, priorities: null };
    const rows = builtinRows(sc, task);
    expect(rows.find((r) => r.name === "labels")?.options).toEqual([]);
    expect(rows.find((r) => r.name === "priority")?.options).toEqual([]);
  });
});

describe("propRows", () => {
  it("lists the editable properties of the schema by name, with their editors and values", () => {
    const rows = propRows(schemaView, { kickoff: "2026-11-07", effort: 5 });
    expect(rows.map((r) => r.name)).toEqual(["area", "effort", "kickoff", "tier"]);
    const kickoff = rows.find((r) => r.name === "kickoff");
    expect(kickoff).toMatchObject({ editor: "date", value: "2026-11-07", label: "kickoff" });
    expect(rows.find((r) => r.name === "effort")).toMatchObject({ editor: "number", value: 5 });
    expect(rows.find((r) => r.name === "tier")).toMatchObject({
      editor: "enum",
      options: ["gold", "silver"],
      multi: false,
    });
  });

  it("shows a relation name left in props (a bad value kept there) as a read-only row", () => {
    const rows = propRows(schemaView, { relates: ["not-a-ulid"] });
    expect(rows.find((r) => r.name === "relates")).toMatchObject({
      editor: null,
      value: ["not-a-ulid"],
    });
  });

  it("shows a built-in name left in props (an old form) as a read-only row, apart from the field of the task", () => {
    const rows = propRows(schemaView, { due: "2026-11-07" });
    expect(rows.find((r) => r.name === "due")).toMatchObject({
      editor: null,
      value: "2026-11-07",
    });
  });

  it("does not make a row for a relation the schema cannot define", () => {
    const sc = { ...schemaView, props: { ...schemaView.props, causes: { type: "text" } } };
    expect(propRows(sc, {}).map((r) => r.name)).not.toContain("causes");
  });

  it("follows prop_order when the schema gives it", () => {
    const names = (o: string[]) =>
      propRows({ ...schemaView, prop_order: o }, {}).map((r) => r.name);
    const base = names([]);
    expect(base.length).toBeGreaterThan(1);
    expect(names([...base].reverse())).toEqual([...base].reverse());
    // Only some are named: they come first, the rest by name.
    expect(names(["kickoff", "effort"])).toEqual(["kickoff", "effort", "area", "tier"]);
  });

  it("appends keys the schema does not define, read-only", () => {
    const rows = propRows(schemaView, { team: "web" });
    expect(rows.at(-1)).toMatchObject({ name: "team", editor: null, value: "web" });
  });
});
