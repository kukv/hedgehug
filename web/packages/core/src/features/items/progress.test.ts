import { describe, expect, it } from "vitest";
import type { Kind } from "../../model/item";
import { item } from "./fixtures.test-util";
import {
  categoryOf,
  childCount,
  dueLabel,
  epicProgress,
  progressPercent,
  stepProgress,
} from "./progress";
import { settings } from "./settings.test-util";

const step = (n: number, status: string) => ({
  ...item("S1", "S1-P1", n, "Step", "S1-I2"),
  status,
});

describe("progress", () => {
  const today = new Date(2026, 10, 13);

  it("finds the category of a status, undefined for an unknown one", () => {
    expect(categoryOf(settings, "st3")).toBe("done");
    expect(categoryOf(settings, "gone")).toBeUndefined();
  });

  it("counts done children and leaves canceled ones out", () => {
    const steps = [step(3, "st3"), step(4, "st1"), step(5, "st4")];
    expect(childCount(steps, settings)).toEqual({ done: 1, total: 2 });
    expect(stepProgress(steps, "S1-I2", settings)).toEqual({ done: 1, total: 2 });
    expect(stepProgress([], "S1-I2", settings)).toBeNull();
  });

  it("labels the due date", () => {
    expect(dueLabel("", "todo", today)).toBeNull();
    expect(dueLabel("2026-11-12", "todo", today)).toEqual({
      text: "期限切れ 11/12",
      emphasis: "overdue",
    });
    expect(dueLabel("2026-11-13", "doing", today)).toEqual({
      text: "今日 11/13",
      emphasis: "today",
    });
    expect(dueLabel("2026-11-12", "done", today)).toEqual({ text: "11/12", emphasis: null });
    expect(dueLabel("2027-01-05", undefined, today)).toEqual({
      text: "2027/01/05",
      emphasis: null,
    });
  });
  it("counts what is under an Epic except Steps and canceled ones", () => {
    const at = (n: number, kind: Kind, parentId: string, status: string) => ({
      ...item("S1", "S1-P1", n, kind, parentId),
      status,
    });
    const items = [
      item("S1", "S1-P1", 2, "Epic", null),
      at(3, "Story", "S1-I2", "st3"),
      at(4, "Task", "S1-I2", "st1"),
      at(5, "Bug", "S1-I2", "st4"),
      at(6, "Step", "S1-I4", "st3"),
      item("S1", "S1-P1", 8, "Epic", null),
      at(9, "Task", "S1-I8", "st3"),
    ];
    // 完了 1(Story)、未完了 1(Task)。Bug は中止、Step は数えない。別の Epic の Task は入れない。
    expect(epicProgress(items, "S1-I2", settings)).toEqual({ done: 1, total: 2 });
    expect(epicProgress(items, "S1-I8", settings)).toEqual({ done: 1, total: 1 });
    expect(epicProgress(items, "S1-I99", settings)).toEqual({ done: 0, total: 0 });
  });

  it("rounds the percent and gives 0 when there is nothing to count", () => {
    expect(progressPercent({ done: 0, total: 0 })).toBe(0);
    expect(progressPercent({ done: 1, total: 3 })).toBe(33);
    expect(progressPercent({ done: 2, total: 3 })).toBe(67);
    expect(progressPercent({ done: 1, total: 8 })).toBe(13);
    expect(progressPercent({ done: 4, total: 4 })).toBe(100);
  });
});
