import { describe, expect, it } from "vitest";
import { ApiError } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import { replaceTerm, suggestTerm } from "./suggest";

const schema = {
  statuses: [
    { name: "backlog", category: "todo" },
    { name: "in_progress", category: "doing" },
    { name: "done", category: "done" },
    { name: "canceled", category: "canceled" },
  ],
  kinds: [{ name: "story" }, { name: "task" }, { name: "bug" }],
  props: {
    priority: { type: "enum", values: ["low", "high", "urgent"] },
    points: { type: "number" },
  },
} as unknown as SchemaView;
// A second store: a status and a property the first one lacks.
const other = {
  statuses: [{ name: "review", category: "doing" }],
  kinds: [{ name: "task" }],
  props: { estimate: { type: "number" } },
} as unknown as SchemaView;

// The server's words are English and cannot be told apart by anything but the detail: that is what
// the query error carries (it has no args).
const bad = (detail: string) => new ApiError({ status: 400, type: "query.invalid", detail });

describe("suggestTerm", () => {
  it("fixes a status that is a letter off", () => {
    const e = bad('term 1 (status:in_progres): unknown status or category "in_progres"');
    expect(suggestTerm(e, [schema])).toBe("status:in_progress");
  });

  it("keeps the minus sign and the other values of the term", () => {
    const e = bad('term 2 (-status:backlgo,done): unknown status or category "backlgo"');
    expect(suggestTerm(e, [schema])).toBe("-status:backlog,done");
  });

  it("offers the categories as statuses, and the statuses of every store", () => {
    expect(
      suggestTerm(bad('term 1 (status:doin): unknown status or category "doin"'), [schema]),
    ).toBe("status:doing");
    expect(
      suggestTerm(bad('term 1 (status:reveiw): unknown status or category "reveiw"'), [
        schema,
        other,
      ]),
    ).toBe("status:review");
  });

  it("fixes a kind", () => {
    expect(suggestTerm(bad('term 1 (kind:stroy): unknown kind "stroy"'), [schema])).toBe(
      "kind:story",
    );
  });

  it("fixes a key: the ones of the query and the properties of the schemas", () => {
    expect(suggestTerm(bad('term 1 (asignee:@me): unknown key "asignee"'), [schema])).toBe(
      "assignee:@me",
    );
    expect(suggestTerm(bad('term 1 (point>=3): unknown key "point"'), [schema])).toBe("points>=3");
    expect(suggestTerm(bad('term 1 (-prioriti:high): unknown key "prioriti"'), [schema])).toBe(
      "-priority:high",
    );
    expect(suggestTerm(bad('term 1 (estimat>2): unknown key "estimat"'), [schema, other])).toBe(
      "estimate>2",
    );
  });

  it("fixes a value of an enum from the list the server gives", () => {
    const e = bad('term 1 (priority:hgh): "hgh" is not one of [low high urgent]');
    expect(suggestTerm(e, [schema])).toBe("priority:high");
  });

  it("takes a word of three letters or more that is part of a name (the example of the design)", () => {
    const e = bad('term 1 (status:progres): unknown status or category "progres"');
    expect(suggestTerm(e, [schema])).toBe("status:in_progress");
    expect(
      suggestTerm(bad('term 1 (status:prog): unknown status or category "prog"'), [schema]),
    ).toBe("status:in_progress");
    // the nearest name still wins over a name that merely holds the word
    expect(
      suggestTerm(bad('term 1 (status:dne): unknown status or category "dne"'), [schema]),
    ).toBe("status:done");
  });

  it("gives nothing when no name is near or holds the word, or the word is too short to tell", () => {
    expect(
      suggestTerm(bad('term 1 (status:zzzz): unknown status or category "zzzz"'), [schema]),
    ).toBeNull();
    expect(suggestTerm(bad('term 1 (x:1): unknown key "x"'), [schema])).toBeNull();
    // two letters are not taken for the part of a name
    expect(
      suggestTerm(bad('term 1 (status:gr): unknown status or category "gr"'), [schema]),
    ).toBeNull();
  });

  it("gives nothing for another error, or for something that is not an error of the query", () => {
    expect(suggestTerm(bad("term 1 (a): unclosed quote"), [schema])).toBeNull();
    expect(
      suggestTerm(new ApiError({ status: 404, type: "task.not_found", detail: "x" }), [schema]),
    ).toBeNull();
    expect(suggestTerm(null, [schema])).toBeNull();
    expect(suggestTerm(new Error("network"), [schema])).toBeNull();
  });

  it("gives nothing while no schema has been read", () => {
    const e = bad('term 1 (status:in_progres): unknown status or category "in_progres"');
    expect(suggestTerm(e, [])).toBeNull();
  });
});

describe("replaceTerm", () => {
  const e = bad('term 2 (status:in_progres): unknown status or category "in_progres"');

  it("puts the fixed term where the wrong one stood", () => {
    expect(replaceTerm("assignee:@me status:in_progres kind:task", e, "status:in_progress")).toBe(
      "assignee:@me status:in_progress kind:task",
    );
  });

  it("gives nothing when the term is not in the part the user types (it is the fixed part)", () => {
    expect(replaceTerm("assignee:@me", e, "status:in_progress")).toBeNull();
  });

  it("changes only the term, not another that holds the same letters", () => {
    expect(replaceTerm("status:in_progres2 status:in_progres", e, "status:in_progress")).toBe(
      "status:in_progres2 status:in_progress",
    );
  });
});
