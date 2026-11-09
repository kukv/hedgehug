import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { inverseRelationNames, RELATIONS, relationLabel, relationOf } from "./relations";

describe("RELATIONS", () => {
  it("Go の表（internal/app/domain/schema/relations.go）と同じ", () => {
    const src = readFileSync(
      resolve(process.cwd(), "../internal/app/domain/schema/relations.go"),
      "utf8",
    );
    const rows = [...src.matchAll(/\{"(\w+)", "([^"]+)", "(\w+)", "([^"]+)",/g)].map((m) => ({
      name: m[1],
      label: m[2],
      inverse: m[3],
      inverseLabel: m[4],
    }));
    expect(rows).toHaveLength(5);
    expect(RELATIONS).toEqual(rows);
  });
});

describe("relationOf / relationLabel", () => {
  it("名前でも逆方向の名前でも引ける", () => {
    expect(relationOf("causes")).toEqual({ name: "causes", inverse: false });
    expect(relationOf("caused_by")).toEqual({ name: "causes", inverse: true });
    expect(relationOf("assignees")).toBeUndefined();
    expect(relationLabel("relates")).toBe("Relates to");
    expect(relationLabel("related_by")).toBe("Related by");
    expect(relationLabel("nope")).toBe("nope");
  });
});

describe("inverseRelationNames", () => {
  it("逆方向の名前を表の順に並べる", () => {
    expect(inverseRelationNames()).toEqual([
      "blocks",
      "caused_by",
      "related_by",
      "duplicated_by",
      "cloned_by",
    ]);
  });
});
