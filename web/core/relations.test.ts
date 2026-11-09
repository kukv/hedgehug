import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { RELATIONS, relationLabel } from "./relations";

describe("RELATIONS", () => {
  it("Go の表（internal/app/domain/schema/relations.go）と同じ", () => {
    const src = readFileSync(
      resolve(process.cwd(), "../internal/app/domain/schema/relations.go"),
      "utf8",
    );
    const rows = [...src.matchAll(/\{\s*"(\w+)", "([^"]+)", "(\w+)", "([^"]+)",/g)].map((m) => ({
      name: m[1],
      label: m[2],
      inverse: m[3],
      inverseLabel: m[4],
    }));
    expect(rows).toHaveLength(5);
    expect(RELATIONS).toEqual(rows);
  });
});

describe("relationLabel", () => {
  it("名前でも逆方向の名前でも表示名を返す", () => {
    expect(relationLabel("causes")).toBe("Causes");
    expect(relationLabel("caused_by")).toBe("Caused by");
    expect(relationLabel("relates")).toBe("Relates to");
    expect(relationLabel("nope")).toBe("nope");
  });
});
