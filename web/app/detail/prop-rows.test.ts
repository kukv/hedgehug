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

  it("leaves out task references, which the relations section shows", () => {
    expect(propRows(schemaView, {}).map((r) => r.name)).not.toContain("relates");
  });

  it("appends keys the schema does not define, read-only", () => {
    const rows = propRows(schemaView, { team: "web" });
    expect(rows.at(-1)).toMatchObject({ name: "team", editor: null, value: "web" });
  });
});
