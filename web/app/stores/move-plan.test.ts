import { describe, expect, it } from "vitest";
import { ApiError } from "../../core/api/client";
import {
  hiddenSentences,
  moveButton,
  parentSentence,
  referenceKind,
  unlinkedRows,
  violationView,
} from "./move-plan";

describe("referenceKind", () => {
  it.each([
    ["blocked_by", "Blocked by"],
    ["sprint", "スプリント"],
    ["releases", "リリース"],
    ["props.relates", "relates"],
    ["props.duplicates", "duplicates"],
    ["something", "something"],
  ])("%s は %s", (field, want) => {
    expect(referenceKind(field)).toBe(want);
  });
});

describe("unlinkedRows", () => {
  it("lists the reference with its kind, the one it points to and the task it is on", () => {
    expect(
      unlinkedRows([
        {
          task: { id: "DM-3", ulid: "U3" },
          field: "blocked_by",
          target: { label: "DM-1", ulid: "U1" },
        },
        { task: { id: "DM-1", ulid: "U1" }, field: "sprint", target: { label: "S1", ulid: "S" } },
        {
          task: { id: "DM-4", ulid: "U4" },
          field: "props.relates",
          target: { label: "DM-1", ulid: "U1" },
        },
      ]),
    ).toEqual([
      { key: "U3 blocked_by U1", kind: "Blocked by", target: "DM-1", id: "DM-3" },
      { key: "U1 sprint S", kind: "スプリント", target: "S1", id: "DM-1" },
      { key: "U4 props.relates U1", kind: "relates", target: "DM-1", id: "DM-4" },
    ]);
  });
  it("is empty for none", () => {
    expect(unlinkedRows([])).toEqual([]);
  });
});

describe("moveButton", () => {
  it("says the number of references it cuts", () => {
    expect(moveButton(0)).toBe("移動");
    expect(moveButton(1)).toBe("1 件の参照を外して移動");
    expect(moveButton(3)).toBe("3 件の参照を外して移動");
  });
});

describe("parentSentence", () => {
  it("leaves the parent in the source and goes under the parent given", () => {
    expect(parentSentence({ from: "DM-1 親", parent: "SC-4 新しい親", workspace: "a" })).toBe(
      "DM-1 親 から外し、SC-4 新しい親 の下に置きます",
    );
  });
  it("goes to the root of the workspace without a parent", () => {
    expect(parentSentence({ from: "DM-1 親", parent: null, workspace: "a" })).toBe(
      "DM-1 親 から外し、a のルートに置きます",
    );
  });
  it("leaves the first half out for a task with no parent", () => {
    expect(parentSentence({ from: null, parent: null, workspace: "a" })).toBe(
      "a のルートに置きます",
    );
    expect(parentSentence({ from: null, parent: "SC-4 親", workspace: "a" })).toBe(
      "SC-4 親 の下に置きます",
    );
  });
});

describe("hiddenSentences", () => {
  it("names who can no longer see it, and that the history stays", () => {
    expect(hiddenSentences(["a@x", "b@x"], "me")).toEqual([
      "見えなくなる人: a@x、b@x",
      "移動前の内容は me の git 履歴に残り、me を読める人は過去の内容を見られます",
    ]);
  });
});

describe("violationView", () => {
  it("splits move.invalid into a headline and one line for each task, without the indent", () => {
    const e = new ApiError({
      status: 422,
      type: "move.invalid",
      detail: "second refuses the tasks:\n  DM-1 A: x\n  DM-2 B: y",
      args: { store: "second", violations: ["  DM-1 A: x", "  DM-2 B: y"] },
    });
    expect(violationView(e)).toEqual({
      headline: "ここには移せません",
      lines: ["DM-1 A: x", "DM-2 B: y"],
    });
  });
  it("writes the violations of schema.violation as lines", () => {
    const e = new ApiError({
      status: 422,
      type: "schema.violation",
      detail: "english",
      args: {
        violations: [
          { code: "unknown_kind", field: "kind", message: "kind bug" },
          { code: "placement", field: "", message: "no" },
        ],
      },
    });
    expect(violationView(e)).toEqual({
      headline: "スキーマに合わない変更です",
      lines: ["未定義の種類です（kind）: kind bug", "この親には置けません: no"],
    });
  });
  it("is null for the other errors, and for a violation with no lines", () => {
    expect(violationView(new ApiError({ status: 409, type: "move.exists" }))).toBeNull();
    expect(violationView(new ApiError({ status: 422, type: "move.invalid", args: {} }))).toBeNull();
    expect(violationView(new Error("x"))).toBeNull();
  });
});
