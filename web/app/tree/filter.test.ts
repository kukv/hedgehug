import { describe, expect, it } from "vitest";
import {
  baseQuery,
  combine,
  hasTerm,
  NO_CANCELED,
  splitTerms,
  toggleTerm,
  withoutOrder,
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

describe("withoutOrder", () => {
  it("takes out the sort: and group: terms, which a tree does not take", () => {
    expect(withoutOrder("workspace:a sort:due,-title group:status -status:canceled")).toBe(
      "workspace:a -status:canceled",
    );
    expect(withoutOrder("sort:due")).toBe("");
    expect(withoutOrder("sort:due group:parent sort:id")).toBe("");
  });

  it("takes out a negated term and a comparison on the key, as the query parser reads the key", () => {
    expect(withoutOrder("a:b -sort:due c:d")).toBe("a:b c:d");
    expect(withoutOrder("a:b sort<3 group>=2 c:d")).toBe("a:b c:d");
  });

  it("keeps the terms that only have the word in them", () => {
    const q = 'title:"sort:x" "sort:y" sorted:1 group_by:x assignee:sort -status:canceled';
    expect(withoutOrder(q)).toBe(q);
    expect(withoutOrder('"a b" sort:"x y" tag:z')).toBe('"a b" tag:z');
  });

  it("leaves a query that has none as it is", () => {
    expect(withoutOrder("assignee:@me")).toBe("assignee:@me");
    expect(withoutOrder("")).toBe("");
  });
});
