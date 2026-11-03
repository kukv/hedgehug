import { describe, expect, it } from "vitest";
import {
  baseQuery,
  combine,
  hasTerm,
  NO_CANCELED,
  splitTerms,
  toggleTerm,
  withoutTerm,
} from "./filter";

describe("splitTerms", () => {
  it("splits at white space outside double quotes", () => {
    expect(splitTerms('a:b  "c d" -status:canceled')).toEqual(["a:b", '"c d"', "-status:canceled"]);
    expect(splitTerms("   ")).toEqual([]);
  });
});

describe("baseQuery", () => {
  it("writes a workspace by name, quoted when it has a space", () => {
    expect(baseQuery({ kind: "workspace", name: "release" }, "api")).toBe("workspace:release");
    expect(baseQuery({ kind: "workspace", name: "my work" }, "cli")).toBe('workspace:"my work"');
  });
  it("writes a project by ULID for the API and by ID for the CLI", () => {
    const b = { kind: "project", ulid: "01JABC", id: "REL-15" } as const;
    expect(baseQuery(b, "api")).toBe("project:01JABC");
    expect(baseQuery(b, "cli")).toBe("project:REL-15");
  });
});

describe("combine", () => {
  it("puts the base first and joins the input with a space", () => {
    expect(combine("project:X", "assignee:@me")).toBe("project:X assignee:@me");
    expect(combine("project:X", "   ")).toBe("project:X");
  });
  it("has no space to lead with when there is no base", () => {
    expect(combine("", " assignee:@me ")).toBe("assignee:@me");
    expect(combine("", "")).toBe("");
  });
});

describe("terms", () => {
  it("finds, removes and toggles a term", () => {
    expect(hasTerm(`a:b ${NO_CANCELED}`, NO_CANCELED)).toBe(true);
    expect(hasTerm("a:b", NO_CANCELED)).toBe(false);
    expect(withoutTerm(`a:b ${NO_CANCELED} c`, NO_CANCELED)).toBe("a:b c");
    expect(toggleTerm("a:b", NO_CANCELED)).toBe(`a:b ${NO_CANCELED}`);
    expect(toggleTerm(`a:b ${NO_CANCELED}`, NO_CANCELED)).toBe("a:b");
    expect(toggleTerm("", NO_CANCELED)).toBe(NO_CANCELED);
  });
});
