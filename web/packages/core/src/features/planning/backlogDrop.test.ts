import { describe, expect, it } from "vitest";
import { dropPlacement, goneText, moveDown, moveUp } from "./backlogDrop";
import { frameOf, rowOf, sprintOf } from "./fixtures.test-util";

// バックログの画面のドロップ先と「上へ」「下へ」(05「並び順」、F2a-3・F2a-8・F2a-12)。

const s1 = sprintOf({ id: "S1", state: "active" });
const frames = [
  frameOf(s1, [rowOf({ id: "X" })]),
  frameOf(sprintOf({ id: "S2" }), []),
  frameOf(null, [rowOf({ id: "A" }), rowOf({ id: "B" }), rowOf({ id: "C" })]),
];

describe("dropPlacement", () => {
  it("puts the item just before the row it is dropped on", () => {
    expect(dropPlacement(frames, "C", { frame: null, row: "A" })).toEqual({
      sprint: null,
      anchor: "A",
      after: false,
    });
  });

  it("moves the item into the frame of the row it is dropped on", () => {
    expect(dropPlacement(frames, "A", { frame: "S1", row: "X" })).toEqual({
      sprint: "S1",
      anchor: "X",
      after: false,
    });
    expect(dropPlacement(frames, "X", { frame: null, row: "B" })).toEqual({
      sprint: null,
      anchor: "B",
      after: false,
    });
  });

  it("does nothing for the same position", () => {
    expect(dropPlacement(frames, "A", { frame: null, row: "A" })).toBeNull();
    expect(dropPlacement(frames, "A", { frame: null, row: "B" })).toBeNull();
    expect(dropPlacement(frames, "C", { frame: null, row: null })).toBeNull();
    expect(dropPlacement(frames, "X", { frame: "S1", row: null })).toBeNull();
  });

  it("puts the item at the end of the frame dropped on its empty area", () => {
    expect(dropPlacement(frames, "A", { frame: "S2", row: null })).toEqual({
      sprint: "S2",
      anchor: null,
      after: false,
    });
    expect(dropPlacement(frames, "A", { frame: null, row: null })).toEqual({
      sprint: null,
      anchor: null,
      after: false,
    });
  });

  it("does nothing for a frame or a row that is not on the screen", () => {
    expect(dropPlacement(frames, "A", { frame: "S9", row: null })).toBeNull();
    expect(dropPlacement(frames, "A", { frame: "S1", row: "B" })).toBeNull();
    expect(dropPlacement(frames, "Z", { frame: null, row: "A" })).toBeNull();
  });
});

describe("moveUp and moveDown", () => {
  it("puts the item before the previous row and after the next row of the same frame", () => {
    expect(moveUp(frames, "B")).toEqual({ sprint: null, anchor: "A", after: false });
    expect(moveDown(frames, "B")).toEqual({ sprint: null, anchor: "C", after: true });
  });

  it("is null at the ends of the frame", () => {
    expect(moveUp(frames, "A")).toBeNull();
    expect(moveDown(frames, "C")).toBeNull();
    expect(moveUp(frames, "X")).toBeNull();
    expect(moveDown(frames, "X")).toBeNull();
  });

  it("keeps the sprint of the frame", () => {
    const two = [frameOf(s1, [rowOf({ id: "X" }), rowOf({ id: "Y" })])];
    expect(moveDown(two, "X")).toEqual({ sprint: "S1", anchor: "Y", after: true });
  });
});

describe("goneText", () => {
  it("says why the item is not shown in the backlog", () => {
    expect(goneText("HH-31", "done")).toBe("HH-31 は完了のため、バックログには表示されません");
    expect(goneText("HH-5", "canceled")).toBe("HH-5 は中止のため、バックログには表示されません");
  });
});
