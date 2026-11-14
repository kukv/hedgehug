import { describe, expect, it } from "vitest";
import {
  asOptions,
  chosen,
  releaseChoices,
  showRelease,
  showSprint,
  sprintChoices,
} from "./choices";
import { overviewOf, releaseOf, sprintOf } from "./fixtures.test-util";

// アイテムのスプリント・リリースの選択肢(02「スプリント・リリース」、05、F1-5・F1-15・F1-21)。
// 概要のスプリントは API が並べた順(実行中、計画中を開始日の早い順・期間未定は最後、完了)。
const sprints = [
  sprintOf({ id: "S1", name: "Sprint 1", state: "active", start: "2026-11-02" }),
  sprintOf({ id: "S2a", name: "Sprint 2a", state: "planned", start: "2026-11-16" }),
  sprintOf({ id: "S2b", name: "Sprint 2b", state: "planned", start: "2026-11-16" }),
  sprintOf({ id: "S4", name: "Sprint 4", state: "planned", start: "2026-11-30" }),
  sprintOf({ id: "S3", name: "Sprint 3", state: "planned" }),
  sprintOf({ id: "S0", name: "Sprint 0", state: "closed", start: "2026-10-01" }),
];
const o = overviewOf(sprints, [
  releaseOf({ id: "R1", name: "v1.0", state: "released" }),
  releaseOf({ id: "R0", name: "v0.9", state: "archived" }),
  releaseOf({ id: "R2", name: "v1.1" }),
]);

describe("sprintChoices", () => {
  it("lists the backlog and the sprints not closed, the active one first", () => {
    expect(sprintChoices(o, "")).toEqual([
      { id: "", label: "バックログ" },
      { id: "S1", label: "Sprint 1(実行中)" },
      { id: "S2a", label: "Sprint 2a" },
      { id: "S2b", label: "Sprint 2b" },
      { id: "S4", label: "Sprint 4" },
      { id: "S3", label: "Sprint 3" },
    ]);
  });

  it("keeps the API order instead of sorting again (the server compares names as UTF-8 bytes)", () => {
    const api = overviewOf([
      sprintOf({ id: "FW", name: "\uFF21", state: "planned", start: "2026-11-16" }),
      sprintOf({ id: "EM", name: "\u{1F600}", state: "planned", start: "2026-11-16" }),
    ]);
    expect(sprintChoices(api, "").map((c) => c.id)).toEqual(["", "FW", "EM"]);
  });

  it("keeps the closed sprint the item is in", () => {
    const ids = sprintChoices(o, "S0").map((c) => c.id);
    expect(ids).toEqual(["", "S1", "S2a", "S2b", "S4", "S3", "S0"]);
    expect(chosen(sprintChoices(o, "S0"), "S0")).toBe("S0");
  });

  it("shows a deleted or missing sprint as the backlog", () => {
    const choices = sprintChoices(o, "gone");
    expect(choices.map((c) => c.id)).not.toContain("gone");
    expect(chosen(choices, "gone")).toBe("");
  });
});

describe("releaseChoices", () => {
  it("lists none and the releases not archived", () => {
    expect(releaseChoices(o, "")).toEqual([
      { id: "", label: "なし" },
      { id: "R1", label: "v1.0" },
      { id: "R2", label: "v1.1" },
    ]);
  });

  it("keeps the archived release the item is in, and shows a missing one as none", () => {
    expect(releaseChoices(o, "R0").map((c) => c.id)).toEqual(["", "R1", "R0", "R2"]);
    expect(chosen(releaseChoices(o, "gone"), "gone")).toBe("");
  });
});

describe("showSprint and showRelease", () => {
  it("shows the sprint for Story, Task and Bug, and the release except for Step", () => {
    const kinds = ["Epic", "Story", "Task", "Bug", "Step"] as const;
    expect(kinds.map(showSprint)).toEqual([false, true, true, true, false]);
    expect(kinds.map(showRelease)).toEqual([true, true, true, true, false]);
  });
});

describe("asOptions", () => {
  it("turns choices into select options", () => {
    expect(asOptions(releaseChoices(o, ""))).toEqual([
      { value: "", label: "なし" },
      { value: "R1", label: "v1.0" },
      { value: "R2", label: "v1.1" },
    ]);
  });
});
