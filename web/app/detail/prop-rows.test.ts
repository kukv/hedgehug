import { describe, expect, it } from "vitest";
import { propRows } from "./prop-rows";
import { schemaView } from "./testkit";

describe("propRows", () => {
  it("lists the editable properties of the schema by name, with their editors and values", () => {
    const rows = propRows(schemaView, { due: "2026-11-07", points: 5 });
    expect(rows.map((r) => r.name)).toEqual(["area", "due", "points", "priority", "start"]);
    const due = rows.find((r) => r.name === "due");
    expect(due).toMatchObject({ editor: "date", value: "2026-11-07", label: "due" });
    expect(rows.find((r) => r.name === "points")).toMatchObject({ editor: "number", value: 5 });
    expect(rows.find((r) => r.name === "priority")).toMatchObject({
      editor: "enum",
      options: ["low", "mid", "high"],
      multi: false,
    });
  });

  it("leaves out the relations, which the relations section shows, even as a stray key", () => {
    const names = propRows(schemaView, { causes: ["x"], relates: ["y"] }).map((r) => r.name);
    expect(names).not.toContain("causes");
    expect(names).not.toContain("relates");
  });

  it("follows prop_order when the schema gives it", () => {
    const names = (o: string[]) =>
      propRows({ ...schemaView, prop_order: o }, {}).map((r) => r.name);
    const base = names([]);
    expect(base.length).toBeGreaterThan(1);
    expect(names([...base].reverse())).toEqual([...base].reverse());
    // Only some are named: they come first, the rest by name.
    expect(names(["start", "points"])).toEqual(["start", "points", "area", "due", "priority"]);
  });

  it("appends keys the schema does not define, read-only", () => {
    const rows = propRows(schemaView, { team: "web" });
    expect(rows.at(-1)).toMatchObject({ name: "team", editor: null, value: "web" });
  });
});
