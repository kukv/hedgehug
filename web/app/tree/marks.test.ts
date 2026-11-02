import { describe, expect, it } from "vitest";
import { task } from "./fixtures";
import { openBlocker, problemLabel, propText } from "./marks";

describe("problemLabel", () => {
  it("says a missing required field as the canvas does", () => {
    expect(problemLabel({ code: "required", field: "points", message: "..." })).toBe(
      "points 未入力",
    );
  });
  it("names the other codes in Japanese and falls back to the code", () => {
    expect(problemLabel({ code: "missing_ref", field: "blocked_by", message: "" })).toBe(
      "参照先がありません",
    );
    expect(problemLabel({ code: "cycle", field: "parent", message: "" })).toBe("循環しています");
    expect(problemLabel({ code: "other_code", field: "", message: "" })).toBe("other_code");
  });
});

describe("propText", () => {
  it("returns a string property and ignores the others", () => {
    const t = task("T1", "task", null, { props: { due: "2026-11-07", points: 3 } });
    expect(propText(t, "due")).toBe("2026-11-07");
    expect(propText(t, "points")).toBeUndefined();
    expect(propText(t, "none")).toBeUndefined();
  });
});

describe("openBlocker", () => {
  const blocked = task("T1", "task", null, {
    blocked_by: [
      { ulid: "B1", id: "REL-2", title: "a" },
      { ulid: "B2", id: "REL-3", title: "b" },
    ],
  });
  const closed = (t: { status: string }) => t.status === "done";

  it("names the first blocker that is still open", () => {
    const m = new Map([
      ["B1", task("B1", "task", null, { status: "done" })],
      ["B2", task("B2", "task", null, { status: "todo" })],
    ]);
    expect(openBlocker(blocked, (u) => m.get(u), closed, false)).toBe("REL-3");
  });
  it("is not blocked when every blocker is done or deleted", () => {
    const m = new Map([
      ["B1", task("B1", "task", null, { status: "done" })],
      ["B2", task("B2", "task", null, { deleted_at: "2026-11-01T00:00:00Z" })],
    ]);
    expect(openBlocker(blocked, (u) => m.get(u), closed, true)).toBeUndefined();
  });
  it("asks the server when a blocker is not in the cache", () => {
    expect(openBlocker(blocked, () => undefined, closed, true)).toBe("REL-2");
    expect(openBlocker(blocked, () => undefined, closed, false)).toBeUndefined();
  });
});
